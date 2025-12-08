package lumberjackx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultMaxSizeMB   = 100
	defaultMaxAgeDays  = 7
	defaultMaxBackups  = 7
	defaultFilenameFmt = "%s-lumberjack.log"
)

// Option defines the function signature for configuration options.
type Option func(*lumberjack.Logger) error

// WithFilename sets the log file path.
// Default: <processname>-lumberjack.log in os.TempDir().
func WithFilename(filename string) Option {
	return func(l *lumberjack.Logger) error {
		if filename == "" {
			return errors.New("filename cannot be empty")
		}
		if err := ensureLogDir(filename); err != nil {
			return err
		}
		l.Filename = filename
		return nil
	}
}

// WithMaxSize sets the maximum size of a single log file (MB).
// Default: 100 MB.
func WithMaxSize(sizeMB int) Option {
	return func(l *lumberjack.Logger) error {
		if sizeMB <= 0 {
			return errors.New("maxsize must be positive")
		}
		l.MaxSize = sizeMB
		return nil
	}
}

// WithMaxAge sets the maximum retention days for log files.
// Default: 7 days.
func WithMaxAge(days int) Option {
	return func(l *lumberjack.Logger) error {
		if days < 0 {
			return errors.New("maxage cannot be negative")
		}
		l.MaxAge = days
		return nil
	}
}

// WithMaxBackups sets the maximum number of backup files.
// Default: 7 backups.
func WithMaxBackups(count int) Option {
	return func(l *lumberjack.Logger) error {
		if count < 0 {
			return errors.New("maxbackups cannot be negative")
		}
		l.MaxBackups = count
		return nil
	}
}

// WithLocalTime sets whether to use local time for backup file naming.
// Default: true.
func WithLocalTime(useLocal bool) Option {
	return func(l *lumberjack.Logger) error {
		l.LocalTime = useLocal
		return nil
	}
}

// WithCompress sets whether to compress old log files.
// Default: true.
func WithCompress(compress bool) Option {
	return func(l *lumberjack.Logger) error {
		l.Compress = compress
		return nil
	}
}

// NewLogger creates and configures a lumberjack.Logger instance.
// All parameters are optional; defaults are used when not specified.
func NewLogger(opts ...Option) (*lumberjack.Logger, error) {
	filename := defaultFilename()

	// Create instance and explicitly set default values.
	logger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    defaultMaxSizeMB,
		MaxAge:     defaultMaxAgeDays,
		MaxBackups: defaultMaxBackups,
		LocalTime:  true,
		Compress:   true,
	}

	// Apply all options.
	for _, opt := range opts {
		if err := opt(logger); err != nil {
			return nil, fmt.Errorf("apply option failed: %w", err)
		}
	}

	if err := ensureLogDir(logger.Filename); err != nil {
		return nil, err
	}

	return logger, nil
}

// MustNewLogger creates a Logger and panics on error (suitable for startup phase).
func MustNewLogger(opts ...Option) *lumberjack.Logger {
	logger, err := NewLogger(opts...)
	if err != nil {
		panic(fmt.Sprintf("create logger failed: %v", err))
	}
	return logger
}

func ensureLogDir(filename string) error {
	dir := filepath.Dir(filename)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create log directory: %w", err)
	}
	return nil
}

func defaultFilename() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf(defaultFilenameFmt, defaultProcessName()))
}

func defaultProcessName() string {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	name := filepath.Base(exePath)
	if trimmed := strings.TrimSuffix(name, filepath.Ext(name)); trimmed != "" {
		name = trimmed
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return "app"
	}
	return name
}
