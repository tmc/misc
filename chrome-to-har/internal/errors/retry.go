// Package errors provides retry mechanisms for handling transient errors.
package errors

import (
	"context"
	"log"
	"time"
)

// RetryConfig defines the configuration for retry attempts
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Backoff     BackoffStrategy
	Verbose     bool
}

// BackoffStrategy defines how retry delays are calculated
type BackoffStrategy int

const (
	// LinearBackoff increases delay linearly
	LinearBackoff BackoffStrategy = iota
	// ExponentialBackoff doubles delay each time
	ExponentialBackoff
	// ConstantBackoff uses the same delay every time
	ConstantBackoff
)

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Backoff:     ExponentialBackoff,
		Verbose:     false,
	}
}

// RetryFunc is a function that can be retried
type RetryFunc func() error

// Retry executes a function with retry logic for retryable errors
func Retry(ctx context.Context, config *RetryConfig, fn RetryFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if the error is retryable
		if !IsRetryable(err) {
			if config.Verbose {
				log.Printf("Error is not retryable, giving up: %v", err)
			}
			return err
		}

		// Don't retry on the last attempt
		if attempt == config.MaxAttempts {
			if config.Verbose {
				log.Printf("Max attempts reached (%d), giving up: %v", config.MaxAttempts, err)
			}
			break
		}

		// Calculate delay for next attempt
		delay := calculateDelay(config, attempt)

		if config.Verbose {
			log.Printf("Attempt %d failed with retryable error: %v", attempt, err)
			log.Printf("Retrying in %v...", delay)
		}

		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return WithContext(
				Wrap(ctx.Err(), CancelError, "retry cancelled due to context"),
				"attempts", attempt,
			)
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// RetryWithResult executes a function with retry logic and returns a result
func RetryWithResult[T any](ctx context.Context, config *RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	if config == nil {
		config = DefaultRetryConfig()
	}

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if the error is retryable
		if !IsRetryable(err) {
			if config.Verbose {
				log.Printf("Error is not retryable, giving up: %v", err)
			}
			return result, err
		}

		// Don't retry on the last attempt
		if attempt == config.MaxAttempts {
			if config.Verbose {
				log.Printf("Max attempts reached (%d), giving up: %v", config.MaxAttempts, err)
			}
			break
		}

		// Calculate delay for next attempt
		delay := calculateDelay(config, attempt)

		if config.Verbose {
			log.Printf("Attempt %d failed with retryable error: %v", attempt, err)
			log.Printf("Retrying in %v...", delay)
		}

		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return result, WithContext(
				Wrap(ctx.Err(), CancelError, "retry cancelled due to context"),
				"attempts", attempt,
			)
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return result, lastErr
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(config *RetryConfig, attempt int) time.Duration {
	var delay time.Duration

	switch config.Backoff {
	case LinearBackoff:
		delay = config.BaseDelay * time.Duration(attempt)
	case ExponentialBackoff:
		delay = config.BaseDelay * time.Duration(1<<uint(attempt-1))
	case ConstantBackoff:
		delay = config.BaseDelay
	default:
		delay = config.BaseDelay
	}

	// Cap at max delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// WithRetry wraps a function to automatically retry on retryable errors
func WithRetry(config *RetryConfig, fn RetryFunc) RetryFunc {
	return func() error {
		return Retry(context.Background(), config, fn)
	}
}

// WithContextRetry wraps a function to automatically retry on retryable errors with context
func WithContextRetry(ctx context.Context, config *RetryConfig, fn RetryFunc) RetryFunc {
	return func() error {
		return Retry(ctx, config, fn)
	}
}

// QuickRetry is a convenience function for simple retry with default config
func QuickRetry(fn RetryFunc) error {
	return Retry(context.Background(), DefaultRetryConfig(), fn)
}

// QuickRetryWithContext is a convenience function for simple retry with context
func QuickRetryWithContext(ctx context.Context, fn RetryFunc) error {
	return Retry(ctx, DefaultRetryConfig(), fn)
}

// IsTransientError checks if an error is likely to be transient
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that are typically transient
	if chromeErr, ok := err.(*ChromeError); ok {
		switch chromeErr.Type {
		case ChromeTimeoutError, NetworkError, NetworkIdleError:
			return true
		case ChromeConnectionError:
			// Connection errors might be transient
			return true
		default:
			return false
		}
	}

	// Check for common transient error patterns in the error message
	errStr := err.Error()
	transientPatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
		"network unreachable",
		"host unreachable",
	}

	for _, pattern := range transientPatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				someMatch(s, substr)))
}

// someMatch is a simple substring search helper
func someMatch(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation struct {
	Name      string
	Operation RetryFunc
	Config    *RetryConfig
	OnRetry   func(attempt int, err error)
	OnSuccess func(attempt int)
	OnFailure func(err error)
}

// Execute runs the retryable operation with callbacks
func (ro *RetryableOperation) Execute(ctx context.Context) error {
	config := ro.Config
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := ro.Operation()
		if err == nil {
			if ro.OnSuccess != nil {
				ro.OnSuccess(attempt)
			}
			return nil
		}

		lastErr = err

		// Check if the error is retryable
		if !IsRetryable(err) {
			if ro.OnFailure != nil {
				ro.OnFailure(err)
			}
			return err
		}

		// Don't retry on the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Call retry callback
		if ro.OnRetry != nil {
			ro.OnRetry(attempt, err)
		}

		// Calculate delay for next attempt
		delay := calculateDelay(config, attempt)

		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return WithContext(
				Wrap(ctx.Err(), CancelError, "retry cancelled due to context"),
				"attempts", attempt,
			)
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	if ro.OnFailure != nil {
		ro.OnFailure(lastErr)
	}

	return lastErr
}
