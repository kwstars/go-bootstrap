package sonyflakex

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// MockRepo implements Repo interface for testing
type MockRepo struct {
	mu               sync.Mutex
	acquireFunc      func(ctx context.Context, ttl time.Duration) (int, error)
	renewFunc        func(ctx context.Context, machineID int, ttl time.Duration) error
	releaseFunc      func(ctx context.Context, machineID int) error
	acquireCallCount int
	renewCallCount   int
	releaseCallCount int
	machineIDPool    map[int]bool
}

func NewMockRepo() *MockRepo {
	return &MockRepo{
		machineIDPool: make(map[int]bool),
	}
}

func (m *MockRepo) AcquireMachineID(ctx context.Context, ttl time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acquireCallCount++

	if m.acquireFunc != nil {
		return m.acquireFunc(ctx, ttl)
	}

	// Default behavior: allocate next available ID
	for i := 0; i < 65536; i++ {
		if !m.machineIDPool[i] {
			m.machineIDPool[i] = true
			return i, nil
		}
	}
	return 0, errors.New("no available machine ID")
}

func (m *MockRepo) RenewMachineID(ctx context.Context, machineID int, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renewCallCount++

	if m.renewFunc != nil {
		return m.renewFunc(ctx, machineID, ttl)
	}
	return nil
}

func (m *MockRepo) ReleaseMachineID(ctx context.Context, machineID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseCallCount++

	if m.releaseFunc != nil {
		return m.releaseFunc(ctx, machineID)
	}

	delete(m.machineIDPool, machineID)
	return nil
}

func (m *MockRepo) GetCallCounts() (acquire, renew, release int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acquireCallCount, m.renewCallCount, m.releaseCallCount
}

// TestNew tests generator creation with valid configuration
func TestNew(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	if g.machineID < 0 {
		t.Errorf("machineID = %d, want >= 0", g.machineID)
	}

	acquire, _, _ := repo.GetCallCounts()
	if acquire != 1 {
		t.Errorf("AcquireMachineID called %d times, want 1", acquire)
	}
}

// TestNew_NilRepo tests that nil repo returns error
func TestNew_NilRepo(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("New(nil) should return error")
	}
	if err.Error() != "sonyflakex repo is required" {
		t.Errorf("error = %v, want 'sonyflakex repo is required'", err)
	}
}

// TestNew_AcquireMachineIDFailure tests acquire failure handling
func TestNew_AcquireMachineIDFailure(t *testing.T) {
	repo := NewMockRepo()
	repo.acquireFunc = func(ctx context.Context, ttl time.Duration) (int, error) {
		return 0, errors.New("network error")
	}

	_, err := New(repo)
	if err == nil {
		t.Fatal("New() should fail when AcquireMachineID fails")
	}
	if !errors.Is(err, ErrAcquireMachineID) {
		t.Errorf("error = %v, want ErrAcquireMachineID", err)
	}
}

// TestNew_InvalidMachineID tests machine ID out of bit range
func TestNew_InvalidMachineID(t *testing.T) {
	repo := NewMockRepo()
	repo.acquireFunc = func(ctx context.Context, ttl time.Duration) (int, error) {
		return 65536, nil // Exceeds 16-bit range
	}

	_, err := New(repo, WithStartTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err == nil {
		t.Fatal("New() should fail with invalid machine ID")
	}

	_, _, release := repo.GetCallCounts()
	if release != 1 {
		t.Errorf("ReleaseMachineID called %d times, want 1", release)
	}
}

// TestWithStartTime tests start time option
func TestWithStartTime(t *testing.T) {
	repo := NewMockRepo()
	startTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	g, err := New(repo, WithStartTime(startTime))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}

	genTime := g.ToTime(id)
	if genTime.Before(startTime) {
		t.Errorf("ToTime() = %v, want >= %v", genTime, startTime)
	}
}

// TestWithStartTime_Future tests validation of future start time
func TestWithStartTime_Future(t *testing.T) {
	repo := NewMockRepo()
	futureTime := time.Now().Add(24 * time.Hour)

	_, err := New(repo, WithStartTime(futureTime))
	if !errors.Is(err, ErrInvalidStartTime) {
		t.Errorf("error = %v, want ErrInvalidStartTime", err)
	}
}

// TestWithTimeUnit tests time unit option
func TestWithTimeUnit(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithTimeUnit(1*time.Millisecond))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("NextID() = %d, want > 0", id)
	}
}

// TestWithTimeUnit_Invalid tests validation of invalid time unit
func TestWithTimeUnit_Invalid(t *testing.T) {
	repo := NewMockRepo()

	_, err := New(repo, WithTimeUnit(500*time.Microsecond))
	if !errors.Is(err, ErrInvalidTimeUnit) {
		t.Errorf("error = %v, want ErrInvalidTimeUnit", err)
	}
}

// TestWithTTL tests TTL option
func TestWithTTL(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithTTL(60*time.Second),
		WithRenewFrequency(20*time.Second))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	if g.ttl != 60*time.Second {
		t.Errorf("ttl = %v, want 60s", g.ttl)
	}
}

// TestWithRenewFrequency tests renew frequency option
func TestWithRenewFrequency(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithRenewFrequency(5*time.Second))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	if g.renewFreq != 5*time.Second {
		t.Errorf("renewFreq = %v, want 5s", g.renewFreq)
	}
}

// TestWithRenewFrequency_ExceedsTTL tests validation when renewFreq >= TTL
func TestWithRenewFrequency_ExceedsTTL(t *testing.T) {
	repo := NewMockRepo()

	_, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithTTL(10*time.Second),
		WithRenewFrequency(10*time.Second))
	if err == nil {
		t.Fatal("New() should fail when renewFreq >= TTL")
	}
}

