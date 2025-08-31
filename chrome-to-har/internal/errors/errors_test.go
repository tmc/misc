package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestChromeError(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		err := New(ChromeLaunchError, "failed to launch Chrome")

		if err.Type != ChromeLaunchError {
			t.Errorf("Expected type %v, got %v", ChromeLaunchError, err.Type)
		}

		if err.Message != "failed to launch Chrome" {
			t.Errorf("Expected message 'failed to launch Chrome', got '%s'", err.Message)
		}

		if !err.Retryable {
			t.Error("Expected ChromeLaunchError to be retryable by default")
		}
	})

	t.Run("Wrap", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Wrap(originalErr, NetworkError, "network operation failed")

		if err.Type != NetworkError {
			t.Errorf("Expected type %v, got %v", NetworkError, err.Type)
		}

		if err.Cause != originalErr {
			t.Errorf("Expected cause to be original error, got %v", err.Cause)
		}

		if !strings.Contains(err.Error(), "network operation failed") {
			t.Errorf("Expected error message to contain 'network operation failed', got '%s'", err.Error())
		}
	})

	t.Run("WithContext", func(t *testing.T) {
		err := New(ValidationError, "validation failed")
		err = WithContext(err, "field", "username")

		if err.Context["field"] != "username" {
			t.Errorf("Expected context field to be 'username', got %v", err.Context["field"])
		}
	})

	t.Run("UserMessage", func(t *testing.T) {
		err := New(ChromeLaunchError, "chrome failed")
		message := err.UserMessage()

		if !strings.Contains(message, "Chrome") {
			t.Errorf("Expected user message to contain 'Chrome', got '%s'", message)
		}
	})

	t.Run("Suggestions", func(t *testing.T) {
		err := New(ChromeLaunchError, "chrome failed")
		suggestions := err.Suggestions()

		if len(suggestions) == 0 {
			t.Error("Expected suggestions for ChromeLaunchError, got none")
		}

		// Check that suggestions contain helpful information
		found := false
		for _, suggestion := range suggestions {
			if strings.Contains(strings.ToLower(suggestion), "chrome") {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected at least one suggestion to mention Chrome")
		}
	})
}

func TestErrorTypes(t *testing.T) {
	testCases := []struct {
		errorType ErrorType
		retryable bool
	}{
		{ChromeLaunchError, true},
		{ChromeConnectionError, true},
		{ChromeNavigationError, false},
		{ChromeTimeoutError, true},
		{NetworkError, true},
		{ValidationError, false},
		{FileNotFoundError, false},
		{ProxyError, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.errorType), func(t *testing.T) {
			err := New(tc.errorType, "test message")

			if err.Retryable != tc.retryable {
				t.Errorf("Expected %v to be retryable=%v, got %v", tc.errorType, tc.retryable, err.Retryable)
			}
		})
	}
}

func TestIsType(t *testing.T) {
	err := New(ChromeLaunchError, "launch failed")

	if !IsType(err, ChromeLaunchError) {
		t.Error("Expected IsType to return true for matching type")
	}

	if IsType(err, NetworkError) {
		t.Error("Expected IsType to return false for non-matching type")
	}

	// Test with non-ChromeError
	regularErr := errors.New("regular error")
	if IsType(regularErr, ChromeLaunchError) {
		t.Error("Expected IsType to return false for non-ChromeError")
	}
}

func TestIsRetryable(t *testing.T) {
	retryableErr := New(ChromeTimeoutError, "timeout")
	nonRetryableErr := New(ValidationError, "validation failed")

	if !IsRetryable(retryableErr) {
		t.Error("Expected timeout error to be retryable")
	}

	if IsRetryable(nonRetryableErr) {
		t.Error("Expected validation error to not be retryable")
	}

	// Test with non-ChromeError
	regularErr := errors.New("regular error")
	if IsRetryable(regularErr) {
		t.Error("Expected regular error to not be retryable")
	}
}

func TestGetUserMessage(t *testing.T) {
	chromeErr := New(ChromeLaunchError, "failed to launch")
	regularErr := errors.New("regular error")

	chromeMessage := GetUserMessage(chromeErr)
	regularMessage := GetUserMessage(regularErr)

	if !strings.Contains(chromeMessage, "Chrome") {
		t.Errorf("Expected Chrome error message to contain 'Chrome', got '%s'", chromeMessage)
	}

	if regularMessage != "regular error" {
		t.Errorf("Expected regular error message to be unchanged, got '%s'", regularMessage)
	}
}

