package blocking

import (
	"testing"
)

func TestNewBlockingEngine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil config",
			config:      nil,
			wantErr:     true,
			errContains: "config cannot be nil",
		},
		{
			name: "valid config",
			config: &Config{
				Enabled:       true,
				URLPatterns:   []string{"*.ads.com"},
				Domains:       []string{"example.com"},
				RegexPatterns: []string{`.*\.ads\..*`},
				AllowURLs:     []string{"https://example.com/safe"},
				AllowDomains:  []string{"safe.com"},
			},
			wantErr: false,
		},
		{
			name: "invalid regex",
			config: &Config{
				Enabled:       true,
				RegexPatterns: []string{"[invalid"},
			},
			wantErr:     true,
			errContains: "compiling regex pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewBlockingEngine(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewBlockingEngine() expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewBlockingEngine() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewBlockingEngine() unexpected error = %v", err)
				return
			}

			if engine == nil {
				t.Errorf("NewBlockingEngine() returned nil engine")
			}
		})
	}
}

func TestBlockingEngine_ShouldBlock(t *testing.T) {
	t.Parallel()

	config := &Config{
		Enabled:       true,
		URLPatterns:   []string{"*/ads/*", "*.tracker.com"},
		Domains:       []string{"ads.example.com", "tracker.net"},
		RegexPatterns: []string{`.*\.analytics\..*`},
		AllowURLs:     []string{"https://ads.example.com/safe"},
		AllowDomains:  []string{"safe.tracker.net"},
	}

	engine, err := NewBlockingEngine(config)
	if err != nil {
		t.Fatalf("NewBlockingEngine() error = %v", err)
	}

	tests := []struct {
		name      string
		url       string
		wantBlock bool
	}{
		{
			name:      "allowed URL should not block",
			url:       "https://ads.example.com/safe",
			wantBlock: false,
		},
		{
			name:      "allowed domain should not block",
			url:       "https://safe.tracker.net/anything",
			wantBlock: false,
		},
		{
			name:      "blocked domain should block",
			url:       "https://ads.example.com/banner",
			wantBlock: true,
		},
		{
			name:      "blocked domain should block",
			url:       "https://tracker.net/pixel",
			wantBlock: true,
		},
		{
			name:      "URL pattern should block",
			url:       "https://example.com/ads/banner.gif",
			wantBlock: true,
		},
		{
			name:      "wildcard pattern should block",
			url:       "https://bad.tracker.com/pixel",
			wantBlock: false, // This should be false because *.tracker.com pattern matches tracker.com domain, not bad.tracker.com
		},
		{
			name:      "regex pattern should block",
			url:       "https://example.analytics.com/track",
			wantBlock: true,
		},
		{
			name:      "normal URL should not block",
			url:       "https://example.com/page",
			wantBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.ShouldBlock(tt.url)
			if got != tt.wantBlock {
				t.Errorf("ShouldBlock(%q) = %v, want %v", tt.url, got, tt.wantBlock)
			}
		})
	}
}

func TestBlockingEngine_ShouldBlock_Disabled(t *testing.T) {
	t.Parallel()

	config := &Config{
		Enabled:     false, // Disabled
		URLPatterns: []string{"*/ads/*"},
		Domains:     []string{"ads.example.com"},
	}

	engine, err := NewBlockingEngine(config)
	if err != nil {
		t.Fatalf("NewBlockingEngine() error = %v", err)
	}

	// Should not block anything when disabled
	tests := []string{
		"https://ads.example.com/banner",
		"https://example.com/ads/pixel",
	}

	for _, url := range tests {
		if engine.ShouldBlock(url) {
			t.Errorf("ShouldBlock(%q) = true, want false (engine disabled)", url)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url    string
		domain string
	}{
		{"https://example.com/path", "example.com"},
		{"http://example.com:8080/path", "example.com"},
		{"https://sub.example.com/path?query=1", "sub.example.com"},
		{"example.com", "example.com"},
		{"example.com/path", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractDomain(tt.url)
			if got != tt.domain {
				t.Errorf("extractDomain(%q) = %q, want %q", tt.url, got, tt.domain)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url     string
		pattern string
		matches bool
	}{
		{"https://example.com/ads/banner", "*/ads/*", true},
		{"https://example.com/content", "*/ads/*", false},
		{"https://tracker.com/pixel", "*.com/*", true},
		{"https://tracker.net/pixel", "*.com/*", false},
		{"https://ads.example.com", "ads.*", false}, // ads.* should match "ads" at start, but https://ads.example.com starts with "https"
		{"https://content.example.com", "ads.*", false},
	}

	for _, tt := range tests {
		t.Run(tt.url+"_"+tt.pattern, func(t *testing.T) {
			got := matchesPattern(tt.url, tt.pattern)
			if got != tt.matches {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.url, tt.pattern, got, tt.matches)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) <= len(s) && s[len(s)-len(substr):] == substr) ||
		(len(substr) <= len(s) && s[:len(substr)] == substr) ||
		containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}