package utils

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	LogOutput = &buf

	// Test different log levels
	SetLogLevel(0) // Warning level
	LogError("Error message")
	LogWarning("Warning message")
	LogInfo("Info message")
	LogDebug("Debug message")

	// Check that only error and warning messages are logged
	output := buf.String()
	if !strings.Contains(output, "ERROR: Error message") {
		t.Error("Expected error message not found")
	}
	if !strings.Contains(output, "WARNING: Warning message") {
		t.Error("Expected warning message not found")
	}
	if strings.Contains(output, "INFO: Info message") {
		t.Error("Info message should not be logged at warning level")
	}
	if strings.Contains(output, "DEBUG: Debug message") {
		t.Error("Debug message should not be logged at warning level")
	}

	// Clear buffer
	buf.Reset()

	// Test info level
	SetLogLevel(1) // Info level
	LogError("Error message")
	LogWarning("Warning message")
	LogInfo("Info message")
	LogDebug("Debug message")

	// Check that error, warning, and info messages are logged
	output = buf.String()
	if !strings.Contains(output, "ERROR: Error message") {
		t.Error("Expected error message not found")
	}
	if !strings.Contains(output, "WARNING: Warning message") {
		t.Error("Expected warning message not found")
	}
	if !strings.Contains(output, "INFO: Info message") {
		t.Error("Expected info message not found")
	}
	if strings.Contains(output, "DEBUG: Debug message") {
		t.Error("Debug message should not be logged at info level")
	}

	// Clear buffer
	buf.Reset()

	// Test debug level
	SetLogLevel(2) // Debug level
	LogError("Error message")
	LogWarning("Warning message")
	LogInfo("Info message")
	LogDebug("Debug message")

	// Check that all messages are logged
	output = buf.String()
	if !strings.Contains(output, "ERROR: Error message") {
		t.Error("Expected error message not found")
	}
	if !strings.Contains(output, "WARNING: Warning message") {
		t.Error("Expected warning message not found")
	}
	if !strings.Contains(output, "INFO: Info message") {
		t.Error("Expected info message not found")
	}
	if !strings.Contains(output, "DEBUG: Debug message") {
		t.Error("Expected debug message not found")
	}

	// Test LogLevelFromString
	if LogLevelFromString("error") != LogLevelError {
		t.Error("Expected LogLevelError for 'error'")
	}
	if LogLevelFromString("warning") != LogLevelWarning {
		t.Error("Expected LogLevelWarning for 'warning'")
	}
	if LogLevelFromString("info") != LogLevelInfo {
		t.Error("Expected LogLevelInfo for 'info'")
	}
	if LogLevelFromString("debug") != LogLevelDebug {
		t.Error("Expected LogLevelDebug for 'debug'")
	}
	if LogLevelFromString("invalid") != LogLevelInfo {
		t.Error("Expected LogLevelInfo for 'invalid'")
	}
} 