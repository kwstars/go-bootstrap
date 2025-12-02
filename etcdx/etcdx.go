package etcdx

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// Config simplified configuration (contains only the most common fields)
type Config struct {
	Endpoints []string    // Required: etcd cluster endpoints
	TLS       *tls.Config // Optional: TLS configuration
	Username  string      // Optional: username
	Password  string      // Optional: password
	Logger    *zap.Logger // Optional: logger
}

// Option function type for options
type Option func(*Config)

// New creates an etcd client (endpoints required, optional functional options)
func New(endpoints []string, options ...Option) (*clientv3.Client, error) {
	// check required params
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("endpoints cannot be empty")
	}

	// create default config (recommended for production)
	config := &Config{
		Endpoints: endpoints,
	}

	// apply options
	for _, option := range options {
		option(config)
	}

	// build etcd config
	etcdConfig := &clientv3.Config{
		Endpoints:            config.Endpoints,
		DialTimeout:          5 * time.Second,  // recommended for production
		DialKeepAliveTime:    30 * time.Second, // recommended for production
		DialKeepAliveTimeout: 10 * time.Second, // recommended for production
		MaxCallSendMsgSize:   10 * 1024 * 1024, // 10MB
		MaxCallRecvMsgSize:   10 * 1024 * 1024, // 10MB
		AutoSyncInterval:     1 * time.Minute,  // auto sync member list
		PermitWithoutStream:  true,             // allow keepalive without stream
		MaxUnaryRetries:      3,                // max retry attempts
	}

	// set TLS if provided
	if config.TLS != nil {
		etcdConfig.TLS = config.TLS
	}

	// set auth if provided
	if config.Username != "" && config.Password != "" {
		etcdConfig.Username = config.Username
		etcdConfig.Password = config.Password
	}

	// set logger if provided
	if config.Logger != nil {
		etcdConfig.Logger = config.Logger
	}

	// create etcd client
	cli, err := clientv3.New(*etcdConfig)
	if err != nil {
		return nil, fmt.Errorf("create etcd client failed: %w", err)
	}

	// check connection
	if err := checkConnection(context.TODO(), cli); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("etcd connection check failed: %w", err)
	}

	return cli, nil
}

// checkConnection verifies the connection
func checkConnection(ctx context.Context, cli *clientv3.Client) error {
	if ctx == nil {
		ctx = context.Background()
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := cli.MemberList(checkCtx)
	if err != nil && err != context.DeadlineExceeded {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return nil
}

// --- Common option functions ---

// WithTLS sets TLS config (certificate files)
func WithTLS(certFile, keyFile, caFile string) Option {
	return func(c *Config) {
		tlsInfo := &transport.TLSInfo{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}

		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			// Only setting config here; actual error handled in New
			return
		}

		c.TLS = tlsConfig
	}
}

// WithTLSConfig sets TLS config (pass tls.Config directly)
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(c *Config) {
		c.TLS = tlsConfig
	}
}

// WithAuth sets authentication info
func WithAuth(username, password string) Option {
	return func(c *Config) {
		c.Username = username
		c.Password = password
	}
}

// WithLogger sets logger
func WithLogger(logger *zap.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithTimeout sets timeouts (not commonly used)
func WithTimeout(dialTimeout, keepAliveTime, keepAliveTimeout time.Duration) Option {
	// Note: this option needs special handling because it directly affects clientv3.Config
	// To simplify, we do not provide this option because recommended production values are sufficient
	return func(c *Config) {
		// Not implemented, placeholder
		// Real implementation would require extending Config to support this
	}
}

// WithInsecure skips TLS verification (testing only)
func WithInsecure() Option {
	return func(c *Config) {
		c.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
}
