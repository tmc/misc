# Security Implementation Guide for Chrome-to-HAR

This guide provides practical implementation steps and code examples for addressing the security vulnerabilities identified in the security audit.

## Priority 1: Enable Chrome Sandboxing (CRITICAL)

### Current Issue
Chrome is launched without sandboxing, exposing the system to potential exploits.

### Implementation Steps

1. **Update browser launch options in `internal/browser/browser.go`**:

```go
// Add these security-focused Chrome flags
securityFlags := []chromedp.ExecAllocatorOption{
    // Enable sandboxing (remove any disable flags)
    chromedp.Flag("enable-sandbox", true),
    // Enable site isolation
    chromedp.Flag("enable-features", "SitePerProcess,NetworkServiceSandbox"),
    // Enable strict site isolation
    chromedp.Flag("site-per-process", true),
    // Block new web contents (popups)
    chromedp.Flag("block-new-web-contents", true),
    // Enable web security
    chromedp.Flag("disable-web-security", false),
    // Keep GPU sandboxing enabled
    chromedp.Flag("gpu-sandbox-failures-fatal", true),
}
```

2. **Create security profiles**:

```go
type SecurityProfile string

const (
    SecurityProfileStrict   SecurityProfile = "strict"
    SecurityProfileBalanced SecurityProfile = "balanced"
    SecurityProfilePermissive SecurityProfile = "permissive"
)

func getSecurityFlags(profile SecurityProfile) []chromedp.ExecAllocatorOption {
    baseFlags := []chromedp.ExecAllocatorOption{
        chromedp.NoFirstRun,
        chromedp.NoDefaultBrowserCheck,
    }
    
    switch profile {
    case SecurityProfileStrict:
        return append(baseFlags,
            chromedp.Flag("enable-sandbox", true),
            chromedp.Flag("enable-features", "SitePerProcess,NetworkServiceSandbox"),
            chromedp.Flag("block-new-web-contents", true),
            chromedp.Flag("disable-plugins", true),
            chromedp.Flag("disable-java", true),
            chromedp.Flag("disable-3d-apis", true),
        )
    case SecurityProfileBalanced:
        return append(baseFlags,
            chromedp.Flag("enable-sandbox", true),
            chromedp.Flag("enable-features", "SitePerProcess"),
        )
    case SecurityProfilePermissive:
        return append(baseFlags,
            chromedp.Flag("disable-web-security", true), // Only for testing!
        )
    default:
        return getSecurityFlags(SecurityProfileBalanced)
    }
}
```

3. **Add security warnings**:

```go
func (b *Browser) Launch(ctx context.Context) error {
    if b.opts.SecurityProfile == SecurityProfilePermissive {
        log.Println("WARNING: Running with permissive security settings. This should only be used for testing!")
    }
    
    // Rest of launch code...
}
```

## Priority 2: Input Validation (CRITICAL)

### Current Issue
User inputs are not properly validated, allowing path traversal and injection attacks.

### Implementation Steps

1. **Create validation package** `internal/validation/validation.go`:

