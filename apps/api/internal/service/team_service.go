package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/sailboxhq/sailbox/apps/api/internal/auth"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type TeamService struct {
	store      store.Store
	jwtManager *auth.JWTManager
	logger     *slog.Logger
	sessions   auth.SessionInvalidator
}

func NewTeamService(s store.Store, jwtManager *auth.JWTManager, logger *slog.Logger, sessions auth.SessionInvalidator) *TeamService {
	return &TeamService{store: s, jwtManager: jwtManager, logger: logger, sessions: sessions}
}

// revokeSessions drops cached session state so a removal or role change takes
// effect on the very next request instead of when the cache entry expires.
func (s *TeamService) revokeSessions(userID uuid.UUID) {
	if s.sessions != nil {
		s.sessions.Invalidate(userID)
	}
}

// ============================================================================
// Members
// ============================================================================

func (s *TeamService) ListMembers(ctx context.Context, orgID uuid.UUID) ([]model.TeamMember, error) {
	users, _, err := s.store.Users().ListByOrg(ctx, orgID, store.ListParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	members := make([]model.TeamMember, len(users))
	for i, u := range users {
		members[i] = model.TeamMember{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			FirstName:   u.FirstName,
			LastName:    u.LastName,
			AvatarURL:   u.AvatarURL,
			Role:        string(u.Role),
			CreatedAt:   u.CreatedAt,
		}
	}
	return members, nil
}

func (s *TeamService) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "member" {
		return errors.New("role must be 'admin' or 'member'")
	}

	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}

	if user.Role == model.RoleOwner {
		return errors.New("cannot change the owner's role")
	}

	// Change the role and revoke issued refresh tokens together, so a demoted
	// user cannot keep renewing a session that still carries the old role.
	err = s.store.RunInTx(ctx, func(ctx context.Context, tx store.Store) error {
		if err := tx.Users().UpdateRole(ctx, userID, role); err != nil {
			return err
		}
		return tx.Users().IncrementTokenVersion(ctx, userID)
	})
	if err != nil {
		return err
	}
	s.revokeSessions(userID)

	s.logger.Info("member role updated",
		slog.String("user_id", userID.String()),
		slog.String("role", role),
	)
	return nil
}

func (s *TeamService) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}

	if user.Role == model.RoleOwner {
		return errors.New("cannot remove the owner")
	}

	// Drop project grants and deactivate the account as one unit: a partial
	// failure used to strip the user of every project while leaving them in the
	// org, with nothing to roll back.
	err = s.store.RunInTx(ctx, func(ctx context.Context, tx store.Store) error {
		memberships, err := tx.ProjectMembers().ListByUser(ctx, userID)
		if err != nil {
			return err
		}
		for _, m := range memberships {
			if err := tx.ProjectMembers().Delete(ctx, m.ProjectID, userID); err != nil {
				return fmt.Errorf("remove project membership %s: %w", m.ProjectID, err)
			}
		}
		// Soft-delete rather than detach: org_id is NOT NULL, and the partial
		// unique index on email keeps the address available for a re-invite.
		return tx.Users().Delete(ctx, userID)
	})
	if err != nil {
		return err
	}
	s.revokeSessions(userID)

	s.logger.Info("member removed from org",
		slog.String("user_id", userID.String()),
		slog.String("org_id", orgID.String()),
	)
	return nil
}

// ============================================================================
// Invitations
// ============================================================================

