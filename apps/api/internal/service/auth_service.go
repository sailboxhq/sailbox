package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
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

type AuthService struct {
	store      store.Store
	jwtManager *auth.JWTManager
	logger     *slog.Logger
}

func NewAuthService(s store.Store, jwtManager *auth.JWTManager, logger *slog.Logger) *AuthService {
	return &AuthService{store: s, jwtManager: jwtManager, logger: logger}
}

type RegisterInput struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
	OrgName     string `json:"org_name" binding:"required"`
}

type LoginInput struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required"`
	TwoFACode string `json:"two_fa_code"` // Required if user has 2FA enabled
}

type AuthResult struct {
	User         *model.User `json:"user,omitempty"`
	AccessToken  string      `json:"access_token,omitempty"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time   `json:"expires_at,omitempty"`
	Requires2FA  bool        `json:"requires_2fa,omitempty"`
}

type SetupStatus struct {
	Initialized bool `json:"initialized"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *AuthService) GetSetupStatus(ctx context.Context) (*SetupStatus, error) {
	count, err := s.store.Users().Count(ctx)
	if err != nil {
		return nil, err
	}
	return &SetupStatus{Initialized: count > 0}, nil
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	count, _ := s.store.Users().Count(ctx)
	if count > 0 {
		return nil, errors.New("registration is disabled — use team invitation to join")
	}

	_, err := s.store.Users().GetByEmail(ctx, input.Email)
	if err == nil {
		return nil, errors.New("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	org := &model.Organization{Name: input.OrgName}
	if err := s.store.Organizations().Create(ctx, org); err != nil {
		return nil, err
	}

	user := &model.User{
		OrgID:        org.ID,
		Email:        input.Email,
		PasswordHash: string(hash),
		DisplayName:  input.DisplayName,
		Role:         model.RoleOwner,
	}
	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, err
	}

	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, org.ID, string(user.Role))
	if err != nil {
		return nil, err
	}

	s.logger.Info("user registered", slog.String("email", input.Email))

	return &AuthResult{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	user, err := s.store.Users().GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// If 2FA enabled and no code provided, return partial response
	if user.TwoFAEnabled && input.TwoFACode == "" {
		return &AuthResult{Requires2FA: true}, nil
	}

	// If 2FA enabled and code provided, verify
	if user.TwoFAEnabled {
		if !verifyTOTP(user.TwoFASecret, input.TwoFACode) {
			return nil, errors.New("invalid two-factor authentication code")
		}
	}

	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.OrgID, string(user.Role))
	if err != nil {
		return nil, err
	}

	s.logger.Info("user logged in", slog.String("email", input.Email))

	return &AuthResult{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (*AuthResult, error) {
	userID, err := s.jwtManager.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.OrgID, string(user.Role))
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.store.Users().GetByID(ctx, userID)
}

// ============================================================================
// Profile Management
// ============================================================================

type UpdateProfileInput struct {
	FirstName   *string `json:"first_name"`
	LastName    *string `json:"last_name"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

type Setup2FAResponse struct {
	Secret string `json:"secret"`
	QRCode string `json:"qr_code"`
}

type Verify2FAInput struct {
	Code string `json:"code" binding:"required,len=6"`
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*model.User, error) {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		user.LastName = *input.LastName
	}
	if input.DisplayName != nil {
		user.DisplayName = *input.DisplayName
	}
	if input.AvatarURL != nil {
		if *input.AvatarURL != "" && !model.IsValidAvatar(*input.AvatarURL) {
			return nil, errors.New("invalid avatar selection")
		}
		user.AvatarURL = *input.AvatarURL
	}

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, err
	}

	s.logger.Info("user profile updated", slog.String("user_id", userID.String()))
	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, input ChangePasswordInput) error {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	s.logger.Info("user password changed", slog.String("user_id", userID.String()))
	return nil
}

func (s *AuthService) Setup2FA(ctx context.Context, userID uuid.UUID) (*Setup2FAResponse, error) {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.TwoFAEnabled {
		return nil, errors.New("2FA is already enabled")
	}

	secret := generateTOTPSecret()
	user.TwoFASecret = secret

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, err
	}

	qrCode := fmt.Sprintf("otpauth://totp/Sailbox:%s?secret=%s&issuer=Sailbox&digits=6&period=30", user.Email, secret)

	s.logger.Info("2FA setup initiated", slog.String("user_id", userID.String()))
	return &Setup2FAResponse{
		Secret: secret,
		QRCode: qrCode,
	}, nil
}

func (s *AuthService) Verify2FA(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.TwoFASecret == "" {
		return errors.New("2FA has not been set up; call setup first")
	}

	if !verifyTOTP(user.TwoFASecret, code) {
		return errors.New("invalid 2FA code")
	}

	user.TwoFAEnabled = true
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	s.logger.Info("2FA enabled", slog.String("user_id", userID.String()))
	return nil
}

func (s *AuthService) Disable2FA(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if !user.TwoFAEnabled {
		return errors.New("2FA is not enabled")
	}

	if !verifyTOTP(user.TwoFASecret, code) {
		return errors.New("invalid 2FA code")
	}

	user.TwoFAEnabled = false
	user.TwoFASecret = ""
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	s.logger.Info("2FA disabled", slog.String("user_id", userID.String()))
	return nil
}

func (s *AuthService) ListAvatars() []string {
	return model.PredefinedAvatars
}

// ============================================================================
// TOTP helpers
// ============================================================================

func generateTOTPSecret() string {
	secret := make([]byte, 20)
	_, _ = rand.Read(secret)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

func verifyTOTP(secret, code string) bool {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false
	}
	now := time.Now().Unix() / 30
	for _, offset := range []int64{-1, 0, 1} {
		if generateTOTPCode(key, now+offset) == code {
			return true
		}
	}
	return false
}

func generateTOTPCode(key []byte, counter int64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", truncated%1000000)
}