```go
package validation

import (
    "fmt"
    "net/url"
    "path/filepath"
    "regexp"
    "strings"
)

var (
    // Safe filename pattern
    safeFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
    // Safe profile name pattern
    safeProfileRegex = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)
    // Dangerous JavaScript patterns
    dangerousJSPatterns = []string{
        "eval(",
        "Function(",
        "setTimeout(",
        "setInterval(",
        ".innerHTML",
        "document.write",
        "window.location",
        "document.cookie",
    }
)

// ValidatePath ensures the path is safe and within allowed directories
func ValidatePath(path string, allowedDirs []string) error {
    // Resolve to absolute path
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("invalid path: %w", err)
    }
    
    // Check for directory traversal
    cleaned := filepath.Clean(absPath)
    if strings.Contains(cleaned, "..") {
        return fmt.Errorf("path traversal detected")
    }
    
    // Check if path is within allowed directories
    allowed := false
    for _, dir := range allowedDirs {
        absDir, _ := filepath.Abs(dir)
        if strings.HasPrefix(cleaned, absDir) {
            allowed = true
            break
        }
    }
    
    if !allowed && len(allowedDirs) > 0 {
        return fmt.Errorf("path outside allowed directories")
    }
    
    return nil
}

// ValidateURL ensures the URL is well-formed and uses allowed protocols
func ValidateURL(rawURL string, allowedProtocols []string) error {
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
    
    // Check for suspicious patterns
    if strings.Contains(strings.ToLower(rawURL), "javascript:") {
        return fmt.Errorf("javascript URLs not allowed")
    }
    
    return nil
}

// ValidateProfileName ensures the profile name is safe
func ValidateProfileName(name string) error {
    if name == "" {
        return fmt.Errorf("profile name cannot be empty")
    }
    
    if len(name) > 255 {
        return fmt.Errorf("profile name too long")
    }
    
    if !safeProfileRegex.MatchString(name) {
        return fmt.Errorf("profile name contains invalid characters")
    }
    
    return nil
}

// ValidateJavaScript performs basic validation of JavaScript content
func ValidateJavaScript(script string, allowDangerous bool) error {
    if script == "" {
        return fmt.Errorf("script cannot be empty")
    }
    
    if !allowDangerous {
        lowerScript := strings.ToLower(script)
        for _, pattern := range dangerousJSPatterns {
            if strings.Contains(lowerScript, strings.ToLower(pattern)) {
                return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
            }
        }
    }
    
    // Basic syntax validation
    openBraces := strings.Count(script, "{")
    closeBraces := strings.Count(script, "}")
    if openBraces != closeBraces {
        return fmt.Errorf("unbalanced braces in script")
    }
    
    return nil
}

// SanitizeFilename ensures a filename is safe for filesystem use
func SanitizeFilename(filename string) string {
    // Remove directory components
    filename = filepath.Base(filename)
    
    // Replace unsafe characters
    safe := safeFilenameRegex.ReplaceAllString(filename, "_")
    
    // Ensure it's not empty
    if safe == "" {
        safe = "output"
    }
    
    return safe
}
```

2. **Update main.go to use validation**:

```go
import "github.com/tmc/misc/chrome-to-har/internal/validation"

func run(ctx context.Context, pm chromeprofiles.ProfileManager, opts options) error {
    // Validate profile name
    if opts.profileDir != "" {
        if err := validation.ValidateProfileName(opts.profileDir); err != nil {
            return errors.Wrap(err, "invalid profile name")
        }
    }
    
    // Validate output file
    if opts.outputFile != "" {
        allowedDirs := []string{
            ".", // Current directory
            os.TempDir(),
            filepath.Join(os.Getenv("HOME"), "Downloads"),
        }
        if err := validation.ValidatePath(opts.outputFile, allowedDirs); err != nil {
            return errors.Wrap(err, "invalid output file path")
        }
    }
    
    // Validate URL
    if opts.startURL != "" {
        allowedProtocols := []string{"http", "https", "file"}
        if err := validation.ValidateURL(opts.startURL, allowedProtocols); err != nil {
            return errors.Wrap(err, "invalid URL")
        }
    }
    
    // Continue with validated inputs...
}
```

## Priority 3: Secure Remote Connections (HIGH)

### Current Issue
Remote Chrome connections lack authentication and encryption.

### Implementation Steps

1. **Create secure remote connection module** `internal/browser/secure_remote.go`:

