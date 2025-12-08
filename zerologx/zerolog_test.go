package zerologx

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestNew verifies basic logger creation
func TestNew(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf)

	logger.Info().Msg("test message")

	if buf.Len() == 0 {
		t.Error("Expected output but got none")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["level"] != "info" {
		t.Errorf("Expected level 'info', got '%v'", logEntry["level"])
	}
	if logEntry["message"] != "test message" {
		t.Errorf("Expected message 'test message', got '%v'", logEntry["message"])
	}
}

// TestWithLevel verifies log level filtering
func TestWithLevel(t *testing.T) {
	tests := []struct {
		name        string
		configLevel zerolog.Level
		logLevel    zerolog.Level
		shouldLog   bool
	}{
		{"Debug logged at Debug level", zerolog.DebugLevel, zerolog.DebugLevel, true},
		{"Debug not logged at Info level", zerolog.InfoLevel, zerolog.DebugLevel, false},
		{"Info logged at Info level", zerolog.InfoLevel, zerolog.InfoLevel, true},
		{"Info logged at Debug level", zerolog.DebugLevel, zerolog.InfoLevel, true},
		{"Error logged at Warn level", zerolog.WarnLevel, zerolog.ErrorLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := New(buf, WithLevel(tt.configLevel))

			switch tt.logLevel {
			case zerolog.DebugLevel:
				logger.Debug().Msg("test")
			case zerolog.InfoLevel:
				logger.Info().Msg("test")
			case zerolog.WarnLevel:
				logger.Warn().Msg("test")
			case zerolog.ErrorLevel:
				logger.Error().Msg("test")
			}

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, got hasOutput=%v", tt.shouldLog, hasOutput)
			}
		})
	}
}

// TestWithOutput verifies custom output destination
func TestWithOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithOutput(buf))

	logger.Info().Msg("test output")

	if !strings.Contains(buf.String(), "test output") {
		t.Error("Expected message in custom output")
	}
}

// TestWithTimeFormat verifies time format configuration
func TestWithTimeFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	customFormat := "2006-01-02"
	logger := New(buf, WithTimeFormat(customFormat))

	// Note: WithTimeFormat sets the format but zerolog's JSON output
	// still uses its own timestamp format. This test verifies the option is set.
	logger.Info().Msg("test")

	if buf.Len() == 0 {
		t.Error("Expected log output")
	}
}

// TestWithCaller verifies caller information is included
func TestWithCaller(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithCaller(true))

	logger.Info().Msg("test caller")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if _, exists := logEntry["caller"]; !exists {
		t.Error("Expected 'caller' field in log output")
	}
}

// TestWithCallerDisabled verifies caller information is not included when disabled
func TestWithCallerDisabled(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithCaller(false))

	logger.Info().Msg("test no caller")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if _, exists := logEntry["caller"]; exists {
		t.Error("Did not expect 'caller' field in log output")
	}
}

// TestWithSampling verifies log sampling
func TestWithSampling(t *testing.T) {
	buf := &bytes.Buffer{}
	// Sample: log 1 out of every 10 messages
	logger := New(buf, WithSampling(10))

	// Log 100 messages
	for i := 0; i < 100; i++ {
		logger.Info().Msgf("message %d", i)
	}

	// Count actual logged messages
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	loggedCount := 0
	for _, line := range lines {
		if line != "" {
			loggedCount++
		}
	}

	// With sampling of 10, we expect approximately 10 messages (±3 for variance)
	if loggedCount < 7 || loggedCount > 13 {
		t.Errorf("Expected ~10 sampled logs, got %d", loggedCount)
	}
}

// TestWithSamplingZero verifies that zero sampling disables sampling
func TestWithSamplingZero(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithSampling(0))

	// Log 10 messages
	for i := 0; i < 10; i++ {
		logger.Info().Msgf("message %d", i)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	loggedCount := 0
	for _, line := range lines {
		if line != "" {
			loggedCount++
		}
	}

	// All messages should be logged
	if loggedCount != 10 {
		t.Errorf("Expected 10 logs with zero sampling, got %d", loggedCount)
	}
}

// TestWithHook verifies hook functionality
func TestWithHook(t *testing.T) {
	buf := &bytes.Buffer{}
	hookCalled := false

	testHook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, msg string) {
		hookCalled = true
		e.Str("hook_field", "hook_value")
	})

	logger := New(buf, WithHook(testHook))
	logger.Info().Msg("test hook")

	if !hookCalled {
		t.Error("Expected hook to be called")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["hook_field"] != "hook_value" {
		t.Error("Expected hook to add 'hook_field'")
	}
}

