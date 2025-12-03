// Package jwtv5x provides a small helper for creating, validating and
// rotating JSON Web Tokens (JWT) using github.com/golang-jwt/jwt/v5.
package jwtv5x

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mockRefreshTokenStore is a simple in-memory store for testing
type mockRefreshTokenStore struct {
	tokens map[string]tokenEntry
}

type tokenEntry struct {
	tokenID   string
	expiresAt time.Time
}

func newMockStore() *mockRefreshTokenStore {
	return &mockRefreshTokenStore{
		tokens: make(map[string]tokenEntry),
	}
}

func (m *mockRefreshTokenStore) Save(ctx context.Context, userID, tokenID string, expiresAt time.Time) error {
	m.tokens[userID] = tokenEntry{tokenID: tokenID, expiresAt: expiresAt}
	return nil
}

func (m *mockRefreshTokenStore) Consume(ctx context.Context, userID, tokenID string) error {
	entry, exists := m.tokens[userID]
	if !exists {
		return errors.New("token not found")
	}
	if entry.tokenID != tokenID {
		return errors.New("token not found")
	}
	delete(m.tokens, userID)
	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name            string
		accessKey       []byte
		refreshKey      []byte
		store           RefreshTokenStore
		opts            []Option
		wantErr         bool
		errContains     string
		checkSignMethod jwt.SigningMethod
	}{
		{
			name:            "valid manager with default signing method",
			accessKey:       []byte("access-secret"),
			refreshKey:      []byte("refresh-secret"),
			store:           newMockStore(),
			wantErr:         false,
			checkSignMethod: jwt.SigningMethodHS256,
		},
		{
			name:            "valid manager with custom signing method",
			accessKey:       []byte("access-secret"),
			refreshKey:      []byte("refresh-secret"),
			store:           newMockStore(),
			opts:            []Option{WithSigningMethod(jwt.SigningMethodHS512)},
			wantErr:         false,
			checkSignMethod: jwt.SigningMethodHS512,
		},
		{
			name:        "empty access token key",
			accessKey:   []byte{},
			refreshKey:  []byte("refresh-secret"),
			store:       newMockStore(),
			wantErr:     true,
			errContains: "accessTokenKey must not be empty",
		},
		{
			name:        "empty refresh token key",
			accessKey:   []byte("access-secret"),
			refreshKey:  []byte{},
			store:       newMockStore(),
			wantErr:     true,
			errContains: "refreshTokenKey must not be empty",
		},
		{
			name:        "nil store",
			accessKey:   []byte("access-secret"),
			refreshKey:  []byte("refresh-secret"),
			store:       nil,
			wantErr:     true,
			errContains: "refresh token store must not be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(tt.accessKey, tt.refreshKey, tt.store, tt.opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want contains %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m == nil {
				t.Fatal("manager is nil")
			}
			if tt.checkSignMethod != nil && m.signingMethod != tt.checkSignMethod {
				t.Errorf("signingMethod = %v, want %v", m.signingMethod, tt.checkSignMethod)
			}
		})
	}
}

func TestManager_Generate(t *testing.T) {
	ctx := context.Background()
	accessKey := []byte("access-secret")
	refreshKey := []byte("refresh-secret")
	store := newMockStore()

	m, err := New(accessKey, refreshKey, store)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	tests := []struct {
		name          string
		claims        jwt.Claims
		refreshExpiry time.Duration
		wantErr       bool
		errContains   string
	}{
		{
			name: "valid generation with RegisteredClaims",
			claims: jwt.RegisteredClaims{
				Subject:   "user123",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       false,
		},
		{
			name: "valid generation with MapClaims",
			claims: jwt.MapClaims{
				"sub": "user456",
				"exp": time.Now().Add(15 * time.Minute).Unix(),
				"iat": time.Now().Unix(),
			},
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       false,
		},
		{
			name: "missing subject in claims",
			claims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       true,
			errContains:   "cannot extract user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessToken, refreshToken, err := m.Generate(ctx, tt.claims, tt.refreshExpiry)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want contains %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if accessToken == "" {
				t.Error("accessToken is empty")
			}
			if refreshToken == "" {
				t.Error("refreshToken is empty")
			}

			// Verify refresh token ID is stored
			userID, _ := tt.claims.GetSubject()
			if _, exists := store.tokens[userID]; !exists {
				t.Error("refresh token not saved to store")
			}
		})
	}
}

