// Package zerologx provides a configurable wrapper around the zerolog logging library.
package zerologx

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Option defines logger configuration options
type Option func(*Config)

// Config is the logger configuration struct
type Config struct {
	level          zerolog.Level
	output         io.Writer
	timeFormat     string
	caller         bool
	sampling       *zerolog.BasicSampler
	hooks          []zerolog.Hook
	pretty         bool
	consoleTimeFmt string
}

// WithLevel sets the log level
func WithLevel(level zerolog.Level) Option {
	return func(c *Config) {
		c.level = level
	}
}

// WithOutput sets the output destination
func WithOutput(output io.Writer) Option {
	return func(c *Config) {
		c.output = output
	}
}

// WithTimeFormat sets the time format
func WithTimeFormat(format string) Option {
	return func(c *Config) {
		c.timeFormat = format
	}
}

// WithCaller enables caller information
func WithCaller(enabled bool) Option {
	return func(c *Config) {
		c.caller = enabled
	}
}

// WithSampling sets the sampling frequency (one out of every N records)
func WithSampling(n uint32) Option {
	return func(c *Config) {
		if n > 0 {
			c.sampling = &zerolog.BasicSampler{N: n}
		}
	}
}

// WithHook adds a log hook
func WithHook(hook zerolog.Hook) Option {
	return func(c *Config) {
		c.hooks = append(c.hooks, hook)
	}
}

// WithPretty enables pretty output (for development environment)
func WithPretty(enabled bool) Option {
	return func(c *Config) {
		c.pretty = enabled
	}
}

// WithConsoleTimeFormat sets the console time format
func WithConsoleTimeFormat(format string) Option {
	return func(c *Config) {
		c.consoleTimeFmt = format
	}
}

// New initializes a zerolog Logger
// Required parameter: output (output destination)
// Optional parameters: passed via options pattern
func New(output io.Writer, opts ...Option) zerolog.Logger {
	// Set default configuration
	config := &Config{
		level:      zerolog.InfoLevel,
		output:     output,
		timeFormat: time.RFC3339,
		caller:     false,
		pretty:     false,
	}

	// Apply option configurations
	for _, opt := range opts {
		opt(config)
	}

	var logger zerolog.Logger

	// If pretty output is needed, use ConsoleWriter
	if config.pretty {
		consoleWriter := zerolog.ConsoleWriter{
			Out:     config.output,
			NoColor: false,
			TimeFormat: func() string {
				if config.consoleTimeFmt != "" {
					return config.consoleTimeFmt
				}
				return config.timeFormat
			}(),
		}
		logger = zerolog.New(consoleWriter).Level(config.level)
	} else {
		logger = zerolog.New(config.output).Level(config.level)
	}

	// Add timestamp
	logger = logger.With().Timestamp().Logger()

	// Enable caller information
	if config.caller {
		logger = logger.With().Caller().Logger()
	}

	// Set sampling
	if config.sampling != nil {
		logger = logger.Sample(config.sampling)
	}

	// Add hooks
	for _, hook := range config.hooks {
		logger = logger.Hook(hook)
	}

	return logger
}

// NewProduction creates a production environment logger instance
func NewProduction(output io.Writer, opts ...Option) zerolog.Logger {
	defaultOpts := []Option{
		WithLevel(zerolog.InfoLevel),
		WithTimeFormat(time.RFC3339),
		WithCaller(false), // Usually not enabled in production to avoid performance overhead
	}
	opts = append(defaultOpts, opts...)
	return New(output, opts...)
}

// NewDevelopment creates a development environment logger instance
func NewDevelopment(output io.Writer, opts ...Option) zerolog.Logger {
	defaultOpts := []Option{
		WithLevel(zerolog.DebugLevel),
		WithTimeFormat(time.StampMilli),
		WithCaller(true),
		WithPretty(true),
	}
	opts = append(defaultOpts, opts...)
	return New(output, opts...)
}

// DefaultLogger creates a default logger instance (output to stdout)
func DefaultLogger(opts ...Option) zerolog.Logger {
	return New(os.Stdout, opts...)
}

// NewFileLogger creates a file logger instance
func NewFileLogger(filepath string, opts ...Option) (zerolog.Logger, error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return zerolog.Logger{}, err
	}

	defaultOpts := []Option{
		WithOutput(file),
		WithTimeFormat(time.RFC3339),
	}
	opts = append(defaultOpts, opts...)

	logger := New(file, opts...)
	return logger, nil
}

func UpdateLogLevel(level zerolog.Level) {
	zerolog.SetGlobalLevel(level)
}
