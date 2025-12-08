package sonyflakex

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sony/sonyflake/v2"
)

var (
	ErrInvalidStartTime = errors.New("start time must be in the past")
	ErrInvalidTimeUnit  = errors.New("time unit must be at least 1ms")
	ErrAcquireMachineID = errors.New("failed to acquire machine ID")
	ErrReleaseMachineID = errors.New("failed to release machine ID")
	ErrInvalidBitLength = errors.New("invalid bit length configuration")
)

const (
	defaultBitsMachine  = 16
	defaultBitsSequence = 8
)

// Repo defines the interface for managing distributed machine IDs
type Repo interface {
	AcquireMachineID(ctx context.Context, ttl time.Duration) (int, error)
	RenewMachineID(ctx context.Context, machineID int, ttl time.Duration) error
	ReleaseMachineID(ctx context.Context, machineID int) error
}

// Generator wraps sonyflake.Sonyflake with distributed machine ID management
type Generator struct {
	sf        *sonyflake.Sonyflake
	repo      Repo
	machineID int
	stopChan  chan struct{}
	doneChan  chan struct{}
	stopOnce  sync.Once
	stopErr   error
	ttl       time.Duration
	renewFreq time.Duration
}

// Option defines optional configuration for Generator
type Option func(*generatorConfig) error

type generatorConfig struct {
	settings  sonyflake.Settings
	ttl       time.Duration
	renewFreq time.Duration
}

// Default production settings based on best practices:
// - StartTime: 2025-01-01 (recent epoch reduces time bit usage)
// - TimeUnit: 10ms (balance between precision and lifespan)
// - TTL: 30s (machine ID lease duration)
// - RenewFreq: 10s (renew every 1/3 of TTL for safety)
func defaultGeneratorConfig() *generatorConfig {
	return &generatorConfig{
		settings: sonyflake.Settings{
			StartTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			TimeUnit:  10 * time.Millisecond,
		},
		ttl:       30 * time.Second,
		renewFreq: 10 * time.Second,
	}
}

// WithStartTime sets the epoch start time
// Must be in the past to avoid time overflow
func WithStartTime(t time.Time) Option {
	return func(c *generatorConfig) error {
		if t.After(time.Now()) {
			return ErrInvalidStartTime
		}
		c.settings.StartTime = t
		return nil
	}
}

// WithTimeUnit sets the time unit precision
// Smaller units provide more precision but reduce lifespan
// Recommended: 10ms (default) for most cases
func WithTimeUnit(d time.Duration) Option {
	return func(c *generatorConfig) error {
		if d < time.Millisecond {
			return ErrInvalidTimeUnit
		}
		c.settings.TimeUnit = d
		return nil
	}
}

// WithTTL sets the machine ID lease duration
// Should be long enough to survive temporary network issues
func WithTTL(d time.Duration) Option {
	return func(c *generatorConfig) error {
		if d <= 0 {
			return errors.New("ttl must be positive")
		}
		c.ttl = d
		return nil
	}
}

// WithRenewFrequency sets how often to renew the machine ID lease
// Should be significantly less than TTL (recommended: TTL/3)
func WithRenewFrequency(d time.Duration) Option {
	return func(c *generatorConfig) error {
		if d <= 0 {
			return errors.New("renew frequency must be positive")
		}
		c.renewFreq = d
		return nil
	}
}

// New creates a new Generator with distributed machine ID management
// repo: required - manages machine ID allocation and uniqueness
// opts: optional - configuration overrides
func New(repo Repo, opts ...Option) (*Generator, error) {
	if repo == nil {
		return nil, errors.New("sonyflakex repo is required")
	}

	cfg := defaultGeneratorConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("apply option failed: %w", err)
		}
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Acquire unique machine ID from repo
	machineID, err := repo.AcquireMachineID(ctx, cfg.ttl)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAcquireMachineID, err)
	}

	// Validate machine ID fits in configured bit space (use package default)
	maxMachineID := 1 << defaultBitsMachine
	if machineID < 0 || machineID >= maxMachineID {
		_ = repo.ReleaseMachineID(ctx, machineID)
		return nil, fmt.Errorf("machine ID %d exceeds bit space (0-%d)", machineID, maxMachineID-1)
	}

	cfg.settings.MachineID = func() (int, error) { return machineID, nil }
	cfg.settings.CheckMachineID = func(id int) bool { return id == machineID }

	sf, err := sonyflake.New(cfg.settings)
	if err != nil {
		_ = repo.ReleaseMachineID(ctx, machineID)
		return nil, fmt.Errorf("sonyflake initialization failed: %w", err)
	}

	g := &Generator{
		sf:        sf,
		repo:      repo,
		machineID: machineID,
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
		ttl:       cfg.ttl,
		renewFreq: cfg.renewFreq,
	}

	// Start background heartbeat to keep machine ID alive
	go g.heartbeat()

	return g, nil
}

// NextID generates the next unique ID
func (g *Generator) NextID() (int64, error) {
	return g.sf.NextID()
}

// ToTime converts an ID back to its generation time
func (g *Generator) ToTime(id int64) time.Time {
	return g.sf.ToTime(id)
}

// Decompose breaks an ID into its components
func (g *Generator) Decompose(id int64) map[string]int64 {
	return g.sf.Decompose(id)
}

// Stop gracefully stops the generator and releases the machine ID
// Should be called before application shutdown
func (g *Generator) Stop(ctx context.Context) error {
	g.stopOnce.Do(func() {
		// Signal heartbeat to stop
		close(g.stopChan)
		// Wait for heartbeat to exit
		<-g.doneChan
		// Release machine ID once
		if err := g.repo.ReleaseMachineID(ctx, g.machineID); err != nil {
			g.stopErr = fmt.Errorf("%w: %v", ErrReleaseMachineID, err)
		}
	})
	return g.stopErr
}

// heartbeat periodically renews the machine ID lease to maintain uniqueness
func (g *Generator) heartbeat() {
	ticker := time.NewTicker(g.renewFreq)
	defer ticker.Stop()

	defer close(g.doneChan)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = g.repo.RenewMachineID(ctx, g.machineID, g.ttl)
			cancel()
		case <-g.stopChan:
			return
		}
	}
}

// validateConfig ensures configuration meets production requirements
func validateConfig(cfg *generatorConfig) error {
	if cfg.settings.StartTime.After(time.Now()) {
		return ErrInvalidStartTime
	}
	if cfg.settings.TimeUnit < time.Millisecond {
		return ErrInvalidTimeUnit
	}
	if cfg.ttl <= 0 || cfg.renewFreq <= 0 {
		return errors.New("TTL and renew frequency must be positive")
	}
	if cfg.renewFreq >= cfg.ttl {
		return errors.New("renew frequency must be less than TTL")
	}
	return nil
}
