// Package jwtv5x provides a small helper for creating, validating and
// rotating JSON Web Tokens (JWT) using github.com/golang-jwt/jwt/v5.
package jwtv5x

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// RefreshTokenStore defines storage operations required for refresh tokens.
type RefreshTokenStore interface {
	// Save stores a refresh token ID (not the full JWT) for the given user
	// until expiresAt.
	Save(ctx context.Context, userID, tokenID string, expiresAt time.Time) error

	// Consume atomically verifies and consumes the refresh token ID for userID.
	// If the ID exists and matches, it should be removed and nil returned.
	// If not found, implementations should return an error that can be checked
	// to determine if the token was not found.
	Consume(ctx context.Context, userID, tokenID string) error
}

// Option configures a Manager.
type Option func(*Manager)

// WithSigningMethod sets the JWT signing method used by the Manager.
// The default signing method is jwt.SigningMethodHS256.
func WithSigningMethod(method jwt.SigningMethod) Option {
	return func(m *Manager) {
		m.signingMethod = method
	}
}

// Manager handles generation, validation and rotation of access and
// refresh tokens.
type Manager struct {
	accessTokenKey  []byte
	refreshTokenKey []byte
	signingMethod   jwt.SigningMethod
	store           RefreshTokenStore
}

// New creates a new Manager.
//
// accessTokenKey and refreshTokenKey are required and are used to sign
// access and refresh tokens respectively. store must be provided to
// persist and consume refresh tokens. Optional functional options may be
// passed to customize behavior.
func New(accessTokenKey, refreshTokenKey []byte, store RefreshTokenStore, opts ...Option) (*Manager, error) {
	if len(accessTokenKey) == 0 {
		return nil, fmt.Errorf("accessTokenKey must not be empty")
	}
	if len(refreshTokenKey) == 0 {
		return nil, fmt.Errorf("refreshTokenKey must not be empty")
	}
	if store == nil {
		return nil, fmt.Errorf("refresh token store must not be nil")
	}

	m := &Manager{
		accessTokenKey:  accessTokenKey,
		refreshTokenKey: refreshTokenKey,
		signingMethod:   jwt.SigningMethodHS256,
		store:           store,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// Generate issues a new access token and a refresh token.
//
// claims provides the access token claims and must include a Subject
// (user ID) which will be used to store the refresh token. refreshExpiry
// controls the refresh token lifetime.
func (m *Manager) Generate(ctx context.Context, claims jwt.Claims, refreshExpiry time.Duration) (string, string, error) {
	// Generate Access Token
	accessToken, err := jwt.NewWithClaims(m.signingMethod, claims).SignedString(m.accessTokenKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// Extract user ID from claims
	userID, err := claims.GetSubject()
	if err != nil {
		return "", "", fmt.Errorf("failed to get subject from claims: %w", err)
	}
	if userID == "" {
		return "", "", fmt.Errorf("cannot extract user ID (subject) from claims for refresh token storage")
	}

	// Generate Refresh Token
	refreshTokenID := uuid.New().String()
	now := time.Now()
	refreshClaims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        refreshTokenID,
		ExpiresAt: jwt.NewNumericDate(now.Add(refreshExpiry)),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	refreshToken, err := jwt.NewWithClaims(m.signingMethod, refreshClaims).SignedString(m.refreshTokenKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Save Refresh Token ID to store (not the full JWT)
	if err := m.store.Save(ctx, userID, refreshTokenID, now.Add(refreshExpiry)); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// Validate verifies an access token string and unmarshals the claims into v.
//
// Returns jwt.ErrTokenExpired when the token is expired, jwt.ErrTokenMalformed,
// jwt.ErrTokenSignatureInvalid, or other jwt package errors for verification failures.
// Returns nil on successful validation.
func (m *Manager) Validate(ctx context.Context, tokenString string, v jwt.Claims) error {
	token, err := jwt.ParseWithClaims(tokenString, v, func(token *jwt.Token) (interface{}, error) {
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.accessTokenKey, nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return jwt.ErrTokenInvalidClaims
	}

	return nil
}

// Refresh consumes the provided refresh token (atomically via the store),
// verifies it and returns a newly issued access token and refresh token.
//
// The old refresh token is consumed and cannot be reused. Returns jwt package
// errors (jwt.ErrTokenExpired, jwt.ErrTokenMalformed, etc.) for token validation
// failures, or wrapped errors for store operations.
func (m *Manager) Refresh(ctx context.Context, userID, oldRefreshTokenString string, newClaims jwt.Claims, newRefreshExpiry time.Duration) (string, string, error) {
	// 1. Verify old Refresh Token JWT (signature, expiration, etc.) first.
	//    We verify before consuming to ensure we don't remove a token when
	//    the JWT itself is invalid or expired.
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(oldRefreshTokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.refreshTokenKey, nil
	})

	if err != nil {
		return "", "", err
	}

	if !token.Valid {
		return "", "", jwt.ErrTokenInvalidClaims
	}

	// 2. Atomically consume old Refresh Token from the store (invalidate it).
	if err := m.store.Consume(ctx, userID, claims.ID); err != nil {
		return "", "", fmt.Errorf("failed to consume refresh token in store: %w", err)
	}

	// 3. Generate new Access Token
	newAccessToken, err := jwt.NewWithClaims(m.signingMethod, newClaims).SignedString(m.accessTokenKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign new access token: %w", err)
	}

	// 4. Generate new Refresh Token
	now := time.Now()
	newRefreshClaims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        uuid.New().String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(newRefreshExpiry)),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	newRefreshToken, err := jwt.NewWithClaims(m.signingMethod, newRefreshClaims).SignedString(m.refreshTokenKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign new refresh token: %w", err)
	}

	// 5. Save new Refresh Token
	if err := m.store.Save(ctx, userID, newRefreshClaims.ID, now.Add(newRefreshExpiry)); err != nil {
		return "", "", fmt.Errorf("failed to save new refresh token: %w", err)
	}

	return newAccessToken, newRefreshToken, nil
}
