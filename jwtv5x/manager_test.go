package jwtv5x

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type mockClock struct{ now time.Time }

func (c *mockClock) Now() time.Time { return c.now }

func (c *mockClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

type mockStore struct {
	saveFunc       func(ctx context.Context, userID, tokenID string, expiresAt time.Time) error
	consumeFunc    func(ctx context.Context, userID, tokenID string) error
	revokeUserFunc func(ctx context.Context, userID string) error
	savedTokens    map[string]string // userID -> last tokenID
	consumedTokens map[string]bool   // tokenID -> consumed
	revokedUsers   map[string]bool
}

func newMockStore() *mockStore {
	return &mockStore{
		savedTokens:    make(map[string]string),
		consumedTokens: make(map[string]bool),
		revokedUsers:   make(map[string]bool),
	}
}

func (s *mockStore) Save(ctx context.Context, userID, tokenID string, expiresAt time.Time) error {
	if s.saveFunc != nil {
		return s.saveFunc(ctx, userID, tokenID, expiresAt)
	}
	s.savedTokens[userID] = tokenID
	return nil
}

func (s *mockStore) Consume(ctx context.Context, userID, tokenID string) error {
	if s.consumeFunc != nil {
		return s.consumeFunc(ctx, userID, tokenID)
	}
	if s.consumedTokens[tokenID] {
		return ErrRefreshTokenUsed
	}
	if saved, ok := s.savedTokens[userID]; !ok || saved != tokenID {
		return ErrRefreshTokenNotFound
	}
	s.consumedTokens[tokenID] = true
	return nil
}

func (s *mockStore) RevokeUserTokens(ctx context.Context, userID string) error {
	if s.revokeUserFunc != nil {
		return s.revokeUserFunc(ctx, userID)
	}
	s.revokedUsers[userID] = true
	return nil
}

var (
	testAccessKey  = []byte("test-access-key-at-least-32-bytes")
	testRefreshKey = []byte("test-refresh-key-at-least-32-bytes")
	testNow        = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newTestManager(t *testing.T, store *mockStore, opts ...Option) *Manager {
	t.Helper()
	clock := &mockClock{now: testNow}
	allOpts := append([]Option{WithClock(clock)}, opts...)
	m, err := New(testAccessKey, testRefreshKey, store, allOpts...)
	require.NoError(t, err)
	return m
}

func defaultInput() GenerateInput {
	return GenerateInput{
		UserID:     "user-123",
		Roles:      []string{"admin", "editor"},
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	}
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		accessKey  []byte
		refreshKey []byte
		store      RefreshTokenStore
		opts       []Option
		wantErr    string
	}{
		{
			name:       "empty access key",
			accessKey:  nil,
			refreshKey: testRefreshKey,
			store:      newMockStore(),
			wantErr:    "accessTokenKey must not be empty",
		},
		{
			name:       "empty refresh key",
			accessKey:  testAccessKey,
			refreshKey: nil,
			store:      newMockStore(),
			wantErr:    "refreshTokenKey must not be empty",
		},
		{
			name:       "nil store",
			accessKey:  testAccessKey,
			refreshKey: testRefreshKey,
			store:      nil,
			wantErr:    "refresh token store must not be nil",
		},
		{
			name:       "valid defaults",
			accessKey:  testAccessKey,
			refreshKey: testRefreshKey,
			store:      newMockStore(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := New(tt.accessKey, tt.refreshKey, tt.store, tt.opts...)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, m)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, m)
			}
		})
	}
}

func TestNew_Options(t *testing.T) {
	t.Parallel()
	store := newMockStore()
	clock := &mockClock{now: testNow}

	m, err := New(testAccessKey, testRefreshKey, store,
		WithIssuer("my-app"),
		WithAudience("my-audience"),
		WithSigningMethod(jwt.SigningMethodHS384),
		WithClock(clock),
	)
	require.NoError(t, err)

	assert.Equal(t, "my-app", m.issuer)
	assert.Equal(t, "my-audience", m.audience)
	assert.Equal(t, jwt.SigningMethodHS384, m.signingMethod)
	assert.Equal(t, clock, m.clock)
}

func TestNew_NilOptionIgnored(t *testing.T) {
	t.Parallel()
	store := newMockStore()

	m, err := New(testAccessKey, testRefreshKey, store,
		WithSigningMethod(nil),
		WithClock(nil),
	)
	require.NoError(t, err)

	assert.Equal(t, jwt.SigningMethodHS256, m.signingMethod)
	assert.IsType(t, realClock{}, m.clock)
}

