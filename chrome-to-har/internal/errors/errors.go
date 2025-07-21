// Package errors provides comprehensive error handling utilities for chrome-to-har.
package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	// Chrome-related errors
	ChromeLaunchError     ErrorType = "chrome_launch"
	ChromeConnectionError ErrorType = "chrome_connection"
	ChromeNavigationError ErrorType = "chrome_navigation"
	ChromeScriptError     ErrorType = "chrome_script"
	ChromeTimeoutError    ErrorType = "chrome_timeout"

	// Profile-related errors
	ProfileNotFoundError ErrorType = "profile_not_found"
	ProfileCopyError     ErrorType = "profile_copy"
	ProfileSetupError    ErrorType = "profile_setup"

	// Network-related errors
	NetworkError      ErrorType = "network"
	NetworkIdleError  ErrorType = "network_idle"
	NetworkRecordError ErrorType = "network_record"

	// File system errors
	FileNotFoundError ErrorType = "file_not_found"
	FilePermissionError ErrorType = "file_permission"
	FileWriteError    ErrorType = "file_write"
	FileReadError     ErrorType = "file_read"

	// Input validation errors
	ValidationError     ErrorType = "validation"
	InvalidURLError     ErrorType = "invalid_url"
	InvalidHeaderError  ErrorType = "invalid_header"
	InvalidScriptError  ErrorType = "invalid_script"

	// Configuration errors
	ConfigurationError ErrorType = "configuration"
	ProxyError         ErrorType = "proxy"
	AuthenticationError ErrorType = "authentication"

	// Generic errors
	InternalError ErrorType = "internal"
	TimeoutError  ErrorType = "timeout"
	CancelError   ErrorType = "cancel"
)

// ChromeError represents an error that occurs when interacting with Chrome
type ChromeError struct {
	Type      ErrorType
	Message   string
	Cause     error
	Context   map[string]interface{}
	Retryable bool
}

// Error implements the error interface
func (e *ChromeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *ChromeError) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of a specific type
func (e *ChromeError) Is(target error) bool {
	if t, ok := target.(*ChromeError); ok {
		return e.Type == t.Type
	}
	return false
}

// IsRetryable returns whether the error is retryable
func (e *ChromeError) IsRetryable() bool {
	return e.Retryable
}

// GetContext returns the error context
func (e *ChromeError) GetContext() map[string]interface{} {
	return e.Context
}

// UserMessage returns a user-friendly error message
func (e *ChromeError) UserMessage() string {
	switch e.Type {
	case ChromeLaunchError:
		return "Failed to launch Chrome browser. Please check that Chrome is installed and accessible."
	case ChromeConnectionError:
		return "Failed to connect to Chrome browser. Please check if Chrome is running and the debug port is available."
	case ChromeNavigationError:
		return "Failed to navigate to the requested URL. Please check the URL and your internet connection."
	case ChromeTimeoutError:
		return "Chrome operation timed out. Try increasing the timeout value or check your network connection."
	case ProfileNotFoundError:
		return "Chrome profile not found. Please check the profile name and ensure it exists."
	case NetworkError:
		return "Network error occurred. Please check your internet connection and try again."
	case FileNotFoundError:
		return "File not found. Please check the file path and ensure the file exists."
	case FilePermissionError:
		return "Permission denied. Please check file permissions and ensure you have the necessary access."
	case ValidationError:
		return "Invalid input provided. Please check your input and try again."
	case InvalidURLError:
		return "Invalid URL format. Please provide a valid URL."
	case ProxyError:
		return "Proxy configuration error. Please check your proxy settings."
	case AuthenticationError:
		return "Authentication failed. Please check your credentials."
	case TimeoutError:
		return "Operation timed out. Please try again or increase the timeout value."
	default:
		return e.Message
	}
}

// Suggestions returns actionable suggestions for the user
func (e *ChromeError) Suggestions() []string {
	switch e.Type {
	case ChromeLaunchError:
		return []string{
			"Ensure Chrome is installed on your system",
			"Check if Chrome is in your PATH or specify --chrome-path",
			"Try running with --headless flag",
			"Close any existing Chrome instances that might be interfering",
		}
	case ChromeConnectionError:
		return []string{
			"Try a different debug port with --debug-port",
			"Ensure no other processes are using the debug port",
			"Increase timeout with --timeout",
			"Check if Chrome is already running in debug mode",
		}
	case ChromeNavigationError:
		return []string{
			"Check if the URL is valid and accessible",
			"Verify your internet connection",
			"Try increasing the timeout value",
			"Check if the site requires authentication",
		}
	case ProfileNotFoundError:
		return []string{
			"List available profiles with --list-profiles",
			"Check the profile name spelling",
			"Ensure the profile directory exists",
		}
	case NetworkError:
		return []string{
			"Check your internet connection",
			"Verify proxy settings if using a proxy",
			"Try accessing the URL directly in a browser",
		}
	case FilePermissionError:
		return []string{
			"Check file permissions",
			"Ensure you have write access to the output directory",
			"Try running with elevated permissions if necessary",
		}
	case ValidationError:
		return []string{
			"Check input format and syntax",
			"Refer to the documentation for valid input examples",
			"Use --help for command usage information",
		}
	case ProxyError:
		return []string{
			"Verify proxy server address and port",
			"Check proxy authentication credentials",
			"Try connecting without proxy to test",
		}
	case TimeoutError:
		return []string{
			"Increase timeout with --timeout flag",
			"Check your network connection speed",
			"Try the operation during off-peak hours",
		}
	default:
		return []string{"Please check the error message for more details"}
	}
}

