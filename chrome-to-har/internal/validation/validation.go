// Package validation provides comprehensive input validation for security hardening.
package validation

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Safe filename pattern - alphanumeric, dots, underscores, hyphens
	safeFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	// Safe profile name pattern - alphanumeric, spaces, underscores, hyphens
	safeProfileRegex = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)
	// Safe host pattern - alphanumeric, dots, hyphens
	safeHostRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	// Dangerous JavaScript patterns that should be blocked
	dangerousJSPatterns = []string{
		"eval(",
		"Function(",
		"setTimeout(",
		"setInterval(",
		".innerHTML",
		"document.write",
		"window.location",
		"document.cookie",
		"localStorage",
		"sessionStorage",
		"XMLHttpRequest",
		"fetch(",
		"import(",
		"new Worker(",
		"postMessage(",
		"WebSocket",
		"EventSource",
		"SharedWorker",
		"ServiceWorker",
		"navigator.geolocation",
		"navigator.mediaDevices",
		"navigator.permissions",
		"Notification",
		"indexedDB",
		"crypto.subtle",
		"crypto.getRandomValues",
		"atob(",
		"btoa(",
		"unescape(",
		"decodeURI(",
		"decodeURIComponent(",
	}
)

// ValidateProfileName ensures the profile name is safe for filesystem use
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("profile name too long (max 255 characters)")
	}

	// Check for directory traversal attempts
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("profile name contains invalid path characters")
	}

	// Check for reserved names
	reservedNames := []string{".", "..", "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	upperName := strings.ToUpper(name)
	for _, reserved := range reservedNames {
		if upperName == reserved {
			return fmt.Errorf("profile name is reserved: %s", name)
		}
	}

	if !safeProfileRegex.MatchString(name) {
		return fmt.Errorf("profile name contains invalid characters (only alphanumeric, spaces, underscores, and hyphens allowed)")
	}

	// Check for control characters
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("profile name contains control characters")
		}
	}

	return nil
}

// ValidatePath ensures the path is safe and within allowed directories
func ValidatePath(path string, allowedDirs []string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(absPath)

	// Check for directory traversal - if clean path is different from abs path, it contained traversal
	if cleanPath != absPath {
		return fmt.Errorf("path contains directory traversal")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes")
	}

	// Check if path is within allowed directories
	if len(allowedDirs) > 0 {
		allowed := false
		for _, dir := range allowedDirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			cleanDir := filepath.Clean(absDir)
			if strings.HasPrefix(cleanPath, cleanDir) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path outside allowed directories")
		}
	}

	return nil
}

// ValidateURL ensures the URL is well-formed and uses allowed protocols
func ValidateURL(rawURL string, allowedProtocols []string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Check for control characters and null bytes
	for _, r := range rawURL {
		if unicode.IsControl(r) || r == 0 {
			return fmt.Errorf("URL contains control characters or null bytes")
		}
	}

	// Check for suspicious patterns
	lowerURL := strings.ToLower(rawURL)
	if strings.Contains(lowerURL, "javascript:") {
		return fmt.Errorf("javascript URLs not allowed")
	}
	if strings.Contains(lowerURL, "data:") {
		return fmt.Errorf("data URLs not allowed")
	}
	if strings.Contains(lowerURL, "vbscript:") {
		return fmt.Errorf("vbscript URLs not allowed")
	}
	if strings.Contains(lowerURL, "file:") && !strings.HasPrefix(lowerURL, "file://") {
		return fmt.Errorf("malformed file URLs not allowed")
	}

	// Parse the URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check protocol
	if len(allowedProtocols) > 0 {
		allowed := false
		for _, proto := range allowedProtocols {
			if u.Scheme == proto {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("protocol %s not allowed", u.Scheme)
		}
	}

	// Additional checks for specific protocols
	if u.Scheme == "file" {
		// File URLs should have proper paths
		if u.Path == "" {
			return fmt.Errorf("file URL missing path")
		}
		// Validate the file path
		if err := ValidatePath(u.Path, nil); err != nil {
			return fmt.Errorf("invalid file path in URL: %w", err)
		}
	}

	return nil
}

// ValidateHostname ensures the hostname is safe for network connections
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if len(hostname) > 253 {
		return fmt.Errorf("hostname too long (max 253 characters)")
	}

	// Check for control characters
	for _, r := range hostname {
		if unicode.IsControl(r) {
			return fmt.Errorf("hostname contains control characters")
		}
	}

	// Try to parse as IP address first (including IPv6)
	if net.ParseIP(hostname) != nil {
		return nil // Valid IP address
	}

	// Check basic pattern for hostnames (not IP addresses)
	if !safeHostRegex.MatchString(hostname) {
		return fmt.Errorf("hostname contains invalid characters")
	}

	// Validate as hostname
	if len(hostname) > 0 && (hostname[0] == '-' || hostname[len(hostname)-1] == '-') {
		return fmt.Errorf("hostname cannot start or end with hyphen")
	}

	if strings.Contains(hostname, "..") {
		return fmt.Errorf("hostname contains consecutive dots")
	}

	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("hostname contains empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("hostname label too long (max 63 characters)")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("hostname label cannot start or end with hyphen")
		}
	}

	return nil
}

// ValidatePort ensures the port number is valid
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Check for privileged ports (optional warning)
	if port < 1024 {
		return fmt.Errorf("privileged port %d requires elevated permissions", port)
	}

	return nil
}

