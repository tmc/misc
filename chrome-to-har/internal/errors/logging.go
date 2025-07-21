// Package errors provides structured logging for error handling.
package errors

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity level of log messages
type LogLevel int

const (
	// LogLevelDebug for detailed debugging information
	LogLevelDebug LogLevel = iota
	// LogLevelInfo for general information
	LogLevelInfo
	// LogLevelWarn for warning messages
	LogLevelWarn
	// LogLevelError for error messages
	LogLevelError
	// LogLevelFatal for fatal error messages
	LogLevelFatal
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ErrorLogger provides structured error logging capabilities
type ErrorLogger struct {
	level   LogLevel
	verbose bool
	logger  *log.Logger
}

// NewErrorLogger creates a new error logger with the specified configuration
func NewErrorLogger(level LogLevel, verbose bool) *ErrorLogger {
	return &ErrorLogger{
		level:   level,
		verbose: verbose,
		logger:  log.New(os.Stderr, "", log.LstdFlags),
	}
}

// DefaultErrorLogger returns a default error logger
func DefaultErrorLogger() *ErrorLogger {
	return NewErrorLogger(LogLevelInfo, false)
}

// SetLevel sets the minimum log level
func (el *ErrorLogger) SetLevel(level LogLevel) {
	el.level = level
}

// SetVerbose enables or disables verbose logging
func (el *ErrorLogger) SetVerbose(verbose bool) {
	el.verbose = verbose
}

// shouldLog checks if a message should be logged based on the current level
func (el *ErrorLogger) shouldLog(level LogLevel) bool {
	return level >= el.level
}

// Debug logs a debug message
func (el *ErrorLogger) Debug(format string, args ...interface{}) {
	if el.shouldLog(LogLevelDebug) {
		el.log(LogLevelDebug, format, args...)
	}
}

// Info logs an info message
func (el *ErrorLogger) Info(format string, args ...interface{}) {
	if el.shouldLog(LogLevelInfo) {
		el.log(LogLevelInfo, format, args...)
	}
}

// Warn logs a warning message
func (el *ErrorLogger) Warn(format string, args ...interface{}) {
	if el.shouldLog(LogLevelWarn) {
		el.log(LogLevelWarn, format, args...)
	}
}

// Error logs an error message
func (el *ErrorLogger) Error(format string, args ...interface{}) {
	if el.shouldLog(LogLevelError) {
		el.log(LogLevelError, format, args...)
	}
}

// Fatal logs a fatal error message
func (el *ErrorLogger) Fatal(format string, args ...interface{}) {
	if el.shouldLog(LogLevelFatal) {
		el.log(LogLevelFatal, format, args...)
	}
}

// log performs the actual logging
func (el *ErrorLogger) log(level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	el.logger.Printf("[%s] %s", level.String(), message)
}

// LogError logs an error with appropriate level and context
func (el *ErrorLogger) LogError(err error) {
	if err == nil {
		return
	}

	if chromeErr, ok := err.(*ChromeError); ok {
		level := el.getLogLevelForError(chromeErr)
		
		if el.verbose {
			el.log(level, "Chrome error: %s", FormatError(err))
		} else {
			el.log(level, "Error: %s", chromeErr.UserMessage())
		}
		
		// Log suggestions if available and at appropriate level
		if level >= LogLevelWarn && len(chromeErr.Suggestions()) > 0 {
			suggestions := strings.Join(chromeErr.Suggestions(), ", ")
			el.log(LogLevelInfo, "Suggestions: %s", suggestions)
		}
	} else {
		el.Error("Unexpected error: %v", err)
	}
}

// getLogLevelForError determines the appropriate log level for a Chrome error
func (el *ErrorLogger) getLogLevelForError(err *ChromeError) LogLevel {
	switch err.Type {
	case ValidationError, InvalidURLError, InvalidHeaderError, InvalidScriptError:
		return LogLevelWarn
	case ConfigurationError:
		return LogLevelWarn
	case ProfileNotFoundError, ProfileSetupError, ProfileCopyError:
		return LogLevelError
	case ChromeLaunchError, ChromeConnectionError:
		return LogLevelError
	case ChromeNavigationError, ChromeScriptError:
		return LogLevelError
	case ChromeTimeoutError, NetworkIdleError, TimeoutError:
		return LogLevelWarn
	case NetworkError, NetworkRecordError:
		return LogLevelError
	case FileNotFoundError, FilePermissionError, FileWriteError, FileReadError:
		return LogLevelError
	case ProxyError, AuthenticationError:
		return LogLevelError
	case InternalError:
		return LogLevelError
	case CancelError:
		return LogLevelInfo
	default:
		return LogLevelError
	}
}

// LogRetryAttempt logs a retry attempt
func (el *ErrorLogger) LogRetryAttempt(attempt int, maxAttempts int, err error, delay string) {
	if el.shouldLog(LogLevelInfo) {
		el.Info("Retry attempt %d/%d failed: %v (retrying in %s)", attempt, maxAttempts, err, delay)
	}
}

// LogRetrySuccess logs a successful retry
func (el *ErrorLogger) LogRetrySuccess(attempt int) {
	if el.shouldLog(LogLevelInfo) {
		el.Info("Operation succeeded after %d attempts", attempt)
	}
}

// LogRetryFailure logs a failed retry sequence
func (el *ErrorLogger) LogRetryFailure(maxAttempts int, err error) {
	if el.shouldLog(LogLevelError) {
		el.Error("Operation failed after %d attempts: %v", maxAttempts, err)
	}
}

// LogOperation logs the start of an operation
func (el *ErrorLogger) LogOperation(operation string, details ...string) {
	if el.shouldLog(LogLevelDebug) {
		if len(details) > 0 {
			el.Debug("Starting operation: %s (%s)", operation, strings.Join(details, ", "))
		} else {
			el.Debug("Starting operation: %s", operation)
		}
	}
}

// LogOperationSuccess logs successful completion of an operation
func (el *ErrorLogger) LogOperationSuccess(operation string, duration string) {
	if el.shouldLog(LogLevelInfo) {
		if duration != "" {
			el.Info("Operation completed successfully: %s (took %s)", operation, duration)
		} else {
			el.Info("Operation completed successfully: %s", operation)
		}
	}
}

// LogOperationFailure logs failed operation
func (el *ErrorLogger) LogOperationFailure(operation string, err error) {
	if el.shouldLog(LogLevelError) {
		el.Error("Operation failed: %s - %v", operation, err)
	}
}

// Global error logger instance
var globalErrorLogger = DefaultErrorLogger()

// SetGlobalLogLevel sets the global log level
func SetGlobalLogLevel(level LogLevel) {
	globalErrorLogger.SetLevel(level)
}

// SetGlobalVerbose enables or disables verbose logging globally
func SetGlobalVerbose(verbose bool) {
	globalErrorLogger.SetVerbose(verbose)
}

// Global logging functions
func LogDebug(format string, args ...interface{}) {
	globalErrorLogger.Debug(format, args...)
}

func LogInfo(format string, args ...interface{}) {
	globalErrorLogger.Info(format, args...)
}

func LogWarn(format string, args ...interface{}) {
	globalErrorLogger.Warn(format, args...)
}

func LogError(format string, args ...interface{}) {
	globalErrorLogger.Error(format, args...)
}

func LogFatal(format string, args ...interface{}) {
	globalErrorLogger.Fatal(format, args...)
}

// LogErrorWithContext logs an error with context information
func LogErrorWithContext(err error) {
	globalErrorLogger.LogError(err)
}

// LogRetryAttempt logs a retry attempt globally
func LogRetryAttempt(attempt int, maxAttempts int, err error, delay string) {
	globalErrorLogger.LogRetryAttempt(attempt, maxAttempts, err, delay)
}

// LogRetrySuccess logs a successful retry globally
func LogRetrySuccess(attempt int) {
	globalErrorLogger.LogRetrySuccess(attempt)
}

// LogRetryFailure logs a failed retry sequence globally
func LogRetryFailure(maxAttempts int, err error) {
	globalErrorLogger.LogRetryFailure(maxAttempts, err)
}

// LogOperation logs the start of an operation globally
func LogOperation(operation string, details ...string) {
	globalErrorLogger.LogOperation(operation, details...)
}

// LogOperationSuccess logs successful completion of an operation globally
func LogOperationSuccess(operation string, duration string) {
	globalErrorLogger.LogOperationSuccess(operation, duration)
}

// LogOperationFailure logs failed operation globally
func LogOperationFailure(operation string, err error) {
	globalErrorLogger.LogOperationFailure(operation, err)
}

// WithErrorLogging wraps a function to automatically log errors
func WithErrorLogging(operation string, fn func() error) error {
	LogOperation(operation)
	err := fn()
	if err != nil {
		LogOperationFailure(operation, err)
	} else {
		LogOperationSuccess(operation, "")
	}
	return err
}

// WithErrorLoggingAndResult wraps a function to automatically log errors and return results
func WithErrorLoggingAndResult[T any](operation string, fn func() (T, error)) (T, error) {
	LogOperation(operation)
	result, err := fn()
	if err != nil {
		LogOperationFailure(operation, err)
	} else {
		LogOperationSuccess(operation, "")
	}
	return result, err
}