func TestManager_Validate(t *testing.T) {
	ctx := context.Background()
	accessKey := []byte("access-secret")
	refreshKey := []byte("refresh-secret")
	store := newMockStore()

	m, err := New(accessKey, refreshKey, store)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Generate valid tokens for testing
	validClaims := jwt.RegisteredClaims{
		Subject:   "user123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	validToken, _, err := m.Generate(ctx, validClaims, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	// Generate expired token
	expiredClaims := jwt.RegisteredClaims{
		Subject:   "user456",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(accessKey)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	// Generate token with wrong key
	wrongKeyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString([]byte("wrong-key"))
	if err != nil {
		t.Fatalf("failed to generate wrong key token: %v", err)
	}

	tests := []struct {
		name        string
		tokenString string
		wantErr     error
	}{
		{
			name:        "valid token",
			tokenString: validToken,
			wantErr:     nil,
		},
		{
			name:        "expired token",
			tokenString: expiredToken,
			wantErr:     jwt.ErrTokenExpired,
		},
		{
			name:        "invalid signature",
			tokenString: wrongKeyToken,
			wantErr:     jwt.ErrTokenSignatureInvalid,
		},
		{
			name:        "malformed token",
			tokenString: "not.a.token",
			wantErr:     jwt.ErrTokenMalformed,
		},
		{
			name:        "empty token",
			tokenString: "",
			wantErr:     jwt.ErrTokenMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &jwt.RegisteredClaims{}
			err := m.Validate(ctx, tt.tokenString, claims)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Refresh(t *testing.T) {
	ctx := context.Background()
	accessKey := []byte("access-secret")
	refreshKey := []byte("refresh-secret")
	store := newMockStore()

	m, err := New(accessKey, refreshKey, store)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Generate initial tokens
	userID := "user123"
	initialClaims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	_, initialRefreshToken, err := m.Generate(ctx, initialClaims, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate initial tokens: %v", err)
	}

	// Extract initial refresh token ID for later verification
	initialRC := &jwt.RegisteredClaims{}
	_, parseErr := jwt.ParseWithClaims(initialRefreshToken, initialRC, func(token *jwt.Token) (interface{}, error) {
		return refreshKey, nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse initial refresh token: %v", parseErr)
	}
	initialRefreshTokenID := initialRC.ID

	tests := []struct {
		name             string
		userID           string
		refreshToken     string
		newClaims        jwt.Claims
		newRefreshExpiry time.Duration
		setupFn          func()
		wantErr          error
		errContains      string
	}{
		{
			name:         "successful refresh",
			userID:       userID,
			refreshToken: initialRefreshToken,
			newClaims: jwt.RegisteredClaims{
				Subject:   userID,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			newRefreshExpiry: 7 * 24 * time.Hour,
			wantErr:          nil,
		},
		{
			name:   "refresh token not found",
			userID: "nonexistent",
			setupFn: func() {
				// Create a valid JWT with a user that doesn't exist in store
			},
			refreshToken: func() string {
				claims := jwt.RegisteredClaims{
					Subject:   "nonexistent",
					ID:        "nonexistent-token-id",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				}
				token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(refreshKey)
				return token
			}(),
			newClaims: jwt.RegisteredClaims{
				Subject:   "nonexistent",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
			newRefreshExpiry: 7 * 24 * time.Hour,
			errContains:      "failed to consume refresh token",
		},
		{
			name:   "refresh token already consumed",
			userID: userID,
			setupFn: func() {
				// Generate new tokens for this test
				claims := jwt.RegisteredClaims{
					Subject:   userID,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				}
				_, _, _ = m.Generate(ctx, claims, 7*24*time.Hour)
			},
			refreshToken: initialRefreshToken,
			newClaims: jwt.RegisteredClaims{
				Subject:   userID,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
			newRefreshExpiry: 7 * 24 * time.Hour,
			errContains:      "failed to consume refresh token",
		},
		{
			name:   "expired refresh token",
			userID: "user_expired",
			setupFn: func() {
				// Generate expired refresh token with ID and save ID in store
				expiredClaims := jwt.RegisteredClaims{
					Subject:   "user_expired",
					ID:        "expired-token-id-1",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				}
				_, _ = jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(refreshKey)
				store.Save(ctx, "user_expired", expiredClaims.ID, time.Now().Add(-1*time.Hour))
			},
			refreshToken: func() string {
				expiredClaims := jwt.RegisteredClaims{
					Subject:   "user_expired",
					ID:        "expired-token-id-1",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				}
				token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(refreshKey)
				return token
			}(),
			newClaims: jwt.RegisteredClaims{
				Subject:   "user_expired",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
			newRefreshExpiry: 7 * 24 * time.Hour,
			wantErr:          jwt.ErrTokenExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFn != nil {
				tt.setupFn()
			}

			newAccessToken, newRefreshToken, err := m.Refresh(ctx, tt.userID, tt.refreshToken, tt.newClaims, tt.newRefreshExpiry)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Refresh() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want contains %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if newAccessToken == "" {
				t.Error("new access token is empty")
			}
			if newRefreshToken == "" {
				t.Error("new refresh token is empty")
			}

			// Verify old refresh token is consumed (using its token ID)
			oldID := initialRefreshTokenID
			if tt.userID != userID {
				// For non-initial user cases, parse the provided token to get its ID
				rc := &jwt.RegisteredClaims{}
				_, _ = jwt.ParseWithClaims(tt.refreshToken, rc, func(token *jwt.Token) (interface{}, error) {
					return refreshKey, nil
				})
				oldID = rc.ID
			}

			if err := store.Consume(ctx, tt.userID, oldID); err == nil {
				t.Error("old refresh token was not consumed")
			}

			// Verify new refresh token is stored
			if _, exists := store.tokens[tt.userID]; !exists {
				t.Error("new refresh token not saved to store")
			}
		})
	}
}

func TestManager_WithSigningMethod(t *testing.T) {
	ctx := context.Background()
	accessKey := []byte("access-secret")
	refreshKey := []byte("refresh-secret")
	store := newMockStore()

	methods := []jwt.SigningMethod{
		jwt.SigningMethodHS256,
		jwt.SigningMethodHS384,
		jwt.SigningMethodHS512,
	}

	for _, method := range methods {
		t.Run(method.Alg(), func(t *testing.T) {
			m, err := New(accessKey, refreshKey, store, WithSigningMethod(method))
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			claims := jwt.RegisteredClaims{
				Subject:   "user123",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			}

			accessToken, _, err := m.Generate(ctx, claims, 7*24*time.Hour)
			if err != nil {
				t.Fatalf("failed to generate token: %v", err)
			}

			// Verify token uses correct signing method
			parsedClaims := &jwt.RegisteredClaims{}
			err = m.Validate(ctx, accessToken, parsedClaims)
			if err != nil {
				t.Errorf("failed to validate token: %v", err)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