```go
package browser

import (
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "net"
    "net/http"
    "time"
)

type SecureRemoteConfig struct {
    Host          string
    Port          int
    AuthToken     string
    UseTLS        bool
    CACert        string
    AllowedHosts  []string
    Timeout       time.Duration
}

// ValidateRemoteHost checks if the host is allowed
func (c *SecureRemoteConfig) ValidateRemoteHost() error {
    // Check if host is in allowed list
    if len(c.AllowedHosts) > 0 {
        allowed := false
        for _, h := range c.AllowedHosts {
            if h == c.Host {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("host %s not in allowed list", c.Host)
        }
    }
    
    // Validate host format
    if net.ParseIP(c.Host) == nil {
        // Not an IP, check if valid hostname
        if _, err := net.LookupHost(c.Host); err != nil {
            return fmt.Errorf("invalid hostname: %w", err)
        }
    }
    
    // Check for local addresses if not explicitly allowed
    if c.Host != "localhost" && c.Host != "127.0.0.1" {
        ip := net.ParseIP(c.Host)
        if ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
            return fmt.Errorf("private/loopback addresses not allowed")
        }
    }
    
    return nil
}

// GetSecureRemoteDebuggingInfo connects securely to remote Chrome
func GetSecureRemoteDebuggingInfo(config SecureRemoteConfig) (*RemoteDebuggingInfo, error) {
    if err := config.ValidateRemoteHost(); err != nil {
        return nil, err
    }
    
    protocol := "http"
    if config.UseTLS {
        protocol = "https"
    }
    
    url := fmt.Sprintf("%s://%s:%d/json/version", protocol, config.Host, config.Port)
    
    client := &http.Client{
        Timeout: config.Timeout,
    }
    
    if config.UseTLS {
        tlsConfig := &tls.Config{
            MinVersion: tls.VersionTLS12,
        }
        
        if config.CACert != "" {
            caCert, err := os.ReadFile(config.CACert)
            if err != nil {
                return nil, fmt.Errorf("reading CA cert: %w", err)
            }
            
            caCertPool := x509.NewCertPool()
            caCertPool.AppendCertsFromPEM(caCert)
            tlsConfig.RootCAs = caCertPool
        }
        
        client.Transport = &http.Transport{
            TLSClientConfig: tlsConfig,
        }
    }
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    // Add authentication
    if config.AuthToken != "" {
        req.Header.Set("Authorization", "Bearer "+config.AuthToken)
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("connecting to remote Chrome: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("authentication failed")
    }
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    
    var info RemoteDebuggingInfo
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
        return nil, fmt.Errorf("parsing response: %w", err)
    }
    
    return &info, nil
}
```

2. **Update browser options for secure remote**:

```go
// Add to Options struct
type Options struct {
    // ... existing fields ...
    
    // Secure remote settings
    RemoteAuthToken  string
    RemoteUseTLS     bool
    RemoteCACert     string
    AllowedRemoteHosts []string
}

// Add secure remote option
func WithSecureRemote(host string, port int, authToken string, useTLS bool) Option {
    return func(o *Options) error {
        o.UseRemote = true
        o.RemoteHost = host
        o.RemotePort = port
        o.RemoteAuthToken = authToken
        o.RemoteUseTLS = useTLS
        o.AllowedRemoteHosts = []string{"localhost", "127.0.0.1"}
        return nil
    }
}
```

## Priority 4: Secure File Operations (HIGH)

### Current Issue
File operations lack proper security controls and validation.

### Implementation Steps

1. **Create secure file operations module** `internal/secureio/secureio.go`:

```go
package secureio

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "syscall"
)

const (
    MaxFileSize = 100 * 1024 * 1024 // 100MB
    SecurePerms = 0600              // Owner read/write only
    SecureDirPerms = 0700           // Owner read/write/execute only
)

// CreateSecureTempDir creates a temporary directory with secure permissions
func CreateSecureTempDir(prefix string) (string, error) {
    // Generate random suffix
    randomBytes := make([]byte, 16)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", fmt.Errorf("generating random bytes: %w", err)
    }
    
    dirName := fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(randomBytes))
    tempDir := filepath.Join(os.TempDir(), dirName)
    
    // Create directory with secure permissions
    if err := os.MkdirAll(tempDir, SecureDirPerms); err != nil {
        return "", fmt.Errorf("creating directory: %w", err)
    }
    
    // Double-check permissions
    if err := os.Chmod(tempDir, SecureDirPerms); err != nil {
        os.RemoveAll(tempDir)
        return "", fmt.Errorf("setting permissions: %w", err)
    }
    
    return tempDir, nil
}

// SecureCopyFile copies a file with size limits and permission checks
func SecureCopyFile(src, dst string, maxSize int64) error {
    // Check source file
    srcInfo, err := os.Stat(src)
    if err != nil {
        return fmt.Errorf("stating source file: %w", err)
    }
    
    // Check file size
    if maxSize > 0 && srcInfo.Size() > maxSize {
        return fmt.Errorf("file too large: %d bytes (max: %d)", srcInfo.Size(), maxSize)
    }
    
    // Open source file
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("opening source file: %w", err)
    }
    defer srcFile.Close()
    
    // Create destination file with secure permissions
    dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, SecurePerms)
    if err != nil {
        return fmt.Errorf("creating destination file: %w", err)
    }
    defer dstFile.Close()
    
    // Copy with size limit
    written, err := io.CopyN(dstFile, srcFile, maxSize)
    if err != nil && err != io.EOF {
        os.Remove(dst)
        return fmt.Errorf("copying file: %w", err)
    }
    
    if written == maxSize {
        // Check if there's more data
        buf := make([]byte, 1)
        if n, _ := srcFile.Read(buf); n > 0 {
            os.Remove(dst)
            return fmt.Errorf("file exceeds size limit")
        }
    }
    
    return nil
}

// SecureWriteFile writes data to a file with secure permissions
func SecureWriteFile(filename string, data []byte) error {
    // Create temporary file in same directory
    dir := filepath.Dir(filename)
    base := filepath.Base(filename)
    
    tempFile, err := os.CreateTemp(dir, base+".tmp")
    if err != nil {
        return fmt.Errorf("creating temp file: %w", err)
    }
    tempName := tempFile.Name()
    
    // Write data
    if _, err := tempFile.Write(data); err != nil {
        tempFile.Close()
        os.Remove(tempName)
        return fmt.Errorf("writing data: %w", err)
    }
    
    // Set secure permissions
    if err := tempFile.Chmod(SecurePerms); err != nil {
        tempFile.Close()
        os.Remove(tempName)
        return fmt.Errorf("setting permissions: %w", err)
    }
    
    // Close and rename atomically
    if err := tempFile.Close(); err != nil {
        os.Remove(tempName)
        return fmt.Errorf("closing file: %w", err)
    }
    
    if err := os.Rename(tempName, filename); err != nil {
        os.Remove(tempName)
        return fmt.Errorf("renaming file: %w", err)
    }
    
    return nil
}

// LockFile provides advisory locking for a file
type LockFile struct {
    file *os.File
}

// NewLockFile creates a new lock file
func NewLockFile(path string) (*LockFile, error) {
    file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, SecurePerms)
    if err != nil {
        return nil, err
    }
    
    // Try to acquire exclusive lock
    if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
        file.Close()
        return nil, fmt.Errorf("acquiring lock: %w", err)
    }
    
    return &LockFile{file: file}, nil
}

// Unlock releases the lock
func (l *LockFile) Unlock() error {
    if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
        return err
    }
    return l.file.Close()
}
```

2. **Update profile manager to use secure operations**:

```go
import "github.com/tmc/misc/chrome-to-har/internal/secureio"

func (pm *profileManager) SetupWorkdir() error {
    dir, err := secureio.CreateSecureTempDir("chrome-to-har")
    if err != nil {
        return errors.Wrap(err, "creating secure temp directory")
    }
    pm.workDir = dir
    pm.logf("Created secure temporary directory: %s", dir)
    return nil
}

func (pm *profileManager) CopyProfile(name string, cookieDomains []string) error {
    // ... validation code ...
    
    // Use secure copy for sensitive files
    files := []struct {
        name string
        maxSize int64
    }{
        {"Cookies", 50 * 1024 * 1024}, // 50MB limit
        {"Login Data", 10 * 1024 * 1024}, // 10MB limit
        {"Web Data", 20 * 1024 * 1024}, // 20MB limit
    }
    
    for _, f := range files {
        src := filepath.Join(srcDir, f.name)
        dst := filepath.Join(dstDir, f.name)
        
        if err := secureio.SecureCopyFile(src, dst, f.maxSize); err != nil {
            if !os.IsNotExist(err) {
                pm.logf("Warning: failed to copy %s: %v", f.name, err)
            }
        }
    }
    
    // ... rest of copy logic ...
}
```

## Priority 5: Resource Limits and DoS Prevention (MEDIUM)