// ValidateJavaScript performs basic validation of JavaScript content
func ValidateJavaScript(script string, allowDangerous bool) error {
	if script == "" {
		return fmt.Errorf("script cannot be empty")
	}

	// Check for extremely long scripts
	if len(script) > 1048576 { // 1MB limit
		return fmt.Errorf("script too long (max 1MB)")
	}

	// Check for control characters (except common ones like \n, \t, \r)
	for _, r := range script {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return fmt.Errorf("script contains control characters")
		}
	}

	if !allowDangerous {
		lowerScript := strings.ToLower(script)
		for _, pattern := range dangerousJSPatterns {
			if strings.Contains(lowerScript, strings.ToLower(pattern)) {
				return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
			}
		}
	}

	// Basic syntax validation - check for balanced braces
	openBraces := strings.Count(script, "{")
	closeBraces := strings.Count(script, "}")
	if openBraces != closeBraces {
		return fmt.Errorf("unbalanced braces in script")
	}

	// Check for balanced parentheses
	openParens := strings.Count(script, "(")
	closeParens := strings.Count(script, ")")
	if openParens != closeParens {
		return fmt.Errorf("unbalanced parentheses in script")
	}

	// Check for balanced square brackets
	openBrackets := strings.Count(script, "[")
	closeBrackets := strings.Count(script, "]")
	if openBrackets != closeBrackets {
		return fmt.Errorf("unbalanced square brackets in script")
	}

	return nil
}

// ValidateTimeout ensures timeout values are reasonable
func ValidateTimeout(timeout int) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if timeout > 3600 { // 1 hour max
		return fmt.Errorf("timeout too long (max 3600 seconds)")
	}

	return nil
}

// ValidateUserAgent ensures the user agent string is reasonable
func ValidateUserAgent(userAgent string) error {
	if userAgent == "" {
		return fmt.Errorf("user agent cannot be empty")
	}

	if len(userAgent) > 1024 {
		return fmt.Errorf("user agent too long (max 1024 characters)")
	}

	// Check for control characters
	for _, r := range userAgent {
		if unicode.IsControl(r) && r != '\t' {
			return fmt.Errorf("user agent contains control characters")
		}
	}

	return nil
}

// ValidateHeaders ensures HTTP headers are safe
func ValidateHeaders(headers map[string]string) error {
	if len(headers) > 100 {
		return fmt.Errorf("too many headers (max 100)")
	}

	for name, value := range headers {
		if name == "" {
			return fmt.Errorf("header name cannot be empty")
		}

		if len(name) > 256 {
			return fmt.Errorf("header name too long: %s", name)
		}

		if len(value) > 8192 {
			return fmt.Errorf("header value too long for %s", name)
		}

		// Check for control characters in header name
		for _, r := range name {
			if unicode.IsControl(r) {
				return fmt.Errorf("header name contains control characters: %s", name)
			}
		}

		// Check for control characters in header value (except \t)
		for _, r := range value {
			if unicode.IsControl(r) && r != '\t' {
				return fmt.Errorf("header value contains control characters: %s", name)
			}
		}

		// Check for dangerous headers
		lowerName := strings.ToLower(name)
		if lowerName == "host" || lowerName == "content-length" || lowerName == "transfer-encoding" {
			return fmt.Errorf("dangerous header not allowed: %s", name)
		}
	}

	return nil
}

// SanitizeFilename ensures a filename is safe for filesystem use
func SanitizeFilename(filename string) string {
	if filename == "" {
		return "output"
	}

	// Remove directory components
	filename = filepath.Base(filename)

	// Replace unsafe characters with underscores
	var result strings.Builder
	for _, r := range filename {
		// Only allow ASCII letters, digits, and specific punctuation
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// Ensure it's not empty after sanitization
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		sanitized = "output"
	}

	// Ensure it doesn't start with a dot (hidden file)
	if len(sanitized) > 0 && sanitized[0] == '.' {
		sanitized = "file_" + sanitized
	}

	// Truncate if too long
	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
	}

	return sanitized
}

// ValidateRemoteHosts checks if a host is in the allowed list
func ValidateRemoteHosts(host string, allowedHosts []string) error {
	if err := ValidateHostname(host); err != nil {
		return err
	}

	if len(allowedHosts) == 0 {
		return fmt.Errorf("no remote hosts allowed")
	}

	for _, allowed := range allowedHosts {
		if host == allowed {
			return nil
		}
	}

	return fmt.Errorf("host %s not in allowed list", host)
}

// ValidateProxyURL validates proxy server URLs
func ValidateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return fmt.Errorf("proxy URL cannot be empty")
	}

	// Parse the proxy URL
	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	// Check supported schemes
	switch u.Scheme {
	case "http", "https", "socks5":
		// OK
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", u.Scheme)
	}

	// Validate hostname
	if err := ValidateHostname(u.Hostname()); err != nil {
		return fmt.Errorf("invalid proxy hostname: %w", err)
	}

	// Validate port
	if u.Port() != "" {
		port := 0
		if _, err := fmt.Sscanf(u.Port(), "%d", &port); err != nil {
			return fmt.Errorf("invalid proxy port: %s", u.Port())
		}
		if err := ValidatePort(port); err != nil {
			return fmt.Errorf("invalid proxy port: %w", err)
		}
	}

	return nil
}
