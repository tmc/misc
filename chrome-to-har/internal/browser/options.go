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
	StabilityConfig   *StabilityConfig
	WaitForStability  bool

	// Proxy settings
	ProxyServer       string // HTTP/HTTPS or SOCKS5 proxy
	ProxyBypassList   string // Comma-separated list of hosts to bypass
	ProxyUsername     string // Proxy authentication username
	ProxyPassword     string // Proxy authentication password

	// Script injection settings
	ScriptBefore []string // Scripts to execute before page load
	ScriptAfter  []string // Scripts to execute after page load
}

// Option is a function that modifies Options
type Option func(*Options) error

// defaultOptions returns the default browser options
func defaultOptions() *Options {
	return &Options{
		Headless:          true,
		Timeout:           180,
		NavigationTimeout: 45,
		StableTimeout:     30,
		UseProfile:        false,
		CookieDomains:     []string{},
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
