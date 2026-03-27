package jwtv5x

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidTokenType = errors.New("invalid token type")
)

// reservedClaims are claim keys that ExtraClaims must not overwrite.
var reservedClaims = map[string]struct{}{
	"uid": {}, "typ": {}, "roles": {},
	"iat": {}, "nbf": {}, "exp": {}, "jti": {},
	"iss": {}, "aud": {}, "sub": {},
}

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Option configures a Manager.
type Option func(*Manager)

func WithSigningMethod(method jwt.SigningMethod) Option {
	return func(m *Manager) {
		if method != nil {
			m.signingMethod = method
		}
	}
}

func WithIssuer(issuer string) Option {
	return func(m *Manager) { m.issuer = issuer }
}

func WithAudience(audience string) Option {
	return func(m *Manager) { m.audience = audience }
}

func WithClock(clock Clock) Option {
	return func(m *Manager) {
		if clock != nil {
			m.clock = clock
		}
	}
}

// Manager handles JWT token generation, parsing, refresh, and revocation.
type Manager struct {
	accessTokenKey  []byte
	refreshTokenKey []byte
	signingMethod   jwt.SigningMethod
	issuer          string
	audience        string
	store           RefreshTokenStore
	clock           Clock
}

// GenerateInput holds parameters for generating a token pair.
type GenerateInput struct {
	UserID      string
	Roles       []string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	ExtraClaims map[string]any
}

// RefreshInput holds parameters for refreshing a token pair.
type RefreshInput struct {
	RefreshToken string
	Roles        []string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	ExtraClaims  map[string]any
}

// TokenPair contains an access token and a refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// New creates a Manager. Both keys and store are required.
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
		clock:           realClock{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Generate creates a new access/refresh token pair and persists the refresh token JTI.
func (m *Manager) Generate(ctx context.Context, in GenerateInput) (*TokenPair, error) {
	if in.UserID == "" {
		return nil, fmt.Errorf("userID must not be empty")
	}
	if in.AccessTTL <= 0 {
		return nil, fmt.Errorf("accessTTL must be > 0")
	}
	if in.RefreshTTL <= 0 {
		return nil, fmt.Errorf("refreshTTL must be > 0")
	}

	now := m.clock.Now()

	// Build access token claims.
	accessClaims := jwt.MapClaims{
		"uid":   in.UserID,
		"typ":   string(TokenTypeAccess),
		"roles": in.Roles,
		"iat":   now.Unix(),
		"nbf":   now.Unix(),
		"exp":   now.Add(in.AccessTTL).Unix(),
		"jti":   uuid.NewString(),
	}
	if m.issuer != "" {
		accessClaims["iss"] = m.issuer
	}
	if m.audience != "" {
		accessClaims["aud"] = []string{m.audience}
	}
	for k, v := range in.ExtraClaims {
		if _, ok := reservedClaims[k]; !ok {
			accessClaims[k] = v
		}
	}

	accessToken, err := jwt.NewWithClaims(m.signingMethod, accessClaims).SignedString(m.accessTokenKey)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// Build refresh token claims.
	refreshJTI := uuid.NewString()
	refreshExp := now.Add(in.RefreshTTL)

	refreshTokenClaims := jwt.MapClaims{
		"sub": in.UserID,
		"jti": refreshJTI,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": refreshExp.Unix(),
		"typ": string(TokenTypeRefresh),
	}
	if m.issuer != "" {
		refreshTokenClaims["iss"] = m.issuer
	}
	if m.audience != "" {
		refreshTokenClaims["aud"] = []string{m.audience}
	}

	refreshToken, err := jwt.NewWithClaims(m.signingMethod, refreshTokenClaims).SignedString(m.refreshTokenKey)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	if err := m.store.Save(ctx, in.UserID, refreshJTI, refreshExp); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ParseAccessToken validates an access token string and returns its claims.
func (m *Manager) ParseAccessToken(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.accessTokenKey, nil
	}, jwt.WithTimeFunc(m.clock.Now))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	typ, _ := claims["typ"].(string)
	if TokenType(typ) != TokenTypeAccess {
		return nil, ErrInvalidTokenType
	}
	return claims, nil
}

// refreshClaims is used internally to parse refresh tokens with typed fields.
type refreshClaims struct {
	Type string `json:"typ"`
	jwt.RegisteredClaims
}

func (m *Manager) parseRefreshToken(tokenString string) (*refreshClaims, error) {
	claims := &refreshClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.refreshTokenKey, nil
	}, jwt.WithTimeFunc(m.clock.Now))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if TokenType(claims.Type) != TokenTypeRefresh {
		return nil, ErrInvalidTokenType
	}
	return claims, nil
}

// Refresh consumes the old refresh token (one-time use) and generates a new token pair.
func (m *Manager) Refresh(ctx context.Context, in RefreshInput) (*TokenPair, error) {
	if in.AccessTTL <= 0 {
		return nil, fmt.Errorf("accessTTL must be > 0")
	}
	if in.RefreshTTL <= 0 {
		return nil, fmt.Errorf("refreshTTL must be > 0")
	}

	old, err := m.parseRefreshToken(in.RefreshToken)
	if err != nil {
		return nil, err
	}
	if old.Subject == "" {
		return nil, fmt.Errorf("refresh token subject is empty")
	}
	if old.ID == "" {
		return nil, fmt.Errorf("refresh token jti is empty")
	}

	if err := m.store.Consume(ctx, old.Subject, old.ID); err != nil {
		return nil, fmt.Errorf("consume refresh token: %w", err)
	}

	return m.Generate(ctx, GenerateInput{
		UserID:      old.Subject,
		Roles:       in.Roles,
		AccessTTL:   in.AccessTTL,
		RefreshTTL:  in.RefreshTTL,
		ExtraClaims: in.ExtraClaims,
	})
}

// RevokeUserRefreshTokens invalidates all refresh tokens for the given user (logout).
func (m *Manager) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID must not be empty")
	}
	return m.store.RevokeUserTokens(ctx, userID)
}
