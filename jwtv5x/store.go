package jwtv5x

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenUsed     = errors.New("refresh token already used or revoked")
)

// RefreshTokenStore persists refresh token JTIs (not the raw JWT) server-side.
// Implementations must be safe for concurrent use.
type RefreshTokenStore interface {
	// Save stores a refresh token JTI with its expiration time.
	Save(ctx context.Context, userID, tokenID string, expiresAt time.Time) error
	// Consume marks a refresh token as used. Returns ErrRefreshTokenUsed if already consumed.
	Consume(ctx context.Context, userID, tokenID string) error
	// RevokeUserTokens invalidates all refresh tokens for the given user.
	RevokeUserTokens(ctx context.Context, userID string) error
}