// ---------------------------------------------------------------------------
// Generate
// ---------------------------------------------------------------------------

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store, WithIssuer("test-iss"), WithAudience("test-aud"))

		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)

		// Verify access token claims.
		claims, err := m.ParseAccessToken(pair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims["uid"])
		assert.Equal(t, string(TokenTypeAccess), claims["typ"])
		assert.Equal(t, "test-iss", claims["iss"])
		aud, _ := claims["aud"].([]any)
		assert.Contains(t, aud, "test-aud")

		// Verify store received the refresh token JTI.
		assert.Contains(t, store.savedTokens, "user-123")
	})

	t.Run("empty userID", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		in := defaultInput()
		in.UserID = ""
		_, err := m.Generate(context.Background(), in)
		assert.ErrorContains(t, err, "userID must not be empty")
	})

	t.Run("zero access TTL", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		in := defaultInput()
		in.AccessTTL = 0
		_, err := m.Generate(context.Background(), in)
		assert.ErrorContains(t, err, "accessTTL must be > 0")
	})

	t.Run("negative refresh TTL", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		in := defaultInput()
		in.RefreshTTL = -1
		_, err := m.Generate(context.Background(), in)
		assert.ErrorContains(t, err, "refreshTTL must be > 0")
	})

	t.Run("extra claims added", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		in := defaultInput()
		in.ExtraClaims = map[string]any{"tenant": "acme"}

		pair, err := m.Generate(context.Background(), in)
		require.NoError(t, err)

		claims, err := m.ParseAccessToken(pair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "acme", claims["tenant"])
	})

	t.Run("extra claims cannot overwrite reserved keys", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		in := defaultInput()
		in.ExtraClaims = map[string]any{
			"uid": "evil-user",
			"typ": "refresh",
			"exp": int64(0),
			"jti": "fixed-jti",
		}

		pair, err := m.Generate(context.Background(), in)
		require.NoError(t, err)

		claims, err := m.ParseAccessToken(pair.AccessToken)
		require.NoError(t, err)
		// Reserved keys must NOT be overwritten.
		assert.Equal(t, "user-123", claims["uid"])
		assert.Equal(t, string(TokenTypeAccess), claims["typ"])
		assert.NotEqual(t, float64(0), claims["exp"])
		assert.NotEqual(t, "fixed-jti", claims["jti"])
	})

	t.Run("store save error propagates", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		store.saveFunc = func(context.Context, string, string, time.Time) error {
			return fmt.Errorf("db connection lost")
		}
		m := newTestManager(t, store)

		_, err := m.Generate(context.Background(), defaultInput())
		assert.ErrorContains(t, err, "save refresh token")
		assert.ErrorContains(t, err, "db connection lost")
	})
}

// ---------------------------------------------------------------------------
// ParseAccessToken
// ---------------------------------------------------------------------------

func TestParseAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		claims, err := m.ParseAccessToken(pair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims["uid"])
		assert.Equal(t, string(TokenTypeAccess), claims["typ"])

		roles, ok := claims["roles"].([]any)
		require.True(t, ok)
		assert.Contains(t, roles, "admin")
		assert.Contains(t, roles, "editor")
	})

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()
		clock := &mockClock{now: testNow}
		store := newMockStore()
		m, _ := New(testAccessKey, testRefreshKey, store, WithClock(clock))

		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		// Advance time past the access TTL.
		clock.Advance(16 * time.Minute)
		_, err = m.ParseAccessToken(pair.AccessToken)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, jwt.ErrTokenExpired))
	})

	t.Run("wrong signing key", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m1, _ := New(testAccessKey, testRefreshKey, store, WithClock(&mockClock{now: testNow}))
		pair, err := m1.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		// Parse with a different access key.
		m2, _ := New([]byte("different-access-key-32-bytes!!!"), testRefreshKey, newMockStore(), WithClock(&mockClock{now: testNow}))
		_, err = m2.ParseAccessToken(pair.AccessToken)
		assert.Error(t, err)
	})

	t.Run("refresh token rejected as access", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		// Using refresh token string with access key won't parse (different key).
		_, err = m.ParseAccessToken(pair.RefreshToken)
		assert.Error(t, err)
	})

	t.Run("tampered token", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		tampered := pair.AccessToken + "tampered"
		_, err = m.ParseAccessToken(tampered)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

func TestRefresh(t *testing.T) {
	t.Parallel()

	t.Run("success rotates tokens", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store)
		ctx := context.Background()

		// Generate initial pair.
		pair1, err := m.Generate(ctx, defaultInput())
		require.NoError(t, err)

		// Refresh.
		pair2, err := m.Refresh(ctx, RefreshInput{
			RefreshToken: pair1.RefreshToken,
			Roles:        []string{"admin"},
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		require.NoError(t, err)

		// New tokens must differ from old.
		assert.NotEqual(t, pair1.AccessToken, pair2.AccessToken)
		assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)

		// New access token must be valid.
		claims, err := m.ParseAccessToken(pair2.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims["uid"])
	})

	t.Run("reuse after consume fails", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store)
		ctx := context.Background()

		pair, err := m.Generate(ctx, defaultInput())
		require.NoError(t, err)

		// First refresh succeeds.
		_, err = m.Refresh(ctx, RefreshInput{
			RefreshToken: pair.RefreshToken,
			Roles:        []string{"admin"},
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		require.NoError(t, err)

		// Second refresh with same token fails (replay attack).
		_, err = m.Refresh(ctx, RefreshInput{
			RefreshToken: pair.RefreshToken,
			Roles:        []string{"admin"},
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		assert.ErrorContains(t, err, "consume refresh token")
		assert.ErrorIs(t, err, ErrRefreshTokenUsed)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		_, err := m.Refresh(context.Background(), RefreshInput{
			RefreshToken: "invalid-token",
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		assert.Error(t, err)
	})

	t.Run("access token rejected as refresh", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		pair, err := m.Generate(context.Background(), defaultInput())
		require.NoError(t, err)

		// Access token used in refresh flow must be rejected.
		_, err = m.Refresh(context.Background(), RefreshInput{
			RefreshToken: pair.AccessToken,
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		assert.Error(t, err)
	})

	t.Run("zero access TTL", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		_, err := m.Refresh(context.Background(), RefreshInput{
			RefreshToken: "any",
			AccessTTL:    0,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		assert.ErrorContains(t, err, "accessTTL must be > 0")
	})

	t.Run("zero refresh TTL", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		_, err := m.Refresh(context.Background(), RefreshInput{
			RefreshToken: "any",
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   0,
		})
		assert.ErrorContains(t, err, "refreshTTL must be > 0")
	})

	t.Run("store consume error propagates", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store)
		ctx := context.Background()

		pair, err := m.Generate(ctx, defaultInput())
		require.NoError(t, err)

		store.consumeFunc = func(context.Context, string, string) error {
			return fmt.Errorf("redis timeout")
		}

		_, err = m.Refresh(ctx, RefreshInput{
			RefreshToken: pair.RefreshToken,
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
		})
		assert.ErrorContains(t, err, "consume refresh token")
		assert.ErrorContains(t, err, "redis timeout")
	})

	t.Run("refresh with extra claims", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store)
		ctx := context.Background()

		pair, err := m.Generate(ctx, defaultInput())
		require.NoError(t, err)

		pair2, err := m.Refresh(ctx, RefreshInput{
			RefreshToken: pair.RefreshToken,
			Roles:        []string{"viewer"},
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   7 * 24 * time.Hour,
			ExtraClaims:  map[string]any{"tenant": "acme"},
		})
		require.NoError(t, err)

		claims, err := m.ParseAccessToken(pair2.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "acme", claims["tenant"])
		roles, _ := claims["roles"].([]any)
		assert.Contains(t, roles, "viewer")
	})
}

// ---------------------------------------------------------------------------
// RevokeUserRefreshTokens
// ---------------------------------------------------------------------------

func TestRevokeUserRefreshTokens(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		m := newTestManager(t, store)

		err := m.RevokeUserRefreshTokens(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, store.revokedUsers["user-123"])
	})

	t.Run("empty userID", func(t *testing.T) {
		t.Parallel()
		m := newTestManager(t, newMockStore())
		err := m.RevokeUserRefreshTokens(context.Background(), "")
		assert.ErrorContains(t, err, "userID must not be empty")
	})

	t.Run("store error propagates", func(t *testing.T) {
		t.Parallel()
		store := newMockStore()
		store.revokeUserFunc = func(context.Context, string) error {
			return fmt.Errorf("store failure")
		}
		m := newTestManager(t, store)

		err := m.RevokeUserRefreshTokens(context.Background(), "user-123")
		assert.ErrorContains(t, err, "store failure")
	})
}

// ---------------------------------------------------------------------------
// Full login/refresh/logout flow
// ---------------------------------------------------------------------------

func TestFullFlow_LoginRefreshLogout(t *testing.T) {
	t.Parallel()
	store := newMockStore()
	m := newTestManager(t, store, WithIssuer("myapp"), WithAudience("web"))
	ctx := context.Background()

	// 1. Login: generate initial token pair.
	pair, err := m.Generate(ctx, GenerateInput{
		UserID:     "user-42",
		Roles:      []string{"user"},
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// 2. Use access token.
	claims, err := m.ParseAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-42", claims["uid"])

	// 3. Refresh: old refresh consumed, new pair issued.
	pair2, err := m.Refresh(ctx, RefreshInput{
		RefreshToken: pair.RefreshToken,
		Roles:        []string{"user"},
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   24 * time.Hour,
	})
	require.NoError(t, err)
	assert.NotEqual(t, pair.AccessToken, pair2.AccessToken)

	// Old refresh token cannot be reused.
	_, err = m.Refresh(ctx, RefreshInput{
		RefreshToken: pair.RefreshToken,
		Roles:        []string{"user"},
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   24 * time.Hour,
	})
	assert.ErrorIs(t, err, ErrRefreshTokenUsed)

	// 4. Logout: revoke all refresh tokens.
	err = m.RevokeUserRefreshTokens(ctx, "user-42")
	require.NoError(t, err)
	assert.True(t, store.revokedUsers["user-42"])
}