### Implementation Steps

1. **Create resource limiter** `internal/limits/limits.go`:

```go
package limits

import (
    "context"
    "fmt"
    "runtime"
    "sync"
    "syscall"
    "time"
)

type ResourceLimiter struct {
    maxMemory      uint64
    maxGoroutines  int
    maxConcurrent  int
    activeRequests int
    mu             sync.Mutex
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(maxMemoryMB uint64, maxGoroutines, maxConcurrent int) *ResourceLimiter {
    return &ResourceLimiter{
        maxMemory:     maxMemoryMB * 1024 * 1024,
        maxGoroutines: maxGoroutines,
        maxConcurrent: maxConcurrent,
    }
}

// CheckMemory ensures memory usage is within limits
func (rl *ResourceLimiter) CheckMemory() error {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    if m.Alloc > rl.maxMemory {
        return fmt.Errorf("memory limit exceeded: %d MB (max: %d MB)", 
            m.Alloc/1024/1024, rl.maxMemory/1024/1024)
    }
    
    return nil
}

// AcquireConcurrent gets a slot for concurrent execution
func (rl *ResourceLimiter) AcquireConcurrent(ctx context.Context) error {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    if rl.activeRequests >= rl.maxConcurrent {
        return fmt.Errorf("concurrent request limit reached: %d", rl.maxConcurrent)
    }
    
    rl.activeRequests++
    return nil
}

// ReleaseConcurrent releases a concurrent execution slot
func (rl *ResourceLimiter) ReleaseConcurrent() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    if rl.activeRequests > 0 {
        rl.activeRequests--
    }
}

// SetSystemLimits sets OS-level resource limits
func SetSystemLimits(maxMemoryMB uint64, maxProcesses uint64) error {
    // Set memory limit
    memLimit := &syscall.Rlimit{
        Cur: maxMemoryMB * 1024 * 1024,
        Max: maxMemoryMB * 1024 * 1024,
    }
    
    if err := syscall.Setrlimit(syscall.RLIMIT_AS, memLimit); err != nil {
        return fmt.Errorf("setting memory limit: %w", err)
    }
    
    // Set process limit
    procLimit := &syscall.Rlimit{
        Cur: maxProcesses,
        Max: maxProcesses,
    }
    
    if err := syscall.Setrlimit(syscall.RLIMIT_NPROC, procLimit); err != nil {
        return fmt.Errorf("setting process limit: %w", err)
    }
    
    return nil
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
    rate     int
    interval time.Duration
    tokens   chan struct{}
    stop     chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
    rl := &RateLimiter{
        rate:     rate,
        interval: interval,
        tokens:   make(chan struct{}, rate),
        stop:     make(chan struct{}),
    }
    
    // Fill initial tokens
    for i := 0; i < rate; i++ {
        rl.tokens <- struct{}{}
    }
    
    // Start token refill goroutine
    go rl.refill()
    
    return rl
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
    select {
    case <-rl.tokens:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
    close(rl.stop)
}

func (rl *RateLimiter) refill() {
    ticker := time.NewTicker(rl.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Try to add tokens up to the rate limit
            for i := 0; i < rl.rate; i++ {
                select {
                case rl.tokens <- struct{}{}:
                default:
                    // Channel full, skip
                }
            }
        case <-rl.stop:
            return
        }
    }
}
```

2. **Integrate resource limits into browser**:

```go
// Add to browser options
type Options struct {
    // ... existing fields ...
    
    // Resource limits
    MaxMemoryMB    uint64
    MaxConcurrent  int
    RateLimit      int
    RateLimitInterval time.Duration
}

// In browser Launch method
func (b *Browser) Launch(ctx context.Context) error {
    // Set system limits if running as root (not recommended)
    if os.Geteuid() == 0 && b.opts.MaxMemoryMB > 0 {
        if err := limits.SetSystemLimits(b.opts.MaxMemoryMB, 10); err != nil {
            log.Printf("Warning: failed to set system limits: %v", err)
        }
    }
    
    // Create resource limiter
    b.limiter = limits.NewResourceLimiter(
        b.opts.MaxMemoryMB,
        1000, // max goroutines
        b.opts.MaxConcurrent,
    )
    
    // Acquire concurrent slot
    if err := b.limiter.AcquireConcurrent(ctx); err != nil {
        return err
    }
    defer b.limiter.ReleaseConcurrent()
    
    // Rest of launch logic...
}
```