func (s *TeamService) InviteMember(ctx context.Context, orgID uuid.UUID, invitedBy uuid.UUID, email, role string) (*model.Invitation, error) {
	if role != "admin" && role != "member" {
		return nil, errors.New("role must be 'admin' or 'member'")
	}

	// Check if user is already a member of the org
	existingUser, err := s.store.Users().GetByEmail(ctx, email)
	if err == nil && existingUser.OrgID == orgID {
		return nil, errors.New("user is already a member of this organization")
	}

	// Check for existing pending invitation
	existingInvs, _ := s.store.Invitations().ListByOrg(ctx, orgID)
	for _, inv := range existingInvs {
		if inv.Email == email && inv.AcceptedAt == nil && time.Now().Before(inv.ExpiresAt) {
			return nil, errors.New("an invitation is already pending for this email")
		}
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	inv := &model.Invitation{
		OrgID:     orgID,
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: &invitedBy,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
	}

	if err := s.store.Invitations().Create(ctx, inv); err != nil {
		return nil, err
	}

	s.logger.Info("invitation created",
		slog.String("email", email),
		slog.String("org_id", orgID.String()),
	)

	// Return the invitation with the token exposed for generating invite URL.
	// The caller can use inv.Token to build the URL; JSON serialization will still hide it.
	return inv, nil
}

// AcceptInvitation moves a user into the inviting organization and returns a
// freshly issued token pair.
//
// The caller's existing tokens carry the previous org and role in their claims,
// so they are revoked as part of the move: left alive they would keep working
// against the old organization, and because the move also clears the user's
// project grants, the "no grants means org-wide access" rule would hand them
// unrestricted access there.
func (s *TeamService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*AuthResult, error) {
	inv, err := s.store.Invitations().GetByToken(ctx, token)
	if err != nil {
		return nil, errors.New("invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation has expired")
	}

	if inv.AcceptedAt != nil {
		return nil, errors.New("invitation has already been accepted")
	}

	// Verify the accepting user's email matches the invitation
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if user.Email != inv.Email {
		return nil, errors.New("invitation was sent to a different email address")
	}

	// Join the org and consume the invitation atomically — otherwise a failure
	// between the two leaves the invitation open after it was already used.
	previousOrg := user.OrgID
	user.OrgID = inv.OrgID
	user.Role = model.Role(inv.Role)
	now := time.Now()
	inv.AcceptedAt = &now

	if err := s.store.RunInTx(ctx, func(ctx context.Context, tx store.Store) error {
		// Grants belong to the organization being left; carrying them across
		// would leave rows that match nothing here, exactly as RemoveMember
		// clears them when someone leaves.
		if previousOrg != inv.OrgID {
			if err := tx.ProjectMembers().DeleteByUser(ctx, userID); err != nil {
				return fmt.Errorf("clear previous project grants: %w", err)
			}
		}
		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}
		// Revoke tokens minted for the previous organization and role.
		if err := tx.Users().IncrementTokenVersion(ctx, userID); err != nil {
			return err
		}
		return tx.Invitations().Update(ctx, inv)
	}); err != nil {
		return nil, err
	}

	// The version bumped inside the transaction is the one the new tokens must
	// carry, so re-read rather than trusting the in-memory copy.
	updated, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.revokeSessions(userID)

	tokens, err := s.jwtManager.GenerateTokenPair(updated.ID, updated.OrgID, string(updated.Role), updated.TokenVersion)
	if err != nil {
		return nil, err
	}

	s.logger.Info("invitation accepted",
		slog.String("user_id", userID.String()),
		slog.String("org_id", inv.OrgID.String()),
	)
	return &AuthResult{
		User:         updated,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}

func (s *TeamService) ListInvitations(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
	return s.store.Invitations().ListByOrg(ctx, orgID)
}

func (s *TeamService) CancelInvitation(ctx context.Context, orgID, invID uuid.UUID) error {
	inv, err := s.store.Invitations().GetByID(ctx, invID)
	if err != nil || inv.OrgID != orgID {
		return errors.New("invitation not found")
	}

	if err := s.store.Invitations().Delete(ctx, invID); err != nil {
		return err
	}

	s.logger.Info("invitation cancelled", slog.String("invitation_id", invID.String()))
	return nil
}

// ============================================================================
// Project Access (for Member role)
// ============================================================================

func (s *TeamService) SetProjectAccess(ctx context.Context, orgID, projectID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "viewer" {
		return errors.New("project role must be 'admin' or 'viewer'")
	}

	if err := s.verifyProjectAndUserInOrg(ctx, orgID, projectID, userID); err != nil {
		return err
	}

	// ProjectMember has no update path, so a role change is delete + create.
	// Run both in one transaction: a failure between them used to leave the
	// user with no access at all instead of their previous role.
	return s.store.RunInTx(ctx, func(ctx context.Context, tx store.Store) error {
		if _, err := tx.ProjectMembers().GetByProjectAndUser(ctx, projectID, userID); err == nil {
			if err := tx.ProjectMembers().Delete(ctx, projectID, userID); err != nil {
				return err
			}
		}
		return tx.ProjectMembers().Create(ctx, &model.ProjectMember{
			ProjectID: projectID,
			UserID:    userID,
			Role:      role,
		})
	})
}

func (s *TeamService) RemoveProjectAccess(ctx context.Context, orgID, projectID, userID uuid.UUID) error {
	if err := s.verifyProjectAndUserInOrg(ctx, orgID, projectID, userID); err != nil {
		return err
	}
	return s.store.ProjectMembers().Delete(ctx, projectID, userID)
}

// verifyProjectAndUserInOrg confirms both ends of a grant belong to the caller's
// organization. Being an owner authorises changes inside that organization only;
// without this an owner who knows the UUIDs could grant or revoke access in
// someone else's tenant — and revoking a member's last grant elsewhere would
// silently widen them back to org-wide access under the "no grants" rule.
func (s *TeamService) verifyProjectAndUserInOrg(ctx context.Context, orgID, projectID, userID uuid.UUID) error {
	project, err := s.store.Projects().GetByID(ctx, projectID)
	if err != nil || project.OrgID != orgID {
		return errors.New("project not found")
	}
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil || user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}
	return nil
}

func (s *TeamService) GetProjectAccess(ctx context.Context, projectID, userID uuid.UUID) (string, error) {
	pm, err := s.store.ProjectMembers().GetByProjectAndUser(ctx, projectID, userID)
	if err != nil {
		return "", err
	}
	return pm.Role, nil
}

func (s *TeamService) ListUserProjects(ctx context.Context, userID uuid.UUID) ([]model.ProjectMember, error) {
	return s.store.ProjectMembers().ListByUser(ctx, userID)
}

func (s *TeamService) GetInvitationByToken(ctx context.Context, token string) (*model.Invitation, error) {
	inv, err := s.store.Invitations().GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation has expired")
	}
	if inv.AcceptedAt != nil {
		return nil, errors.New("invitation already accepted")
	}
	return inv, nil
}

func (s *TeamService) AcceptInvitationWithRegister(ctx context.Context, token, password, displayName string) (*AuthResult, error) {
	inv, err := s.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Check if user already exists
	_, userErr := s.store.Users().GetByEmail(ctx, inv.Email)
	if userErr == nil {
		return nil, errors.New("account already exists — log in first, then accept the invitation from your dashboard")
	}

	// Create the user
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		OrgID:        inv.OrgID,
		Email:        inv.Email,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		Role:         model.Role(inv.Role),
	}
	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, err
	}

	// Mark invitation as accepted
	now := time.Now()
	inv.AcceptedAt = &now
	_ = s.store.Invitations().Update(ctx, inv)

	// Generate tokens
	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.OrgID, string(user.Role), user.TokenVersion)
	if err != nil {
		return nil, err
	}

	s.logger.Info("invitation accepted via registration", slog.String("email", inv.Email))

	return &AuthResult{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}
