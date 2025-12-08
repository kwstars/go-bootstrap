package goredisx

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  RedisConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: RedisConfig{
				Addr: "localhost:6379",
				DB:   0,
			},
			wantErr: false,
		},
		{
			name: "empty addr",
			config: RedisConfig{
				Addr: "",
				DB:   0,
			},
			wantErr: true,
		},
		{
			name: "negative db",
			config: RedisConfig{
				Addr: "localhost:6379",
				DB:   -1,
			},
			wantErr: true,
		},
		{
			name: "valid config with username and password",
			config: RedisConfig{
				Addr:     "localhost:6379",
				DB:       0,
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	t.Skip("Skipping functional test - requires Redis server")

	config := RedisConfig{
		Addr: "localhost:6379",
		DB:   0,
	}

	client, err := NewStandaloneClient(config)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()
	assert.NotNil(t, client)

	// Test with custom StandaloneOptions
	clientWithOpts, err := NewStandaloneClient(
		config,
		WithStandaloneDialTimeout(10*time.Second),
		WithStandalonePoolSize(20),
		WithStandaloneMinIdleConns(10),
		WithStandalonePoolTimeout(8*time.Second),
		WithStandaloneClientName("test-app"),
	)
	require.NoError(t, err)
	defer func() {
		_ = clientWithOpts.Close()
	}()
	assert.NotNil(t, clientWithOpts)
}

func TestNewClient_InvalidConfig(t *testing.T) {
	t.Parallel()

	invalidConfig := RedisConfig{
		Addr: "",
		DB:   -1,
	}

	_, err := NewStandaloneClient(invalidConfig)
	assert.Error(t, err)
}

func TestHealthCheck(t *testing.T) {
	t.Skip("Skipping functional test - requires Redis server")

	config := RedisConfig{
		Addr: "localhost:6379",
		DB:   0,
	}

	client, err := NewStandaloneClient(config)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	ctx := context.Background()
	err = HealthCheck(ctx, client)
	assert.NoError(t, err)

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = HealthCheck(cancelledCtx, client)
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	t.Skip("Skipping functional test - requires Redis server")

	config := RedisConfig{
		Addr: "localhost:6379",
		DB:   0,
	}

	client, err := NewStandaloneClient(config)
	require.NoError(t, err)

	err = Close(client)
	assert.NoError(t, err)
}

func TestWithStandaloneTLSConfig(t *testing.T) {
	t.Parallel()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	opt := WithStandaloneTLSConfig(tlsConfig)
	redisOpts := &redis.Options{}
	err := opt(redisOpts)
	assert.NoError(t, err)
	assert.Equal(t, tlsConfig, redisOpts.TLSConfig)
}

func TestWithStandaloneWriteTimeout(t *testing.T) {
	t.Parallel()

	timeout := 15 * time.Second
	opt := WithStandaloneWriteTimeout(timeout)
	redisOpts := &redis.Options{}
	err := opt(redisOpts)
	assert.NoError(t, err)
	assert.Equal(t, timeout, redisOpts.WriteTimeout)
}

func TestWithStandaloneAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "valid addr",
			addr:    "localhost:6379",
			wantErr: false,
		},
		{
			name:    "empty addr",
			addr:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneAddr(tt.addr)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.addr, redisOpts.Addr)
			}
		})
	}
}

func TestWithStandaloneDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		db      int
		wantErr bool
	}{
		{
			name:    "valid db",
			db:      5,
			wantErr: false,
		},
		{
			name:    "zero db",
			db:      0,
			wantErr: false,
		},
		{
			name:    "negative db",
			db:      -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneDB(tt.db)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.db, redisOpts.DB)
			}
		})
	}
}

func TestWithStandaloneUsername(t *testing.T) {
	t.Parallel()

	username := "testuser"
	opt := WithStandaloneUsername(username)
	redisOpts := &redis.Options{}
	err := opt(redisOpts)
	assert.NoError(t, err)
	assert.Equal(t, username, redisOpts.Username)
}

func TestWithPassword(t *testing.T) {
	t.Parallel()

	password := "testpass"
	opt := WithPassword(password)
	redisOpts := &redis.Options{}
	err := opt(redisOpts)
	assert.NoError(t, err)
	assert.Equal(t, password, redisOpts.Password)
}

func TestWithStandaloneDialTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: -1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneDialTimeout(tt.timeout)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.timeout, redisOpts.DialTimeout)
			}
		})
	}
}

func TestWithStandaloneReadTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: -1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneReadTimeout(tt.timeout)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.timeout, redisOpts.ReadTimeout)
			}
		})
	}
}

func TestWithStandaloneWriteTimeoutErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 15 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: -1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneWriteTimeout(tt.timeout)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.timeout, redisOpts.WriteTimeout)
			}
		})
	}
}

func TestWithStandalonePoolSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{
			name:    "valid size",
			size:    20,
			wantErr: false,
		},
		{
			name:    "zero size",
			size:    0,
			wantErr: true,
		},
		{
			name:    "negative size",
			size:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandalonePoolSize(tt.size)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.size, redisOpts.PoolSize)
			}
		})
	}
}

func TestWithStandaloneMinIdleConns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		count   int
		wantErr bool
	}{
		{
			name:    "valid count",
			count:   5,
			wantErr: false,
		},
		{
			name:    "zero count",
			count:   0,
			wantErr: false,
		},
		{
			name:    "negative count",
			count:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneMinIdleConns(tt.count)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.count, redisOpts.MinIdleConns)
			}
		})
	}
}

func TestWithStandalonePoolTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 8 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: -1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandalonePoolTimeout(tt.timeout)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.timeout, redisOpts.PoolTimeout)
			}
		})
	}
}

func TestWithStandaloneConnMaxIdleTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		wantErr  bool
	}{
		{
			name:     "valid duration",
			duration: 30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "zero duration",
			duration: 0,
			wantErr:  true,
		},
		{
			name:     "negative duration",
			duration: -1 * time.Minute,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneConnMaxIdleTime(tt.duration)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.duration, redisOpts.ConnMaxIdleTime)
			}
		})
	}
}

func TestWithStandaloneMaxRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		count   int
		wantErr bool
	}{
		{
			name:    "valid count",
			count:   5,
			wantErr: false,
		},
		{
			name:    "zero count",
			count:   0,
			wantErr: false,
		},
		{
			name:    "negative count",
			count:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opt := WithStandaloneMaxRetries(tt.count)
			redisOpts := &redis.Options{}
			err := opt(redisOpts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.count, redisOpts.MaxRetries)
			}
		})
	}
}

func TestWithStandaloneClientName(t *testing.T) {
	t.Parallel()

	clientName := "my-test-client"
	opt := WithStandaloneClientName(clientName)
	redisOpts := &redis.Options{}
	err := opt(redisOpts)
	assert.NoError(t, err)
	assert.Equal(t, clientName, redisOpts.ClientName)
}

func TestNewClient_WithInvalidOption(t *testing.T) {
	t.Parallel()

	config := RedisConfig{
		Addr: "localhost:6379",
		DB:   0,
	}

	// Test with invalid dial timeout
	_, err := NewStandaloneClient(config, WithStandaloneDialTimeout(-1*time.Second))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply option failed")

	// Test with invalid pool size
	_, err = NewStandaloneClient(config, WithStandalonePoolSize(-1))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply option failed")
}
