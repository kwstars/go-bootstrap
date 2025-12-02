package consulx

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_EmptyAddress test error on empty address
func TestNewClient_EmptyAddress(t *testing.T) {
	client, err := NewClient("")
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "address is required")
}

// TestNewClient_MinimalConfig test minimal configuration
func TestNewClient_MinimalConfig(t *testing.T) {
	client, err := NewClient("127.0.0.1:8500")
	require.NoError(t, err)
	require.NotNil(t, client)
}

// TestWithToken test Token option
func TestWithToken(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithToken("test-token")
	opt(cfg)
	assert.Equal(t, "test-token", cfg.token)
}

// TestWithTokenFile test TokenFile option
func TestWithTokenFile(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithTokenFile("/path/to/token")
	opt(cfg)
	assert.Equal(t, "/path/to/token", cfg.tokenFile)
}

// TestWithBasicAuth test basic auth option
func TestWithBasicAuth(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithBasicAuth("admin", "password")
	opt(cfg)
	require.NotNil(t, cfg.httpAuth)
	assert.Equal(t, "admin", cfg.httpAuth.Username)
	assert.Equal(t, "password", cfg.httpAuth.Password)
}

// TestWithDatacenter test datacenter option
func TestWithDatacenter(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithDatacenter("dc1")
	opt(cfg)
	assert.Equal(t, "dc1", cfg.datacenter)
}

// TestWithNamespace test namespace option
func TestWithNamespace(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithNamespace("production")
	opt(cfg)
	assert.Equal(t, "production", cfg.namespace)
}

// TestWithPartition test partition option
func TestWithPartition(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithPartition("team-a")
	opt(cfg)
	assert.Equal(t, "team-a", cfg.partition)
}

// TestWithTLS test TLS option
func TestWithTLS(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithTLS("ca.pem", "cert.pem", "key.pem")
	opt(cfg)
	assert.Equal(t, "https", cfg.scheme)
	require.NotNil(t, cfg.tlsConfig)
	assert.Equal(t, "ca.pem", cfg.tlsConfig.CAFile)
	assert.Equal(t, "cert.pem", cfg.tlsConfig.CertFile)
	assert.Equal(t, "key.pem", cfg.tlsConfig.KeyFile)
}

// TestWithTLSConfig test full TLS config
func TestWithTLSConfig(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	tlsConfig := api.TLSConfig{
		CAFile:             "ca.pem",
		CertFile:           "cert.pem",
		KeyFile:            "key.pem",
		InsecureSkipVerify: true,
	}
	opt := WithTLSConfig(tlsConfig)
	opt(cfg)
	assert.Equal(t, "https", cfg.scheme)
	require.NotNil(t, cfg.tlsConfig)
	assert.Equal(t, "ca.pem", cfg.tlsConfig.CAFile)
	assert.True(t, cfg.tlsConfig.InsecureSkipVerify)
}

// TestWithInsecureTLS test insecure TLS
func TestWithInsecureTLS(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithInsecureTLS()
	opt(cfg)
	assert.Equal(t, "https", cfg.scheme)
	require.NotNil(t, cfg.tlsConfig)
	assert.True(t, cfg.tlsConfig.InsecureSkipVerify)
}

// TestWithHTTPClient test custom HTTP client
func TestWithHTTPClient(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	opt := WithHTTPClient(httpClient)
	opt(cfg)
	assert.Equal(t, httpClient, cfg.httpClient)
}

// TestWithTransport test custom Transport
func TestWithTransport(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 50
	opt := WithTransport(transport)
	opt(cfg)
	assert.Equal(t, transport, cfg.transport)
	assert.Equal(t, 50, cfg.transport.MaxIdleConns)
}

// TestWithHeaders test set headers in batch
func TestWithHeaders(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	headers := http.Header{
		"X-Custom-1": []string{"value1"},
		"X-Custom-2": []string{"value2"},
	}
	opt := WithHeaders(headers)
	opt(cfg)
	assert.Equal(t, headers, cfg.headers)
}

// TestWithHeader test single header
func TestWithHeader(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithHeader("X-API-Key", "secret")
	opt(cfg)
	assert.Equal(t, "secret", cfg.headers.Get("X-API-Key"))
}