func TestGetSuggestions(t *testing.T) {
	chromeErr := New(ChromeLaunchError, "failed to launch")
	regularErr := errors.New("regular error")

	chromeSuggestions := GetSuggestions(chromeErr)
	regularSuggestions := GetSuggestions(regularErr)

	if len(chromeSuggestions) == 0 {
		t.Error("Expected Chrome error to have suggestions")
	}

	if len(regularSuggestions) == 0 {
		t.Error("Expected regular error to have default suggestions")
	}
}

func TestFormatError(t *testing.T) {
	err := WithContext(
		New(ChromeLaunchError, "failed to launch"),
		"debug_port", 9222,
	)

	formatted := FormatError(err)

	expectedParts := []string{
		"Type: chrome_launch",
		"Message: failed to launch",
		"Context: debug_port=9222",
		"Retryable: true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(formatted, part) {
			t.Errorf("Expected formatted error to contain '%s', got '%s'", part, formatted)
		}
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("username", "cannot be empty")

	if err.Type != ValidationError {
		t.Errorf("Expected type %v, got %v", ValidationError, err.Type)
	}

	if err.Context["field"] != "username" {
		t.Errorf("Expected field context to be 'username', got %v", err.Context["field"])
	}

	if !strings.Contains(err.Message, "cannot be empty") {
		t.Errorf("Expected message to contain 'cannot be empty', got '%s'", err.Message)
	}
}

func TestFileError(t *testing.T) {
	originalErr := errors.New("permission denied")
	err := FileError("write", "/tmp/test.txt", originalErr)

	if err.Type != FilePermissionError {
		t.Errorf("Expected type %v, got %v", FilePermissionError, err.Type)
	}

	if err.Context["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path context to be '/tmp/test.txt', got %v", err.Context["path"])
	}

	if err.Cause != originalErr {
		t.Errorf("Expected cause to be original error, got %v", err.Cause)
	}
}

func TestNewNetworkError(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := NewNetworkError("fetch", "https://example.com", originalErr)

	if err.Type != NetworkError {
		t.Errorf("Expected type %v, got %v", NetworkError, err.Type)
	}

	if err.Context["url"] != "https://example.com" {
		t.Errorf("Expected url context to be 'https://example.com', got %v", err.Context["url"])
	}

	if err.Cause != originalErr {
		t.Errorf("Expected cause to be original error, got %v", err.Cause)
	}
}

func TestNewChromeError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "timeout error",
			err:      errors.New("timeout waiting for response"),
			expected: ChromeTimeoutError,
		},
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: ChromeConnectionError,
		},
		{
			name:     "navigation error",
			err:      errors.New("navigation failed"),
			expected: ChromeNavigationError,
		},
		{
			name:     "script error",
			err:      errors.New("script execution failed"),
			expected: ChromeScriptError,
		},
		{
			name:     "generic error",
			err:      errors.New("unknown error"),
			expected: ChromeLaunchError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chromeErr := NewChromeError("test operation", tc.err)

			if chromeErr.Type != tc.expected {
				t.Errorf("Expected type %v, got %v", tc.expected, chromeErr.Type)
			}

			if chromeErr.Cause != tc.err {
				t.Errorf("Expected cause to be original error, got %v", chromeErr.Cause)
			}
		})
	}
}

func TestErrorUnwrapping(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, NetworkError, "network failed")

	if unwrapped := wrappedErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be original error, got %v", unwrapped)
	}

	// Test with errors.Is
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Expected errors.Is to find original error in wrapped error")
	}
}

func TestErrorIs(t *testing.T) {
	err1 := New(ChromeLaunchError, "launch failed")
	err2 := New(ChromeLaunchError, "launch failed")
	err3 := New(NetworkError, "network failed")

	if !err1.Is(err2) {
		t.Error("Expected errors of same type to be equal")
	}

	if err1.Is(err3) {
		t.Error("Expected errors of different types to not be equal")
	}

	// Test with non-ChromeError
	regularErr := errors.New("regular error")
	if err1.Is(regularErr) {
		t.Error("Expected ChromeError to not equal regular error")
	}
}

func BenchmarkErrorCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(ChromeLaunchError, "test error")
	}
}

func BenchmarkErrorWithContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		err := New(ChromeLaunchError, "test error")
		_ = WithContext(err, "test_key", "test_value")
	}
}

func BenchmarkErrorFormatting(b *testing.B) {
	err := WithContext(
		New(ChromeLaunchError, "test error"),
		"test_key", "test_value",
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatError(err)
	}
}
