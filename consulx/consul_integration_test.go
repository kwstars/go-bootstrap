//go:build integration

package consulx

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lumina/pkg/consulx" // replace with your actual module path
)

// These tests require a real Consul server
// Run with: go test -tags=integration

// TestIntegration_MinimalConfig test minimal connection
func TestIntegration_MinimalConfig(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)
	require.NotNil(t, client)

	// verify connection
	_, err = client.Agent().Self()
	assert.NoError(t, err)
}

// TestIntegration_WithToken test connection with Token
func TestIntegration_WithToken(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
		c.ACL.DefaultPolicy = "deny"
		c.ACL.Tokens.InitialManagement = "root-token"
		c.ACL.Tokens.Agent = "root-token"
	})
	require.NoError(t, err)
	defer server.Stop()
	server.WaitForLeader(t)

	// with correct Token
	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithToken("root-token"),
	)
	require.NoError(t, err)

	_, err = client.Agent().Self()
	assert.NoError(t, err)
}

// TestIntegration_WithoutToken_Denied test denied when no Token provided
func TestIntegration_WithoutToken_Denied(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
		c.ACL.DefaultPolicy = "deny"
		c.ACL.Tokens.InitialManagement = "root-token"
	})
	require.NoError(t, err)
	defer server.Stop()
	server.WaitForLeader(t)

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	// should be denied
	_, err = client.Agent().Self()
	assert.Error(t, err)
}

// TestIntegration_KVOperations test KV store operations
func TestIntegration_KVOperations(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	kv := client.KV()

	// Put
	pair := &api.KVPair{
		Key:   "test/key",
		Value: []byte("test-value"),
	}
	_, err = kv.Put(pair, nil)
	require.NoError(t, err)

	// Get
	result, _, err := kv.Get("test/key", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test/key", result.Key)
	assert.Equal(t, []byte("test-value"), result.Value)

	// Update
	pair.Value = []byte("updated-value")
	_, err = kv.Put(pair, nil)
	require.NoError(t, err)

	result, _, err = kv.Get("test/key", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("updated-value"), result.Value)

	// Delete
	_, err = kv.Delete("test/key", nil)
	require.NoError(t, err)

	result, _, err = kv.Get("test/key", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// TestIntegration_ServiceRegistration test service registration
func TestIntegration_ServiceRegistration(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	agent := client.Agent()

	// register service
	registration := &api.AgentServiceRegistration{
		ID:      "test-service-1",
		Name:    "test-service",
		Port:    8080,
		Address: "127.0.0.1",
		Tags:    []string{"v1", "test"},
		Meta:    map[string]string{"version": "1.0"},
		Check: &api.AgentServiceCheck{
			HTTP:     "http://127.0.0.1:8080/health",
			Interval: "10s",
			Timeout:  "1s",
		},
	}
	err = agent.ServiceRegister(registration)
	require.NoError(t, err)

	// query services
	services, err := agent.Services()
	require.NoError(t, err)
	assert.Contains(t, services, "test-service-1")

	service := services["test-service-1"]
	assert.Equal(t, "test-service", service.Service)
	assert.Equal(t, 8080, service.Port)
	assert.Equal(t, "127.0.0.1", service.Address)
	assert.ElementsMatch(t, []string{"v1", "test"}, service.Tags)

	// deregister service
	err = agent.ServiceDeregister("test-service-1")
	require.NoError(t, err)

	// verify deregistration
	services, err = agent.Services()
	require.NoError(t, err)
	assert.NotContains(t, services, "test-service-1")
}

// TestIntegration_MultipleServices test multiple service registrations
func TestIntegration_MultipleServices(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	agent := client.Agent()

	// register multiple services
	for i := 1; i <= 3; i++ {
		registration := &api.AgentServiceRegistration{
			ID:      fmt.Sprintf("service-%d", i),
			Name:    "test-service",
			Port:    8080 + i,
			Address: "127.0.0.1",
		}
		err = agent.ServiceRegister(registration)
		require.NoError(t, err)
	}

	// verify all services registered
	services, err := agent.Services()
	require.NoError(t, err)
	assert.Contains(t, services, "service-1")
	assert.Contains(t, services, "service-2")
	assert.Contains(t, services, "service-3")

	// cleanup
	for i := 1; i <= 3; i++ {
		err = agent.ServiceDeregister(fmt.Sprintf("service-%d", i))
		require.NoError(t, err)
	}
}

// TestIntegration_HealthChecks test health checks
func TestIntegration_HealthChecks(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	agent := client.Agent()

	// register service with health check
	registration := &api.AgentServiceRegistration{
		ID:      "web-service",
		Name:    "web",
		Port:    80,
		Address: "127.0.0.1",
		Check: &api.AgentServiceCheck{
			TTL: "10s",
		},
	}
	err = agent.ServiceRegister(registration)
	require.NoError(t, err)
	defer agent.ServiceDeregister("web-service")

	// query health checks
	health := client.Health()
	checks, _, err := health.Checks("web", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, checks)
}

// TestIntegration_CatalogServices test catalog services
func TestIntegration_CatalogServices(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(t, err)

	catalog := client.Catalog()

	// query all services
	services, _, err := catalog.Services(nil)
	require.NoError(t, err)
	assert.NotNil(t, services)
	assert.Contains(t, services, "consul")
}

// TestIntegration_Datacenter test datacenter configuration
func TestIntegration_Datacenter(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, func(c *testutil.TestServerConfig) {
		c.Datacenter = "dc1"
	})
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithDatacenter("dc1"),
	)
	require.NoError(t, err)

	// verify datacenter
	self, err := client.Agent().Self()
	require.NoError(t, err)
	assert.Equal(t, "dc1", self["Config"]["Datacenter"])
}

// TestIntegration_CustomHeaders test custom headers
func TestIntegration_CustomHeaders(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithHeader("X-Request-ID", "test-123"),
		consulx.WithHeader("X-Custom-Header", "custom-value"),
	)
	require.NoError(t, err)

	// verify headers are set
	headers := client.Headers()
	assert.Equal(t, "test-123", headers.Get("X-Request-ID"))
	assert.Equal(t, "custom-value", headers.Get("X-Custom-Header"))

	// verify communication works
	_, err = client.Agent().Self()
	assert.NoError(t, err)
}

