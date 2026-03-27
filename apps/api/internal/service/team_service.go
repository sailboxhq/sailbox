package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type TeamService struct {
	store  store.Store
	logger *slog.Logger
}

func NewTeamService(s store.Store, logger *slog.Logger) *TeamService {
	return &TeamService{store: s, logger: logger}
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

	if err := s.store.Users().UpdateRole(ctx, userID, role); err != nil {
		return err
	}

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

	// Remove project-level memberships for this user
	memberships, err := s.store.ProjectMembers().ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, m := range memberships {
		if err := s.store.ProjectMembers().Delete(ctx, m.ProjectID, userID); err != nil {
			s.logger.Warn("failed to remove project member during org removal",
				slog.String("project_id", m.ProjectID.String()),
				slog.String("user_id", userID.String()),
				slog.Any("error", err),
			)
		}
	}

	// Remove user from the organization
	if err := s.store.Users().RemoveFromOrg(ctx, userID); err != nil {
		return err
	}

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

func (s *TeamService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) error {
	inv, err := s.store.Invitations().GetByToken(ctx, token)
	if err != nil {
		return errors.New("invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		return errors.New("invitation has expired")
	}

	if inv.AcceptedAt != nil {
		return errors.New("invitation has already been accepted")
	}

	// Verify the accepting user's email matches the invitation
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	if user.Email != inv.Email {
		return errors.New("invitation was sent to a different email address")
	}

	user.OrgID = inv.OrgID
	user.Role = model.Role(inv.Role)
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	// Mark invitation as accepted
	now := time.Now()
	inv.AcceptedAt = &now
	if err := s.store.Invitations().Update(ctx, inv); err != nil {
		return err
	}

	s.logger.Info("invitation accepted",
		slog.String("user_id", userID.String()),
		slog.String("org_id", inv.OrgID.String()),
	)
	return nil
}

func (s *TeamService) ListInvitations(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
	return s.store.Invitations().ListByOrg(ctx, orgID)
}

func (s *TeamService) CancelInvitation(ctx context.Context, invID uuid.UUID) error {
	if err := s.store.Invitations().Delete(ctx, invID); err != nil {
		return err
	}

	s.logger.Info("invitation cancelled", slog.String("invitation_id", invID.String()))
	return nil
}

// ============================================================================
// Project Access (for Member role)
// ============================================================================

func (s *TeamService) SetProjectAccess(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "viewer" {
		return errors.New("project role must be 'admin' or 'viewer'")
	}

	// Check if membership already exists
	existing, err := s.store.ProjectMembers().GetByProjectAndUser(ctx, projectID, userID)
	if err == nil {
		// Update existing
		existing.Role = role
		// Delete and re-create since ProjectMember has no Update method — use delete+create
		if err := s.store.ProjectMembers().Delete(ctx, projectID, userID); err != nil {
			return err
		}
	}

	pm := &model.ProjectMember{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
	}
	return s.store.ProjectMembers().Create(ctx, pm)
}

func (s *TeamService) RemoveProjectAccess(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.store.ProjectMembers().Delete(ctx, projectID, userID)
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
