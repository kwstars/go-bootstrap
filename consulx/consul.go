package consulx

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
)

// ClientOption defines client configuration options
type ClientOption func(*clientConfig)

// clientConfig internal configuration structure
type clientConfig struct {
	// Basic configuration
	token      string
	tokenFile  string
	datacenter string
	namespace  string
	partition  string

	// HTTP configuration
	httpAuth   *api.HttpBasicAuth
	headers    http.Header
	transport  *http.Transport
	httpClient *http.Client

	// TLS configuration
	tlsConfig *api.TLSConfig

	// Timeout configuration
	waitTime time.Duration

	// Other configuration
	scheme string
}

// NewClient creates a Consul client
// address is a required parameter, format like "127.0.0.1:8500" or "https://consul.example.com"
func NewClient(address string, opts ...ClientOption) (*api.Client, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Initialize default configuration
	cfg := &clientConfig{
		scheme:   "http",
		waitTime: 0,
		headers:  make(http.Header),
	}

	// Apply all options
	for _, opt := range opts {
		opt(cfg)
	}

	// Build Consul API Config
	config := api.DefaultConfig()
	config.Address = address
	config.Scheme = cfg.scheme

	// Set authentication
	if cfg.token != "" {
		config.Token = cfg.token
	}
	if cfg.tokenFile != "" {
		config.TokenFile = cfg.tokenFile
	}
	if cfg.httpAuth != nil {
		config.HttpAuth = cfg.httpAuth
	}

	// Set datacenter and namespace
	if cfg.datacenter != "" {
		config.Datacenter = cfg.datacenter
	}
	if cfg.namespace != "" {
		config.Namespace = cfg.namespace
	}
	if cfg.partition != "" {
		config.Partition = cfg.partition
	}

	// Set TLS
	if cfg.tlsConfig != nil {
		config.TLSConfig = *cfg.tlsConfig
	}

	// Set wait time
	if cfg.waitTime > 0 {
		config.WaitTime = cfg.waitTime
	}

	// Set Transport
	if cfg.transport != nil {
		config.Transport = cfg.transport
	}

	// Set HTTP client
	if cfg.httpClient != nil {
		config.HttpClient = cfg.httpClient
	}

	// Create client
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	// Set custom Headers
	if len(cfg.headers) > 0 {
		client.SetHeaders(cfg.headers)
	}

	return client, nil
}

// ==================== Authentication related options ====================

// WithToken sets the access token
func WithToken(token string) ClientOption {
	return func(c *clientConfig) {
		c.token = token
	}
}

// WithTokenFile sets the token file path
func WithTokenFile(tokenFile string) ClientOption {
	return func(c *clientConfig) {
		c.tokenFile = tokenFile
	}
}

// WithBasicAuth sets HTTP basic authentication
func WithBasicAuth(username, password string) ClientOption {
	return func(c *clientConfig) {
		c.httpAuth = &api.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
}

// ==================== Location related options ====================

// WithDatacenter sets the datacenter
func WithDatacenter(datacenter string) ClientOption {
	return func(c *clientConfig) {
		c.datacenter = datacenter
	}
}

// WithNamespace sets the namespace (Enterprise feature)
func WithNamespace(namespace string) ClientOption {
	return func(c *clientConfig) {
		c.namespace = namespace
	}
}

// WithPartition sets the partition (Enterprise feature)
func WithPartition(partition string) ClientOption {
	return func(c *clientConfig) {
		c.partition = partition
	}
}

// ==================== TLS related options ====================

// WithTLS sets TLS configuration
func WithTLS(caFile, certFile, keyFile string) ClientOption {
	return func(c *clientConfig) {
		c.scheme = "https"
		c.tlsConfig = &api.TLSConfig{
			CAFile:   caFile,
			CertFile: certFile,
			KeyFile:  keyFile,
		}
	}
}

// WithTLSConfig sets complete TLS configuration
func WithTLSConfig(tlsConfig api.TLSConfig) ClientOption {
	return func(c *clientConfig) {
		if tlsConfig.CAFile != "" || tlsConfig.CertFile != "" ||
			tlsConfig.InsecureSkipVerify {
			c.scheme = "https"
		}
		c.tlsConfig = &tlsConfig
	}
}

// WithInsecureTLS sets to skip TLS verification (for testing only)
func WithInsecureTLS() ClientOption {
	return func(c *clientConfig) {
		c.scheme = "https"
		if c.tlsConfig == nil {
			c.tlsConfig = &api.TLSConfig{}
		}
		c.tlsConfig.InsecureSkipVerify = true
	}
}

// ==================== HTTP related options ====================

// WithHTTPClient sets custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithTransport sets custom Transport
func WithTransport(transport *http.Transport) ClientOption {
	return func(c *clientConfig) {
		c.transport = transport
	}
}

// WithHeaders sets custom HTTP headers
func WithHeaders(headers http.Header) ClientOption {
	return func(c *clientConfig) {
		c.headers = headers
	}
}

// WithHeader adds a single HTTP header
func WithHeader(key, value string) ClientOption {
	return func(c *clientConfig) {
		c.headers.Set(key, value)
	}
}

// ==================== Timeout related options ====================

// WithTimeout sets connection and request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) {
		if c.transport == nil {
			c.transport = http.DefaultTransport.(*http.Transport).Clone()
		}
		c.transport.ResponseHeaderTimeout = timeout
		c.transport.IdleConnTimeout = timeout
	}
}

// WithWaitTime sets the maximum wait time for blocking queries
func WithWaitTime(waitTime time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.waitTime = waitTime
	}
}

// ==================== Connection pooling related options ====================

// WithConnectionPooling sets connection pool configuration
func WithConnectionPooling(maxIdleConns, maxIdleConnsPerHost int) ClientOption {
	return func(c *clientConfig) {
		if c.transport == nil {
			c.transport = http.DefaultTransport.(*http.Transport).Clone()
		}
		c.transport.MaxIdleConns = maxIdleConns
		c.transport.MaxIdleConnsPerHost = maxIdleConnsPerHost
		c.transport.DisableKeepAlives = false
	}
}

// WithoutConnectionPooling disables connection pooling
func WithoutConnectionPooling() ClientOption {
	return func(c *clientConfig) {
		if c.transport == nil {
			c.transport = http.DefaultTransport.(*http.Transport).Clone()
		}
		c.transport.DisableKeepAlives = true
	}
}

// ==================== Production environment preset configuration ====================

// WithProductionDefaults applies production environment recommended configuration
func WithProductionDefaults() ClientOption {
	return func(c *clientConfig) {
		// Set reasonable timeouts
		if c.transport == nil {
			c.transport = http.DefaultTransport.(*http.Transport).Clone()
		}

		// Connection timeout: 10 seconds
		c.transport.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext

		// Response header timeout: 30 seconds
		c.transport.ResponseHeaderTimeout = 30 * time.Second

		// TLS handshake timeout: 10 seconds
		c.transport.TLSHandshakeTimeout = 10 * time.Second

		// Idle connection timeout: 90 seconds
		c.transport.IdleConnTimeout = 90 * time.Second

		// Connection pool configuration
		c.transport.MaxIdleConns = 100
		c.transport.MaxIdleConnsPerHost = 10
		c.transport.MaxConnsPerHost = 100

		// Enable HTTP/2
		c.transport.ForceAttemptHTTP2 = true

		// Blocking query wait time: 5 minutes
		c.waitTime = 5 * time.Minute
	}
}