// TestNextID tests ID generation
func TestNextID(t *testing.T) {
	repo := NewMockRepo()
	g, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	ids := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id, err := g.NextID()
		if err != nil {
			t.Fatalf("NextID() failed: %v", err)
		}
		if ids[id] {
			t.Fatalf("duplicate ID: %d", id)
		}
		ids[id] = true
	}
}

// TestNextID_Uniqueness tests ID uniqueness across multiple generators
func TestNextID_Uniqueness(t *testing.T) {
	repo := NewMockRepo()

	g1, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() g1 failed: %v", err)
	}
	defer g1.Stop(context.Background())

	g2, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() g2 failed: %v", err)
	}
	defer g2.Stop(context.Background())

	ids := make(map[int64]bool)
	for i := 0; i < 500; i++ {
		id1, _ := g1.NextID()
		id2, _ := g2.NextID()

		if ids[id1] {
			t.Fatalf("duplicate ID from g1: %d", id1)
		}
		if ids[id2] {
			t.Fatalf("duplicate ID from g2: %d", id2)
		}
		ids[id1] = true
		ids[id2] = true
	}
}

// TestToTime tests time extraction from ID
func TestToTime(t *testing.T) {
	repo := NewMockRepo()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	g, err := New(repo, WithStartTime(startTime))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	before := time.Now()
	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	after := time.Now()

	genTime := g.ToTime(id)
	// sonyflake uses a time unit (default 10ms) which can round generation time.
	// Allow a small tolerance to account for rounding and scheduling jitter.
	const tolerance = 20 * time.Millisecond
	if genTime.Before(before.Add(-tolerance)) || genTime.After(after.Add(tolerance)) {
		t.Errorf("ToTime() = %v, want between %v and %v (tolerance %v)", genTime, before.Add(-tolerance), after.Add(tolerance), tolerance)
	}
}

// TestDecompose tests ID decomposition
func TestDecompose(t *testing.T) {
	repo := NewMockRepo()
	g, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}

	parts := g.Decompose(id)
	if parts["id"] != id {
		t.Errorf("parts[id] = %d, want %d", parts["id"], id)
	}
	if parts["machine-id"] != int64(g.machineID) {
		t.Errorf("parts[machine-id] = %d, want %d", parts["machine-id"], g.machineID)
	}
	if parts["time"] < 0 {
		t.Errorf("parts[time] = %d, want >= 0", parts["time"])
	}
	if parts["sequence"] < 0 {
		t.Errorf("parts[sequence] = %d, want >= 0", parts["sequence"])
	}
}

// TestStop tests graceful shutdown
func TestStop(t *testing.T) {
	repo := NewMockRepo()
	g, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	err = g.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	_, _, release := repo.GetCallCounts()
	if release != 1 {
		t.Errorf("ReleaseMachineID called %d times, want 1", release)
	}
}

// TestStop_ReleaseFailure tests error handling when release fails
func TestStop_ReleaseFailure(t *testing.T) {
	repo := NewMockRepo()
	repo.releaseFunc = func(ctx context.Context, machineID int) error {
		return errors.New("connection refused")
	}

	g, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	err = g.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop() should return error when release fails")
	}
	if !errors.Is(err, ErrReleaseMachineID) {
		t.Errorf("error = %v, want ErrReleaseMachineID", err)
	}
}

// TestHeartbeat tests periodic renewal of machine ID
func TestHeartbeat(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithTTL(1*time.Second),
		WithRenewFrequency(100*time.Millisecond))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	time.Sleep(350 * time.Millisecond)

	_, renew, _ := repo.GetCallCounts()
	if renew < 2 {
		t.Errorf("RenewMachineID called %d times, want >= 2", renew)
	}
}

// TestHeartbeat_StopsAfterStop tests heartbeat stops after Stop()
func TestHeartbeat_StopsAfterStop(t *testing.T) {
	repo := NewMockRepo()

	g, err := New(repo,
		WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		WithTTL(1*time.Second),
		WithRenewFrequency(50*time.Millisecond))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	g.Stop(context.Background())

	_, renewBefore, _ := repo.GetCallCounts()
	time.Sleep(150 * time.Millisecond)
	_, renewAfter, _ := repo.GetCallCounts()

	if renewAfter > renewBefore {
		t.Errorf("heartbeat continued after Stop(), renewals: %d -> %d", renewBefore, renewAfter)
	}
}

// TestConcurrentGeneration tests concurrent ID generation
func TestConcurrentGeneration(t *testing.T) {
	repo := NewMockRepo()
	g, err := New(repo, WithStartTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer g.Stop(context.Background())

	var wg sync.WaitGroup
	idChan := make(chan int64, 1000)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				id, err := g.NextID()
				if err != nil {
					t.Errorf("NextID() failed: %v", err)
					return
				}
				idChan <- id
			}
		}()
	}

	wg.Wait()
	close(idChan)

	ids := make(map[int64]bool)
	for id := range idChan {
		if ids[id] {
			t.Fatalf("duplicate ID in concurrent generation: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != 1000 {
		t.Errorf("generated %d unique IDs, want 1000", len(ids))
	}
}

// TestValidateConfig tests configuration validation
func TestValidateConfig(t *testing.T) {
	// Only validate that the default config passes validation.
	if err := validateConfig(defaultGeneratorConfig()); err != nil {
		t.Fatalf("defaultGeneratorConfig failed validation: %v", err)
	}
}
