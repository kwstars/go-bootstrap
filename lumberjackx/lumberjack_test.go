package lumberjackx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoggerDefaults(t *testing.T) {
	tmpRoot := t.TempDir()
	customTmp := filepath.Join(tmpRoot, "nested-tmp")
	t.Setenv("TMPDIR", customTmp)

	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	expectedFilename := defaultFilename()
	if logger.Filename != expectedFilename {
		t.Fatalf("unexpected filename. got %q want %q", logger.Filename, expectedFilename)
	}
	if logger.MaxSize != defaultMaxSizeMB {
		t.Fatalf("unexpected max size. got %d want %d", logger.MaxSize, defaultMaxSizeMB)
	}
	if logger.MaxAge != defaultMaxAgeDays {
		t.Fatalf("unexpected max age. got %d want %d", logger.MaxAge, defaultMaxAgeDays)
	}
	if logger.MaxBackups != defaultMaxBackups {
		t.Fatalf("unexpected max backups. got %d want %d", logger.MaxBackups, defaultMaxBackups)
	}
	if !logger.LocalTime {
		t.Fatalf("expected LocalTime default to true")
	}
	if !logger.Compress {
		t.Fatalf("expected Compress default to true")
	}

	if info, err := os.Stat(filepath.Dir(expectedFilename)); err != nil || !info.IsDir() {
		t.Fatalf("expected log directory to exist: %v", err)
	}
}

func TestNewLoggerWithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	customFilename := filepath.Join(tmpDir, "logs", "custom.log")

	logger, err := NewLogger(
		WithFilename(customFilename),
		WithMaxSize(256),
		WithMaxAge(30),
		WithMaxBackups(3),
		WithLocalTime(false),
		WithCompress(false),
	)
	if err != nil {
		t.Fatalf("NewLogger with options returned error: %v", err)
	}

	if logger.Filename != customFilename {
		t.Fatalf("expected filename override. got %q want %q", logger.Filename, customFilename)
	}
	if logger.MaxSize != 256 {
		t.Fatalf("expected max size override to 256, got %d", logger.MaxSize)
	}
	if logger.MaxAge != 30 {
		t.Fatalf("expected max age override to 30, got %d", logger.MaxAge)
	}
	if logger.MaxBackups != 3 {
		t.Fatalf("expected max backups override to 3, got %d", logger.MaxBackups)
	}
	if logger.LocalTime {
		t.Fatalf("expected LocalTime override to false")
	}
	if logger.Compress {
		t.Fatalf("expected Compress override to false")
	}

	if info, err := os.Stat(filepath.Dir(customFilename)); err != nil || !info.IsDir() {
		t.Fatalf("expected custom log directory to be created: %v", err)
	}
}