// TestIntegration_ProductionConfig test production configuration
func TestIntegration_ProductionConfig(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithProductionDefaults(),
	)
	require.NoError(t, err)

	// verify it works
	_, err = client.Agent().Self()
	assert.NoError(t, err)

	// perform some real operations
	kv := client.KV()
	_, err = kv.Put(&api.KVPair{
		Key:   "prod/config",
		Value: []byte("production"),
	}, nil)
	assert.NoError(t, err)

	result, _, err := kv.Get("prod/config", nil)
	assert.NoError(t, err)
	assert.Equal(t, []byte("production"), result.Value)
}

// TestIntegration_WaitTime test blocking queries
func TestIntegration_WaitTime(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithWaitTime(5*time.Second),
	)
	require.NoError(t, err)

	kv := client.KV()

	// write initial value
	_, err = kv.Put(&api.KVPair{
		Key:   "watch/key",
		Value: []byte("initial"),
	}, nil)
	require.NoError(t, err)

	// get current index
	result, meta, err := kv.Get("watch/key", nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// use blocking query (modify value in background)
	go func() {
		time.Sleep(1 * time.Second)
		kv.Put(&api.KVPair{
			Key:   "watch/key",
			Value: []byte("updated"),
		}, nil)
	}()

	// blocking query should return after the value is updated
	start := time.Now()
	result, meta2, err := kv.Get("watch/key", &api.QueryOptions{
		WaitIndex: meta.LastIndex,
		WaitTime:  5 * time.Second,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Greater(t, meta2.LastIndex, meta.LastIndex)
	assert.Less(t, elapsed, 3*time.Second) // should return within ~1-2 seconds
	assert.Equal(t, []byte("updated"), result.Value)
}

// TestIntegration_ConcurrentOperations test concurrent operations
func TestIntegration_ConcurrentOperations(t *testing.T) {
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	defer server.Stop()

	client, err := consulx.NewClient(
		server.HTTPAddr,
		consulx.WithConnectionPooling(50, 10),
	)
	require.NoError(t, err)

	kv := client.KV()

	// concurrent writes
	const numGoroutines = 20
	const numOperations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("concurrent/%d/%d", id, j)
				_, err := kv.Put(&api.KVPair{
					Key:   key,
					Value: []byte(fmt.Sprintf("value-%d-%d", id, j)),
				}, nil)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// verify all data written successfully
	pairs, _, err := kv.List("concurrent/", nil)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*numOperations, len(pairs))
}

// BenchmarkIntegration_KVPut benchmark KV put
func BenchmarkIntegration_KVPut(b *testing.B) {
	server, err := testutil.NewTestServerConfigT(b, nil)
	require.NoError(b, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(b, err)

	kv := client.KV()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := kv.Put(&api.KVPair{
			Key:   fmt.Sprintf("bench/key-%d", i),
			Value: []byte("benchmark-value"),
		}, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIntegration_KVGet benchmark KV get
func BenchmarkIntegration_KVGet(b *testing.B) {
	server, err := testutil.NewTestServerConfigT(b, nil)
	require.NoError(b, err)
	defer server.Stop()

	client, err := consulx.NewClient(server.HTTPAddr)
	require.NoError(b, err)

	kv := client.KV()

	// pre-insert data
	_, err = kv.Put(&api.KVPair{
		Key:   "bench/read-key",
		Value: []byte("benchmark-value"),
	}, nil)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := kv.Get("bench/read-key", nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
