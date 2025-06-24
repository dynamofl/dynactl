package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// LogLevelError represents error level logging
	LogLevelError LogLevel = iota
	// LogLevelWarning represents warning level logging
	LogLevelWarning
	// LogLevelInfo represents info level logging
	LogLevelInfo
	// LogLevelDebug represents debug level logging
	LogLevelDebug
)

var (
	// CurrentLogLevel is the current logging level
	CurrentLogLevel = LogLevelInfo
	// LogOutput is the output writer for logs
	LogOutput io.Writer = os.Stdout
)

// SetLogLevel sets the current logging level based on verbosity
func SetLogLevel(verbosity int) {
	switch verbosity {
	case 0:
		CurrentLogLevel = LogLevelWarning
	case 1:
		CurrentLogLevel = LogLevelInfo
	default:
		CurrentLogLevel = LogLevelDebug
	}
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	if CurrentLogLevel >= LogLevelError {
		fmt.Fprintf(LogOutput, "ERROR: %s\n", fmt.Sprintf(format, args...))
	}
}

// LogWarning logs a warning message
func LogWarning(format string, args ...interface{}) {
	if CurrentLogLevel >= LogLevelWarning {
		fmt.Fprintf(LogOutput, "WARNING: %s\n", fmt.Sprintf(format, args...))
	}
}

// LogInfo logs an info message
func LogInfo(format string, args ...interface{}) {
	if CurrentLogLevel >= LogLevelInfo {
		fmt.Fprintf(LogOutput, "INFO: %s\n", fmt.Sprintf(format, args...))
	}
}

// LogDebug logs a debug message
func LogDebug(format string, args ...interface{}) {
	if CurrentLogLevel >= LogLevelDebug {
		fmt.Fprintf(LogOutput, "DEBUG: %s\n", fmt.Sprintf(format, args...))
	}
}

// LogFatal logs a fatal error and exits
func LogFatal(format string, args ...interface{}) {
	fmt.Fprintf(LogOutput, "FATAL: %s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}

// LogLevelFromString converts a string to a LogLevel
func LogLevelFromString(level string) LogLevel {
	switch strings.ToLower(level) {
	case "error":
		return LogLevelError
	case "warning":
		return LogLevelWarning
	case "info":
		return LogLevelInfo
	case "debug":
		return LogLevelDebug
	default:
		return LogLevelInfo
	}
} 