// Package errors provides comprehensive input validation utilities.
package errors

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Validator provides methods for validating different types of input
type Validator struct {
	errors []error
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make([]error, 0),
	}
}

// AddError adds an error to the validator
func (v *Validator) AddError(err error) {
	if err != nil {
		v.errors = append(v.errors, err)
	}
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors
func (v *Validator) Errors() []error {
	return v.errors
}

// FirstError returns the first validation error, or nil if none
func (v *Validator) FirstError() error {
	if len(v.errors) > 0 {
		return v.errors[0]
	}
	return nil
}

// AllErrors returns a combined error with all validation errors
func (v *Validator) AllErrors() error {
	if len(v.errors) == 0 {
		return nil
	}
	
	if len(v.errors) == 1 {
		return v.errors[0]
	}
	
	messages := make([]string, len(v.errors))
	for i, err := range v.errors {
		messages[i] = err.Error()
	}
	
	return New(ValidationError, "multiple validation errors: "+strings.Join(messages, "; "))
}

// ValidateURL validates a URL string
func (v *Validator) ValidateURL(urlStr string, fieldName string) *Validator {
	if urlStr == "" {
		v.AddError(NewValidationError(fieldName, "URL cannot be empty"))
		return v
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		v.AddError(WithContext(
			Wrap(err, InvalidURLError, "invalid URL format"),
			"field", fieldName,
		))
		return v
	}
	
	if parsedURL.Scheme == "" {
		v.AddError(WithContext(
			New(InvalidURLError, "URL must include a scheme (http:// or https://)"),
			"field", fieldName,
		))
		return v
	}
	
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		v.AddError(WithContext(
			New(InvalidURLError, "URL scheme must be http or https"),
			"field", fieldName,
		))
		return v
	}
	
	if parsedURL.Host == "" {
		v.AddError(WithContext(
			New(InvalidURLError, "URL must include a host"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// ValidateHeader validates an HTTP header string
func (v *Validator) ValidateHeader(header string, fieldName string) *Validator {
	if header == "" {
		v.AddError(NewValidationError(fieldName, "header cannot be empty"))
		return v
	}
	
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		v.AddError(WithContext(
			New(InvalidHeaderError, "header must be in format 'name: value'"),
			"field", fieldName,
		))
		return v
	}
	
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	
	if name == "" {
		v.AddError(WithContext(
			New(InvalidHeaderError, "header name cannot be empty"),
			"field", fieldName,
		))
		return v
	}
	
	// Basic header name validation (RFC 7230)
	headerNameRegex := regexp.MustCompile(`^[a-zA-Z0-9!#$%&'*+\-.^_` + "`" + `|~]+$`)
	if !headerNameRegex.MatchString(name) {
		v.AddError(WithContext(
			New(InvalidHeaderError, "header name contains invalid characters"),
			"field", fieldName,
		))
		return v
	}
	
	// Check for common problematic headers
	lowerName := strings.ToLower(name)
	if lowerName == "content-length" || lowerName == "transfer-encoding" {
		v.AddError(WithContext(
			New(InvalidHeaderError, "header is automatically managed and cannot be set manually"),
			"field", fieldName,
		))
		return v
	}
	
	// Value should not contain control characters
	for _, char := range value {
		if char < 32 && char != 9 { // Allow tab (9) but not other control characters
			v.AddError(WithContext(
				New(InvalidHeaderError, "header value contains invalid control characters"),
				"field", fieldName,
			))
			break
		}
	}
	
	return v
}

// ValidateFilePath validates a file path
func (v *Validator) ValidateFilePath(path string, fieldName string, mustExist bool) *Validator {
	if path == "" {
		v.AddError(NewValidationError(fieldName, "file path cannot be empty"))
		return v
	}
	
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		v.AddError(WithContext(
			New(ValidationError, "file path contains path traversal sequences"),
			"field", fieldName,
		))
		return v
	}
	
	// Convert to absolute path for validation
	absPath, err := filepath.Abs(path)
	if err != nil {
		v.AddError(WithContext(
			Wrap(err, ValidationError, "invalid file path"),
			"field", fieldName,
		))
		return v
	}
	
	if mustExist {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			v.AddError(WithContext(
				New(FileNotFoundError, "file does not exist"),
				"field", fieldName,
			))
			return v
		} else if err != nil {
			v.AddError(WithContext(
				FileError("access", absPath, err),
				"field", fieldName,
			))
			return v
		}
	}
	
	return v
}

// ValidateTimeout validates a timeout value
func (v *Validator) ValidateTimeout(timeout int, fieldName string) *Validator {
	if timeout < 0 {
		v.AddError(WithContext(
			New(ValidationError, "timeout cannot be negative"),
			"field", fieldName,
		))
		return v
	}
	
	if timeout == 0 {
		v.AddError(WithContext(
			New(ValidationError, "timeout cannot be zero"),
			"field", fieldName,
		))
		return v
	}
	
	// Check for reasonable upper limit (10 hours)
	if timeout > 36000 {
		v.AddError(WithContext(
			New(ValidationError, "timeout is too large (maximum 36000 seconds)"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// ValidatePort validates a port number
func (v *Validator) ValidatePort(port int, fieldName string) *Validator {
	if port < 0 {
		v.AddError(WithContext(
			New(ValidationError, "port cannot be negative"),
			"field", fieldName,
		))
		return v
	}
	
	if port > 65535 {
		v.AddError(WithContext(
			New(ValidationError, "port cannot be greater than 65535"),
			"field", fieldName,
		))
		return v
	}
	
	// Check for reserved ports (optional warning)
	if port > 0 && port < 1024 {
		LogWarn("Port %d is in the reserved range (1-1023) and may require elevated privileges", port)
	}
	
	return v
}

// ValidateHTTPMethod validates an HTTP method
func (v *Validator) ValidateHTTPMethod(method string, fieldName string) *Validator {
	if method == "" {
		v.AddError(NewValidationError(fieldName, "HTTP method cannot be empty"))
		return v
	}
	
	method = strings.ToUpper(method)
	validMethods := []string{
		"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "TRACE",
	}
	
	valid := false
	for _, validMethod := range validMethods {
		if method == validMethod {
			valid = true
			break
		}
	}
	
	if !valid {
		v.AddError(WithContext(
			New(ValidationError, "invalid HTTP method"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// ValidateJavaScript validates JavaScript code
func (v *Validator) ValidateJavaScript(script string, fieldName string) *Validator {
	if script == "" {
		v.AddError(NewValidationError(fieldName, "JavaScript code cannot be empty"))
		return v
	}
	
	// Basic syntax validation (check for balanced braces, brackets, parentheses)
	braceCount := 0
	parenCount := 0
	bracketCount := 0
	
	for _, char := range script {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
	}
	
	if braceCount != 0 {
		v.AddError(WithContext(
			New(InvalidScriptError, "unbalanced braces in JavaScript"),
			"field", fieldName,
		))
	}
	
	if parenCount != 0 {
		v.AddError(WithContext(
			New(InvalidScriptError, "unbalanced parentheses in JavaScript"),
			"field", fieldName,
		))
	}
	
	if bracketCount != 0 {
		v.AddError(WithContext(
			New(InvalidScriptError, "unbalanced brackets in JavaScript"),
			"field", fieldName,
		))
	}
	
	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"eval(",
		"document.write(",
		"innerHTML =",
		"outerHTML =",
		"location.href =",
		"location.replace(",
		"location.assign(",
	}
	
	scriptLower := strings.ToLower(script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, pattern) {
			LogWarn("JavaScript contains potentially dangerous pattern: %s", pattern)
			break
		}
	}
	
	return v
}

// ValidateOutputFormat validates output format
func (v *Validator) ValidateOutputFormat(format string, fieldName string) *Validator {
	if format == "" {
		v.AddError(NewValidationError(fieldName, "output format cannot be empty"))
		return v
	}
	
	validFormats := []string{"html", "har", "text", "json"}
	valid := false
	for _, validFormat := range validFormats {
		if strings.ToLower(format) == validFormat {
			valid = true
			break
		}
	}
	
	if !valid {
		v.AddError(WithContext(
			New(ValidationError, "invalid output format (must be one of: html, har, text, json)"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// ValidateProxyURL validates a proxy URL
func (v *Validator) ValidateProxyURL(proxyURL string, fieldName string) *Validator {
	if proxyURL == "" {
		v.AddError(NewValidationError(fieldName, "proxy URL cannot be empty"))
		return v
	}
	
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		v.AddError(WithContext(
			Wrap(err, ProxyError, "invalid proxy URL format"),
			"field", fieldName,
		))
		return v
	}
	
	validSchemes := []string{"http", "https", "socks5"}
	valid := false
	for _, scheme := range validSchemes {
		if parsedURL.Scheme == scheme {
			valid = true
			break
		}
	}
	
	if !valid {
		v.AddError(WithContext(
			New(ProxyError, "proxy URL scheme must be http, https, or socks5"),
			"field", fieldName,
		))
		return v
	}
	
	if parsedURL.Host == "" {
		v.AddError(WithContext(
			New(ProxyError, "proxy URL must include a host"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// ValidateCredentials validates username:password format
func (v *Validator) ValidateCredentials(credentials string, fieldName string) *Validator {
	if credentials == "" {
		v.AddError(NewValidationError(fieldName, "credentials cannot be empty"))
		return v
	}
	
	if !strings.Contains(credentials, ":") {
		v.AddError(WithContext(
			New(ValidationError, "credentials must be in format 'username:password'"),
			"field", fieldName,
		))
		return v
	}
	
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		v.AddError(WithContext(
			New(ValidationError, "credentials must be in format 'username:password'"),
			"field", fieldName,
		))
		return v
	}
	
	username := parts[0]
	password := parts[1]
	
	if username == "" {
		v.AddError(WithContext(
			New(ValidationError, "username cannot be empty"),
			"field", fieldName,
		))
		return v
	}
	
	if password == "" {
		v.AddError(WithContext(
			New(ValidationError, "password cannot be empty"),
			"field", fieldName,
		))
		return v
	}
	
	return v
}

// Convenience functions for single field validation

// ValidateURL validates a single URL
func ValidateURL(urlStr string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateURL(urlStr, fieldName)
	return validator.FirstError()
}

// ValidateHeader validates a single header
func ValidateHeader(header string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateHeader(header, fieldName)
	return validator.FirstError()
}

// ValidateFilePath validates a single file path
func ValidateFilePath(path string, fieldName string, mustExist bool) error {
	validator := NewValidator()
	validator.ValidateFilePath(path, fieldName, mustExist)
	return validator.FirstError()
}

// ValidateTimeout validates a single timeout value
func ValidateTimeout(timeout int, fieldName string) error {
	validator := NewValidator()
	validator.ValidateTimeout(timeout, fieldName)
	return validator.FirstError()
}

// ValidatePort validates a single port number
func ValidatePort(port int, fieldName string) error {
	validator := NewValidator()
	validator.ValidatePort(port, fieldName)
	return validator.FirstError()
}

// ValidateHTTPMethod validates a single HTTP method
func ValidateHTTPMethod(method string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateHTTPMethod(method, fieldName)
	return validator.FirstError()
}

// ValidateJavaScript validates a single JavaScript code
func ValidateJavaScript(script string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateJavaScript(script, fieldName)
	return validator.FirstError()
}

// ValidateOutputFormat validates a single output format
func ValidateOutputFormat(format string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateOutputFormat(format, fieldName)
	return validator.FirstError()
}

// ValidateProxyURL validates a single proxy URL
func ValidateProxyURL(proxyURL string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateProxyURL(proxyURL, fieldName)
	return validator.FirstError()
}

// ValidateCredentials validates a single credentials string
func ValidateCredentials(credentials string, fieldName string) error {
	validator := NewValidator()
	validator.ValidateCredentials(credentials, fieldName)
	return validator.FirstError()
}