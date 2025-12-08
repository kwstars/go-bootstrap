package goredisx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// Package goredisx provides helpers to create and manage Redis clients
// using the go-redis v9 library, with a set of functional options for
// configuring standalone Redis instances.

// RedisConfig holds parameters for connecting to a standalone Redis server.
type RedisConfig struct {
	Addr     string
	DB       int
	Username string
	Password string
}

// Validate checks that the RedisConfig contains valid, required values.
func (c *RedisConfig) Validate() error {
	switch {
	case c.Addr == "":
		return errors.New("addr is required")
	case c.DB < 0:
		return errors.New("db must be non-negative")
	}
	return nil
}

// StandaloneOption is a functional option used to configure redis.Options
// when creating a standalone Redis client.
type StandaloneOption func(*redis.Options) error

// WithStandaloneAddr returns a StandaloneOption that sets the Redis server address.
func WithStandaloneAddr(addr string) StandaloneOption {
	return func(o *redis.Options) error {
		if addr == "" {
			return errors.New("addr cannot be empty")
		}
		o.Addr = addr
		return nil
	}
}

// WithStandaloneDB returns a StandaloneOption that sets the Redis database number.
func WithStandaloneDB(db int) StandaloneOption {
	return func(o *redis.Options) error {
		if db < 0 {
			return errors.New("db must be non-negative")
		}
		o.DB = db
		return nil
	}
}

// WithStandaloneUsername returns a StandaloneOption that sets the Redis username.
func WithStandaloneUsername(username string) StandaloneOption {
	return func(o *redis.Options) error {
		o.Username = username
		return nil
	}
}

// WithPassword returns a StandaloneOption that sets the Redis password.
func WithPassword(password string) StandaloneOption {
	return func(o *redis.Options) error {
		o.Password = password
		return nil
	}
}

// WithStandaloneDialTimeout returns a StandaloneOption that sets the dial timeout.
func WithStandaloneDialTimeout(timeout time.Duration) StandaloneOption {
	return func(o *redis.Options) error {
		if timeout <= 0 {
			return errors.New("dial timeout must be positive")
		}
		o.DialTimeout = timeout
		return nil
	}
}

// WithStandaloneReadTimeout returns a StandaloneOption that sets the read timeout.
func WithStandaloneReadTimeout(timeout time.Duration) StandaloneOption {
	return func(o *redis.Options) error {
		if timeout <= 0 {
			return errors.New("read timeout must be positive")
		}
		o.ReadTimeout = timeout
		return nil
	}
}

// WithStandaloneWriteTimeout returns a StandaloneOption that sets the write timeout.
func WithStandaloneWriteTimeout(timeout time.Duration) StandaloneOption {
	return func(o *redis.Options) error {
		if timeout <= 0 {
			return errors.New("write timeout must be positive")
		}
		o.WriteTimeout = timeout
		return nil
	}
}

// WithStandalonePoolSize returns a StandaloneOption that sets the connection pool size.
func WithStandalonePoolSize(size int) StandaloneOption {
	return func(o *redis.Options) error {
		if size <= 0 {
			return errors.New("pool size must be positive")
		}
		o.PoolSize = size
		return nil
	}
}

// WithStandaloneMinIdleConns returns a StandaloneOption that sets the minimum number of idle connections.
func WithStandaloneMinIdleConns(count int) StandaloneOption {
	return func(o *redis.Options) error {
		if count < 0 {
			return errors.New("min idle conns cannot be negative")
		}
		o.MinIdleConns = count
		return nil
	}
}

// WithStandalonePoolTimeout returns a StandaloneOption that sets the pool timeout.
func WithStandalonePoolTimeout(timeout time.Duration) StandaloneOption {
	return func(o *redis.Options) error {
		if timeout <= 0 {
			return errors.New("pool timeout must be positive")
		}
		o.PoolTimeout = timeout
		return nil
	}
}

// WithStandaloneConnMaxIdleTime returns a StandaloneOption that sets the maximum idle time for connections.
func WithStandaloneConnMaxIdleTime(duration time.Duration) StandaloneOption {
	return func(o *redis.Options) error {
		if duration <= 0 {
			return errors.New("conn max idle time must be positive")
		}
		o.ConnMaxIdleTime = duration
		return nil
	}
}

// WithStandaloneMaxRetries returns a StandaloneOption that sets the maximum number of retries for commands.
func WithStandaloneMaxRetries(count int) StandaloneOption {
	return func(o *redis.Options) error {
		if count < 0 {
			return errors.New("max retries cannot be negative")
		}
		o.MaxRetries = count
		return nil
	}
}

// WithStandaloneTLSConfig returns a StandaloneOption that configures TLS for the client connection.
func WithStandaloneTLSConfig(config *tls.Config) StandaloneOption {
	return func(o *redis.Options) error {
		o.TLSConfig = config
		return nil
	}
}

// WithStandaloneClientName returns a StandaloneOption that sets the client name reported to Redis.
func WithStandaloneClientName(name string) StandaloneOption {
	return func(o *redis.Options) error {
		o.ClientName = name
		return nil
	}
}

// NewStandaloneClient creates and returns a configured redis.UniversalClient for a standalone Redis instance.
// It validates cfg, applies provided StandaloneOption values, constructs the client, and verifies
// connectivity by performing a Ping using the configured DialTimeout.
func NewStandaloneClient(cfg RedisConfig, opts ...StandaloneOption) (redis.UniversalClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create instance and explicitly set default values.
	options := &redis.Options{
		Addr:     cfg.Addr,
		DB:       cfg.DB,
		Username: cfg.Username,
		Password: cfg.Password,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled, // Disable maintenance notifications
		},
	}

	// Apply all options.
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, fmt.Errorf("apply option failed: %w", err)
		}
	}

	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}

// HealthCheck pings the provided Redis client and returns any error encountered.
// It is a convenience wrapper for readiness/liveness checks.
func HealthCheck(ctx context.Context, client redis.UniversalClient) error {
	return client.Ping(ctx).Err()
}

// Close closes the provided Redis client, returning any error encountered.
func Close(client redis.UniversalClient) error {
	return client.Close()
}
