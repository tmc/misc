package browser

import (
	"github.com/pkg/errors"
)

// Options controls browser behavior
type Options struct {
	// Browser settings
	Headless      bool
	ChromePath    string
	DebugPort     int
	Timeout       int
	UseProfile    bool
	ProfileName   string
	CookieDomains []string
	Verbose       bool
	ChromeFlags   []string

	// Remote connection settings
	RemoteHost  string
	RemotePort  int
	RemoteTabID string
	UseRemote   bool

	// Navigation settings
	NavigationTimeout int
	WaitNetworkIdle   bool
	WaitSelector      string
	StableTimeout     int

	// Stability detection settings
	StabilityConfig  *StabilityConfig
	WaitForStability bool

	// Proxy settings
	ProxyServer     string // HTTP/HTTPS or SOCKS5 proxy
	ProxyBypassList string // Comma-separated list of hosts to bypass
	ProxyUsername   string // Proxy authentication username
	ProxyPassword   string // Proxy authentication password

	// Script injection settings
	ScriptBefore []string // Scripts to execute before page load
	ScriptAfter  []string // Scripts to execute after page load

	// Blocking settings
	BlockingEnabled       bool     // Enable URL/domain blocking
	BlockingVerbose       bool     // Enable verbose blocking logging
	BlockedURLPatterns    []string // URL patterns to block
	BlockedDomains        []string // Domains to block
	BlockedRegexPatterns  []string // Regex patterns to block
	AllowedURLs           []string // URLs to allow (whitelist)
	AllowedDomains        []string // Domains to allow (whitelist)
	BlockingRuleFile      string   // File containing blocking rules
	BlockCommonAds        bool     // Block common ad domains
	BlockCommonTracking   bool     // Block common tracking domains
	
	// Security settings
	SecurityProfile       string   // Security profile: strict, balanced, permissive
	AllowedRemoteHosts    []string // Allowed remote hosts for connections
	RemoteAuthToken       string   // Authentication token for remote connections
	RemoteUseTLS          bool     // Use TLS for remote connections
	RemoteCACert          string   // Path to CA certificate for remote TLS
	MaxMemoryMB           uint64   // Maximum memory usage in MB
	MaxConcurrentReqs     int      // Maximum concurrent requests
	EnableResourceLimits  bool     // Enable resource limiting
	ScriptValidation      bool     // Enable script validation
	AllowDangerousScripts bool     // Allow potentially dangerous scripts

	// File upload settings
	UploadFiles           []FileUpload // Files to upload
	UploadFormSelector    string       // Form selector for file uploads
	UploadInputSelector   string       // File input selector
	UploadMethod          string       // Upload method: form, ajax, drop
	UploadProgressReport  bool         // Enable upload progress reporting
	UploadChunkSize       int64        // Chunk size for chunked uploads (0 = no chunking)
	UploadMaxFileSize     int64        // Maximum file size allowed (0 = no limit)
	UploadTimeout         int          // Upload timeout in seconds
	UploadRetryAttempts   int          // Number of retry attempts for failed uploads
	UploadValidateTypes   []string     // Allowed file types (empty = all types)
	UploadCompressFiles   bool         // Compress files before upload
}

// FileUpload represents a file to be uploaded
type FileUpload struct {
	Path        string            // Local file path
	FieldName   string            // Form field name (for form uploads)
	FileName    string            // Override filename (optional)
	ContentType string            // Override content type (optional)
	Metadata    map[string]string // Additional metadata
}

// Option is a function that modifies Options
type Option func(*Options) error

// defaultOptions returns the default browser options with secure defaults
func defaultOptions() *Options {
	return &Options{
		Headless:          true,
		Timeout:           180,
		NavigationTimeout: 45,
		StableTimeout:     30,
		UseProfile:        false,
		CookieDomains:     []string{},
		
		// Secure defaults
		SecurityProfile:       "balanced",
		AllowedRemoteHosts:    []string{"localhost", "127.0.0.1"},
		EnableResourceLimits:  true,
		MaxMemoryMB:           1024, // 1GB default
		MaxConcurrentReqs:     10,
		ScriptValidation:      true,
		AllowDangerousScripts: false,
	}
}