// TestWithTimeout test timeout configuration
func TestWithTimeout(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithTimeout(30 * time.Second)
	opt(cfg)
	require.NotNil(t, cfg.transport)
	assert.Equal(t, 30*time.Second, cfg.transport.ResponseHeaderTimeout)
	assert.Equal(t, 30*time.Second, cfg.transport.IdleConnTimeout)
}

// TestWithWaitTime test wait time configuration
func TestWithWaitTime(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithWaitTime(5 * time.Minute)
	opt(cfg)
	assert.Equal(t, 5*time.Minute, cfg.waitTime)
}

// TestWithConnectionPooling test connection pooling configuration
func TestWithConnectionPooling(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithConnectionPooling(100, 10)
	opt(cfg)
	require.NotNil(t, cfg.transport)
	assert.Equal(t, 100, cfg.transport.MaxIdleConns)
	assert.Equal(t, 10, cfg.transport.MaxIdleConnsPerHost)
	assert.False(t, cfg.transport.DisableKeepAlives)
}

// TestWithoutConnectionPooling test disable connection pooling
func TestWithoutConnectionPooling(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithoutConnectionPooling()
	opt(cfg)
	require.NotNil(t, cfg.transport)
	assert.True(t, cfg.transport.DisableKeepAlives)
}

// TestWithProductionDefaults test production defaults
func TestWithProductionDefaults(t *testing.T) {
	cfg := &clientConfig{headers: make(http.Header)}
	opt := WithProductionDefaults()
	opt(cfg)
	require.NotNil(t, cfg.transport)
	assert.Equal(t, 30*time.Second, cfg.transport.ResponseHeaderTimeout)
	assert.Equal(t, 10*time.Second, cfg.transport.TLSHandshakeTimeout)
	assert.Equal(t, 90*time.Second, cfg.transport.IdleConnTimeout)
	assert.Equal(t, 100, cfg.transport.MaxIdleConns)
	assert.Equal(t, 10, cfg.transport.MaxIdleConnsPerHost)
	assert.Equal(t, 100, cfg.transport.MaxConnsPerHost)
	assert.True(t, cfg.transport.ForceAttemptHTTP2)
	assert.Equal(t, 5*time.Minute, cfg.waitTime)
}

// TestNewClient_MultipleOptions test multiple options combined
func TestNewClient_MultipleOptions(t *testing.T) {
	client, err := NewClient(
		"127.0.0.1:8500",
		WithToken("test-token"),
		WithDatacenter("dc1"),
		WithNamespace("production"),
		WithHeader("X-Request-ID", "123"),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// verify headers
	headers := client.Headers()
	assert.Equal(t, "123", headers.Get("X-Request-ID"))
}

// TestNewClient_TokenFile test token file
func TestNewClient_TokenFile(t *testing.T) {
	// create temporary file
	tmpFile, err := os.CreateTemp("", "consul-token-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("file-token-123")
	require.NoError(t, err)
	tmpFile.Close()

	// create client
	client, err := NewClient(
		"127.0.0.1:8500",
		WithTokenFile(tmpFile.Name()),
	)
	require.NoError(t, err)
	require.NotNil(t, client)
}

// TestNewClient_SchemeDefaults test scheme defaults
func TestNewClient_SchemeDefaults(t *testing.T) {
	tests := []struct {
		name           string
		opts           []ClientOption
		expectedScheme string
	}{
		{
			name:           "default is http",
			opts:           []ClientOption{},
			expectedScheme: "http",
		},
		{
			name:           "with TLS is https",
			opts:           []ClientOption{WithTLS("ca.pem", "cert.pem", "key.pem")},
			expectedScheme: "https",
		},
		{
			name:           "with insecure TLS is https",
			opts:           []ClientOption{WithInsecureTLS()},
			expectedScheme: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &clientConfig{
				scheme:  "http",
				headers: make(http.Header),
			}
			for _, opt := range tt.opts {
				opt(cfg)
			}
			assert.Equal(t, tt.expectedScheme, cfg.scheme)
		})
	}
}

// BenchmarkNewClient benchmark
func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewClient("127.0.0.1:8500")
	}
}

// BenchmarkNewClient_WithOptions benchmark with options
func BenchmarkNewClient_WithOptions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewClient(
			"127.0.0.1:8500",
			WithToken("test"),
			WithDatacenter("dc1"),
			WithProductionDefaults(),
		)
	}
}
