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

	// Navigation settings
	NavigationTimeout int
	WaitNetworkIdle   bool
	WaitSelector      string
	StableTimeout     int
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