// WithHeadless controls whether Chrome runs in headless mode
func WithHeadless(headless bool) Option {
	return func(o *Options) error {
		o.Headless = headless
		return nil
	}
}

// WithChromePath sets custom Chrome executable path
func WithChromePath(path string) Option {
	return func(o *Options) error {
		o.ChromePath = path
		return nil
	}
}

// WithDebugPort sets a custom Chrome DevTools debugging port
func WithDebugPort(port int) Option {
	return func(o *Options) error {
		if port < 0 {
			return errors.New("debug port must be positive")
		}
		o.DebugPort = port
		return nil
	}
}

// WithTimeout sets global timeout in seconds
func WithTimeout(timeout int) Option {
	return func(o *Options) error {
		if timeout <= 0 {
			return errors.New("timeout must be positive")
		}
		o.Timeout = timeout
		return nil
	}
}

// WithNavigationTimeout sets timeout for navigations in seconds
func WithNavigationTimeout(timeout int) Option {
	return func(o *Options) error {
		if timeout <= 0 {
			return errors.New("navigation timeout must be positive")
		}
		o.NavigationTimeout = timeout
		return nil
	}
}

// WithProfile enables profile usage with the specified name
func WithProfile(name string) Option {
	return func(o *Options) error {
		o.UseProfile = true
		o.ProfileName = name
		return nil
	}
}