// New creates a new ChromeError
func New(errorType ErrorType, message string) *ChromeError {
	return &ChromeError{
		Type:      errorType,
		Message:   message,
		Context:   make(map[string]interface{}),
		Retryable: isRetryableByDefault(errorType),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errorType ErrorType, message string) *ChromeError {
	return &ChromeError{
		Type:      errorType,
		Message:   message,
		Cause:     err,
		Context:   make(map[string]interface{}),
		Retryable: isRetryableByDefault(errorType),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *ChromeError {
	return &ChromeError{
		Type:      errorType,
		Message:   fmt.Sprintf(format, args...),
		Cause:     err,
		Context:   make(map[string]interface{}),
		Retryable: isRetryableByDefault(errorType),
	}
}

// WithContext adds context to an error
func WithContext(err *ChromeError, key string, value interface{}) *ChromeError {
	if err.Context == nil {
		err.Context = make(map[string]interface{})
	}
	err.Context[key] = value
	return err
}

// WithRetryable sets the retryable flag
func WithRetryable(err *ChromeError, retryable bool) *ChromeError {
	err.Retryable = retryable
	return err
}

// isRetryableByDefault determines if an error type is retryable by default
func isRetryableByDefault(errorType ErrorType) bool {
	switch errorType {
	case ChromeTimeoutError, NetworkError, NetworkIdleError, TimeoutError:
		return true
	case ChromeConnectionError, ChromeLaunchError:
		return true // Can retry with different settings
	default:
		return false
	}
}

// FormatError formats an error with full context for logging
func FormatError(err error) string {
	if chromeErr, ok := err.(*ChromeError); ok {
		var parts []string
		parts = append(parts, fmt.Sprintf("Type: %s", chromeErr.Type))
		parts = append(parts, fmt.Sprintf("Message: %s", chromeErr.Message))
		
		if chromeErr.Cause != nil {
			parts = append(parts, fmt.Sprintf("Cause: %v", chromeErr.Cause))
		}
		
		if len(chromeErr.Context) > 0 {
			contextParts := make([]string, 0, len(chromeErr.Context))
			for k, v := range chromeErr.Context {
				contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
			}
			parts = append(parts, fmt.Sprintf("Context: %s", strings.Join(contextParts, ", ")))
		}
		
		parts = append(parts, fmt.Sprintf("Retryable: %t", chromeErr.Retryable))
		
		return strings.Join(parts, " | ")
	}
	
	return fmt.Sprintf("Error: %v", err)
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if chromeErr, ok := err.(*ChromeError); ok {
		return chromeErr.Type == errorType
	}
	return false
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if chromeErr, ok := err.(*ChromeError); ok {
		return chromeErr.Retryable
	}
	return false
}

// GetUserMessage returns a user-friendly error message
func GetUserMessage(err error) string {
	if chromeErr, ok := err.(*ChromeError); ok {
		return chromeErr.UserMessage()
	}
	return err.Error()
}

// GetSuggestions returns actionable suggestions for an error
func GetSuggestions(err error) []string {
	if chromeErr, ok := err.(*ChromeError); ok {
		return chromeErr.Suggestions()
	}
	return []string{"Please check the error message for more details"}
}

// NewValidationError creates a validation error with details
func NewValidationError(field, message string) *ChromeError {
	return WithContext(New(ValidationError, message), "field", field)
}

// FileError creates a file-related error
func FileError(operation, path string, err error) *ChromeError {
	var errorType ErrorType
	if strings.Contains(err.Error(), "permission denied") {
		errorType = FilePermissionError
	} else if strings.Contains(err.Error(), "no such file") {
		errorType = FileNotFoundError
	} else {
		errorType = FileWriteError
	}
	
	chromeErr := Wrapf(err, errorType, "failed to %s file", operation)
	return WithContext(chromeErr, "path", path)
}

// NewNetworkError creates a network-related error
func NewNetworkError(operation, url string, err error) *ChromeError {
	chromeErr := Wrapf(err, NetworkError, "network error during %s", operation)
	return WithContext(chromeErr, "url", url)
}

// NewChromeError creates a Chrome-related error
func NewChromeError(operation string, err error) *ChromeError {
	var errorType ErrorType
	errStr := strings.ToLower(err.Error())
	
	switch {
	case strings.Contains(errStr, "timeout"):
		errorType = ChromeTimeoutError
	case strings.Contains(errStr, "connection"):
		errorType = ChromeConnectionError
	case strings.Contains(errStr, "navigation"):
		errorType = ChromeNavigationError
	case strings.Contains(errStr, "script"):
		errorType = ChromeScriptError
	default:
		errorType = ChromeLaunchError
	}
	
	return Wrapf(err, errorType, "Chrome %s failed", operation)
}