## Security Testing Script

Create `test_security.sh`:

```bash
#!/bin/bash

echo "Chrome-to-HAR Security Test Suite"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
PASSED=0
FAILED=0

# Test function
test_security() {
    local test_name=$1
    local command=$2
    local expected_fail=$3
    
    echo -n "Testing: $test_name... "
    
    if eval "$command" 2>/dev/null; then
        if [ "$expected_fail" = "true" ]; then
            echo -e "${RED}FAILED${NC} (command succeeded when it should have failed)"
            ((FAILED++))
        else
            echo -e "${GREEN}PASSED${NC}"
            ((PASSED++))
        fi
    else
        if [ "$expected_fail" = "true" ]; then
            echo -e "${GREEN}PASSED${NC} (command failed as expected)"
            ((PASSED++))
        else
            echo -e "${RED}FAILED${NC} (command failed when it should have succeeded)"
            ((FAILED++))
        fi
    fi
}

echo ""
echo "1. Input Validation Tests"
echo "-------------------------"

test_security "Path traversal in output" \
    "./chrome-to-har -output '../../../etc/passwd' -url https://example.com" \
    "true"

test_security "Command injection in profile" \
    "./chrome-to-har -profile \"'; rm -rf /tmp/test; echo '\" -url https://example.com" \
    "true"

test_security "Invalid URL protocol" \
    "./chrome-to-har -url 'javascript:alert(1)'" \
    "true"

test_security "Valid HTTPS URL" \
    "./chrome-to-har -url 'https://example.com' -headless -timeout 10" \
    "false"

echo ""
echo "2. Remote Connection Tests"
echo "--------------------------"

test_security "Unauthorized remote connection" \
    "./chrome-to-har -remote-host 'evil.com' -remote-port 9222" \
    "true"

test_security "Local remote connection" \
    "./chrome-to-har -remote-host 'localhost' -remote-port 9222" \
    "false"

echo ""
echo "3. Resource Limit Tests"
echo "-----------------------"

test_security "Memory limit enforcement" \
    "./chrome-to-har -url 'https://example.com' -max-memory 100" \
    "false"

echo ""
echo "4. File Security Tests"
echo "----------------------"

test_security "Secure temp directory creation" \
    "./chrome-to-har -url 'https://example.com' -profile 'Default' && ls -la /tmp/chrome-to-har-* | grep 'drwx------'" \
    "false"

echo ""
echo "Summary"
echo "-------"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All security tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some security tests failed!${NC}"
    exit 1
fi
```

## Deployment Security Checklist

### Pre-deployment
- [ ] Enable all security flags in production build
- [ ] Remove or disable debug/verbose logging
- [ ] Set up proper file permissions
- [ ] Configure allowed remote hosts
- [ ] Set resource limits
- [ ] Enable rate limiting
- [ ] Configure secure temp directory
- [ ] Review and update allowed URL protocols
- [ ] Set up monitoring and alerting

### Runtime Security
- [ ] Run with minimal privileges (non-root)
- [ ] Use container isolation (Docker/Kubernetes)
- [ ] Enable AppArmor/SELinux profiles
- [ ] Monitor resource usage
- [ ] Log security events
- [ ] Implement log rotation
- [ ] Set up intrusion detection
- [ ] Regular security updates

### Monitoring
- [ ] Track failed authentication attempts
- [ ] Monitor resource usage spikes
- [ ] Alert on suspicious file access
- [ ] Log all remote connections
- [ ] Track JavaScript execution
- [ ] Monitor network requests
- [ ] Set up anomaly detection

## Conclusion

This implementation guide provides concrete steps to address the security vulnerabilities identified in the audit. Implement these changes incrementally, testing thoroughly at each stage. Priority should be given to enabling Chrome sandboxing and implementing input validation, as these address the most critical vulnerabilities.