// WithCookieDomains sets domains to include cookies from
func WithCookieDomains(domains []string) Option {
	return func(o *Options) error {
		o.CookieDomains = domains
		return nil
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(o *Options) error {
		o.Verbose = verbose
		return nil
	}
}

// WithWaitNetworkIdle makes the browser wait for network activity to become idle
func WithWaitNetworkIdle(wait bool) Option {
	return func(o *Options) error {
		o.WaitNetworkIdle = wait
		return nil
	}
}

// WithWaitSelector makes the browser wait for a CSS selector to be visible
func WithWaitSelector(selector string) Option {
	return func(o *Options) error {
		o.WaitSelector = selector
		return nil
	}
}

// WithStableTimeout sets the maximum time to wait for stability in seconds
func WithStableTimeout(timeout int) Option {
	return func(o *Options) error {
		if timeout <= 0 {
			return errors.New("stable timeout must be positive")
		}
		o.StableTimeout = timeout
		return nil
	}
}

// WithChromeFlags adds custom Chrome command line flags
func WithChromeFlags(flags []string) Option {
	return func(o *Options) error {
		o.ChromeFlags = append(o.ChromeFlags, flags...)
		return nil
	}
}

// WithRemoteChrome configures connection to a running Chrome instance
func WithRemoteChrome(host string, port int) Option {
	return func(o *Options) error {
		if port <= 0 {
			return errors.New("remote port must be positive")
		}
		o.UseRemote = true
		o.RemoteHost = host
		o.RemotePort = port
		return nil
	}
}

// WithRemoteTab specifies a specific tab ID to connect to
func WithRemoteTab(tabID string) Option {
	return func(o *Options) error {
		if tabID == "" {
			return errors.New("tab ID cannot be empty")
		}
		o.RemoteTabID = tabID
		return nil
	}
}

// WithWaitForNetworkIdle makes the browser wait for network activity to become idle (compatibility)
func WithWaitForNetworkIdle(wait bool) Option {
	return WithWaitNetworkIdle(wait)
}

// WithRemoteTabConnection specifies connection to a specific tab in remote Chrome
func WithRemoteTabConnection(host string, port int, tabID string) Option {
	return func(o *Options) error {
		if port <= 0 {
			return errors.New("remote port must be positive")
		}
		if tabID == "" {
			return errors.New("tab ID cannot be empty")
		}
		o.UseRemote = true
		o.RemoteHost = host
		o.RemotePort = port
		o.RemoteTabID = tabID
		return nil
	}
}

// WithScriptBefore adds a script to execute before page load
func WithScriptBefore(script string) Option {
	return func(o *Options) error {
		if script == "" {
			return errors.New("script cannot be empty")
		}
		o.ScriptBefore = append(o.ScriptBefore, script)
		return nil
	}
}

// WithScriptAfter adds a script to execute after page load
func WithScriptAfter(script string) Option {
	return func(o *Options) error {
		if script == "" {
			return errors.New("script cannot be empty")
		}
		o.ScriptAfter = append(o.ScriptAfter, script)
		return nil
	}
}

// WithScriptsBefore adds multiple scripts to execute before page load
func WithScriptsBefore(scripts []string) Option {
	return func(o *Options) error {
		for _, script := range scripts {
			if script == "" {
				return errors.New("script cannot be empty")
			}
		}
		o.ScriptBefore = append(o.ScriptBefore, scripts...)
		return nil
	}
}

// WithScriptsAfter adds multiple scripts to execute after page load
func WithScriptsAfter(scripts []string) Option {
	return func(o *Options) error {
		for _, script := range scripts {
			if script == "" {
				return errors.New("script cannot be empty")
			}
		}
		o.ScriptAfter = append(o.ScriptAfter, scripts...)
		return nil
	}
}

// WithProxy configures proxy settings
func WithProxy(proxyServer string) Option {
	return func(o *Options) error {
		if proxyServer == "" {
			return errors.New("proxy server cannot be empty")
		}
		o.ProxyServer = proxyServer
		return nil
	}
}

// WithProxyBypassList sets hosts to bypass proxy
func WithProxyBypassList(bypassList string) Option {
	return func(o *Options) error {
		o.ProxyBypassList = bypassList
		return nil
	}
}

// WithProxyAuth sets proxy authentication credentials
func WithProxyAuth(username, password string) Option {
	return func(o *Options) error {
		if username == "" {
			return errors.New("proxy username cannot be empty")
		}
		o.ProxyUsername = username
		o.ProxyPassword = password
		return nil
	}
}

// WithStabilityConfig sets custom stability detection configuration
func WithStabilityConfig(config *StabilityConfig) Option {
	return func(o *Options) error {
		if config == nil {
			return errors.New("stability config cannot be nil")
		}
		o.StabilityConfig = config
		o.WaitForStability = true
		return nil
	}
}

// WithWaitForStability enables full page stability detection
func WithWaitForStability(wait bool) Option {
	return func(o *Options) error {
		o.WaitForStability = wait
		if wait && o.StabilityConfig == nil {
			o.StabilityConfig = DefaultStabilityConfig()
		}
		return nil
	}
}

// WithStabilityOptions configures stability detection with specific options
func WithStabilityOptions(opts ...StabilityOption) Option {
	return func(o *Options) error {
		if o.StabilityConfig == nil {
			o.StabilityConfig = DefaultStabilityConfig()
		}
		for _, opt := range opts {
			opt(o.StabilityConfig)
		}
		o.WaitForStability = true
		return nil
	}
}

// WithBlocking enables URL/domain blocking
func WithBlocking(enabled bool) Option {
	return func(o *Options) error {
		o.BlockingEnabled = enabled
		return nil
	}
}

// WithBlockingVerbose enables verbose logging for blocking
func WithBlockingVerbose(verbose bool) Option {
	return func(o *Options) error {
		o.BlockingVerbose = verbose
		return nil
	}
}

// WithBlockedURLPatterns sets URL patterns to block
func WithBlockedURLPatterns(patterns []string) Option {
	return func(o *Options) error {
		o.BlockedURLPatterns = append(o.BlockedURLPatterns, patterns...)
		return nil
	}
}

// WithBlockedURLPattern adds a single URL pattern to block
func WithBlockedURLPattern(pattern string) Option {
	return func(o *Options) error {
		if pattern == "" {
			return errors.New("URL pattern cannot be empty")
		}
		o.BlockedURLPatterns = append(o.BlockedURLPatterns, pattern)
		return nil
	}
}

// WithBlockedDomains sets domains to block
func WithBlockedDomains(domains []string) Option {
	return func(o *Options) error {
		o.BlockedDomains = append(o.BlockedDomains, domains...)
		return nil
	}
}

// WithBlockedDomain adds a single domain to block
func WithBlockedDomain(domain string) Option {
	return func(o *Options) error {
		if domain == "" {
			return errors.New("domain cannot be empty")
		}
		o.BlockedDomains = append(o.BlockedDomains, domain)
		return nil
	}
}

// WithBlockedRegexPatterns sets regex patterns to block
func WithBlockedRegexPatterns(patterns []string) Option {
	return func(o *Options) error {
		o.BlockedRegexPatterns = append(o.BlockedRegexPatterns, patterns...)
		return nil
	}
}

// WithBlockedRegexPattern adds a single regex pattern to block
func WithBlockedRegexPattern(pattern string) Option {
	return func(o *Options) error {
		if pattern == "" {
			return errors.New("regex pattern cannot be empty")
		}
		o.BlockedRegexPatterns = append(o.BlockedRegexPatterns, pattern)
		return nil
	}
}

// WithAllowedURLs sets URLs to allow (whitelist)
func WithAllowedURLs(urls []string) Option {
	return func(o *Options) error {
		o.AllowedURLs = append(o.AllowedURLs, urls...)
		return nil
	}
}

// WithAllowedURL adds a single URL to allow
func WithAllowedURL(url string) Option {
	return func(o *Options) error {
		if url == "" {
			return errors.New("URL cannot be empty")
		}
		o.AllowedURLs = append(o.AllowedURLs, url)
		return nil
	}
}

// WithAllowedDomains sets domains to allow (whitelist)
func WithAllowedDomains(domains []string) Option {
	return func(o *Options) error {
		o.AllowedDomains = append(o.AllowedDomains, domains...)
		return nil
	}
}

// WithAllowedDomain adds a single domain to allow
func WithAllowedDomain(domain string) Option {
	return func(o *Options) error {
		if domain == "" {
			return errors.New("domain cannot be empty")
		}
		o.AllowedDomains = append(o.AllowedDomains, domain)
		return nil
	}
}

// WithBlockingRuleFile sets the file containing blocking rules
func WithBlockingRuleFile(filename string) Option {
	return func(o *Options) error {
		if filename == "" {
			return errors.New("rule file name cannot be empty")
		}
		o.BlockingRuleFile = filename
		return nil
	}
}

// WithBlockCommonAds enables blocking of common ad domains
func WithBlockCommonAds(enabled bool) Option {
	return func(o *Options) error {
		o.BlockCommonAds = enabled
		return nil
	}
}

// WithBlockCommonTracking enables blocking of common tracking domains
func WithBlockCommonTracking(enabled bool) Option {
	return func(o *Options) error {
		o.BlockCommonTracking = enabled
		return nil
	}
}

// WithSecurityProfile sets the security profile (strict, balanced, permissive)
func WithSecurityProfile(profile string) Option {
	return func(o *Options) error {
		switch profile {
		case "strict", "balanced", "permissive":
			o.SecurityProfile = profile
		default:
			return errors.New("invalid security profile: must be strict, balanced, or permissive")
		}
		return nil
	}
}

// WithAllowedRemoteHosts sets the allowed remote hosts for connections
func WithAllowedRemoteHosts(hosts []string) Option {
	return func(o *Options) error {
		o.AllowedRemoteHosts = hosts
		return nil
	}
}

// WithRemoteAuthToken sets the authentication token for remote connections
func WithRemoteAuthToken(token string) Option {
	return func(o *Options) error {
		o.RemoteAuthToken = token
		return nil
	}
}

// WithRemoteTLS enables TLS for remote connections
func WithRemoteTLS(enabled bool, caCertPath string) Option {
	return func(o *Options) error {
		o.RemoteUseTLS = enabled
		o.RemoteCACert = caCertPath
		return nil
	}
}

// WithResourceLimits enables resource limiting with specified limits
func WithResourceLimits(maxMemoryMB uint64, maxConcurrentReqs int) Option {
	return func(o *Options) error {
		if maxMemoryMB <= 0 {
			return errors.New("max memory must be positive")
		}
		if maxConcurrentReqs <= 0 {
			return errors.New("max concurrent requests must be positive")
		}
		o.EnableResourceLimits = true
		o.MaxMemoryMB = maxMemoryMB
		o.MaxConcurrentReqs = maxConcurrentReqs
		return nil
	}
}

// WithScriptValidation enables validation of JavaScript scripts
func WithScriptValidation(enabled bool) Option {
	return func(o *Options) error {
		o.ScriptValidation = enabled
		return nil
	}
}

// WithAllowDangerousScripts allows potentially dangerous JavaScript patterns
func WithAllowDangerousScripts(allow bool) Option {
	return func(o *Options) error {
		o.AllowDangerousScripts = allow
		return nil
	}
}