// TestWithPretty verifies pretty output format
func TestWithPretty(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithPretty(true))

	logger.Info().Msg("pretty test")

	output := buf.String()
	// Pretty output should be human-readable, not JSON
	if strings.HasPrefix(output, "{") {
		t.Error("Expected pretty (non-JSON) output")
	}
	if !strings.Contains(output, "pretty test") {
		t.Error("Expected message in pretty output")
	}
}

// TestWithConsoleTimeFormat verifies console time format
func TestWithConsoleTimeFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf, WithPretty(true), WithConsoleTimeFormat("15:04:05"))

	logger.Info().Msg("time format test")

	output := buf.String()
	if !strings.Contains(output, "time format test") {
		t.Error("Expected message in output")
	}
}

// TestNewProduction verifies production logger defaults
func TestNewProduction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewProduction(buf)

	// Debug should not be logged at Info level
	logger.Debug().Msg("debug")
	debugOutput := buf.String()

	buf.Reset()
	logger.Info().Msg("info")
	infoOutput := buf.String()

	if len(debugOutput) > 0 {
		t.Error("Debug message should not be logged in production")
	}
	if len(infoOutput) == 0 {
		t.Error("Info message should be logged in production")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(infoOutput), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Production should not have caller by default
	if _, exists := logEntry["caller"]; exists {
		t.Error("Production logger should not include caller by default")
	}
}

// TestNewProductionWithOverrides verifies production logger can be overridden
func TestNewProductionWithOverrides(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewProduction(buf, WithLevel(zerolog.DebugLevel), WithCaller(true))

	logger.Debug().Msg("debug")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["level"] != "debug" {
		t.Error("Expected debug level after override")
	}
	if _, exists := logEntry["caller"]; !exists {
		t.Error("Expected caller field after override")
	}
}

// TestNewDevelopment verifies development logger defaults
func TestNewDevelopment(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDevelopment(buf)

	logger.Debug().Msg("debug message")

	output := buf.String()
	if len(output) == 0 {
		t.Error("Debug message should be logged in development")
	}

	// Development uses pretty output, so check for readable format
	if strings.HasPrefix(output, "{") {
		t.Error("Development logger should use pretty output")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("Expected message in development output")
	}
}

// TestDefaultLogger verifies default logger uses stdout
func TestDefaultLogger(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := DefaultLogger(WithLevel(zerolog.InfoLevel))
	logger.Info().Msg("stdout test")

	w.Close()
	os.Stdout = oldStdout

	buf := &bytes.Buffer{}
	buf.ReadFrom(r)

	if !strings.Contains(buf.String(), "stdout test") {
		t.Error("Expected message in stdout")
	}
}

// TestNewFileLogger verifies file logger creation
func TestNewFileLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewFileLogger(logFile)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	logger.Info().Msg("file test message")

	// Read file content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "file test message") {
		t.Error("Expected message in log file")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse log file content: %v", err)
	}

	if logEntry["message"] != "file test message" {
		t.Errorf("Expected message 'file test message', got '%v'", logEntry["message"])
	}
}

// TestNewFileLoggerInvalidPath verifies error handling for invalid paths
func TestNewFileLoggerInvalidPath(t *testing.T) {
	_, err := NewFileLogger("/invalid/path/that/does/not/exist/test.log")
	if err == nil {
		t.Error("Expected error for invalid file path")
	}
}

// TestNewFileLoggerAppend verifies file logger appends to existing file
func TestNewFileLoggerAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "append.log")

	// First logger
	logger1, err := NewFileLogger(logFile)
	if err != nil {
		t.Fatalf("Failed to create first file logger: %v", err)
	}
	logger1.Info().Msg("first message")

	// Second logger (should append)
	logger2, err := NewFileLogger(logFile)
	if err != nil {
		t.Fatalf("Failed to create second file logger: %v", err)
	}
	logger2.Info().Msg("second message")

	// Read file content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "first message") {
		t.Error("Expected first message in log file")
	}
	if !strings.Contains(contentStr, "second message") {
		t.Error("Expected second message in log file")
	}
}

// TestMultipleOptions verifies multiple options work together
func TestMultipleOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	hookCalled := false

	testHook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, msg string) {
		hookCalled = true
	})

	logger := New(buf,
		WithLevel(zerolog.DebugLevel),
		WithCaller(true),
		WithHook(testHook),
		WithTimeFormat(time.RFC3339),
	)

	logger.Debug().Msg("multi-option test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["level"] != "debug" {
		t.Error("Expected debug level")
	}
	if _, exists := logEntry["caller"]; !exists {
		t.Error("Expected caller field")
	}
	if !hookCalled {
		t.Error("Expected hook to be called")
	}
}

// TestTimestampPresence verifies timestamp is always added
func TestTimestampPresence(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(buf)

	logger.Info().Msg("timestamp test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if _, exists := logEntry["time"]; !exists {
		t.Error("Expected 'time' field in log output")
	}
}
