package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrTokenExpired = errors.New("token has expired")
)

// Claims represents the JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"uid"`
	OrgID  uuid.UUID `json:"oid"`
	Role   string    `json:"role"`
}

// JWTManager handles JWT token generation and validation.
type JWTManager struct {
	secret        []byte
	tokenExpiry   time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, tokenExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		tokenExpiry:   tokenExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// GenerateTokenPair creates a new access + refresh token pair.
func (m *JWTManager) GenerateTokenPair(userID, orgID uuid.UUID, role string) (*TokenPair, error) {
	now := time.Now()

	// Access token
	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.tokenExpiry)),
			Issuer:    "sailbox",
		},
		UserID: userID,
		OrgID:  orgID,
		Role:   role,
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	// Refresh token (longer lived, minimal claims)
	refreshClaims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
		Issuer:    "sailbox-refresh",
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(m.tokenExpiry),
	}, nil
}

// ValidateAccessToken parses and validates an access token.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateRefreshToken parses and validates a refresh token, returning the user ID.
func (m *JWTManager) ValidateRefreshToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid || claims.Issuer != "sailbox-refresh" {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return userID, nil
}
