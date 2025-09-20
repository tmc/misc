package blocking

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// Config holds the configuration for URL/domain blocking
type Config struct {
	Verbose       bool
	Enabled       bool
	URLPatterns   []string
	Domains       []string
	RegexPatterns []string
	AllowURLs     []string
	AllowDomains  []string
	RuleFile      string
}

// BlockingStats holds statistics about blocking activity
type BlockingStats struct {
	RequestsBlocked   int
	RequestsAllowed   int
	DomainsBlocked    int
	PatternsBlocked   int
	TotalRules        int
}

// BlockingEngine handles URL and domain blocking logic
type BlockingEngine struct {
	config        *Config
	compiledRegex []*regexp.Regexp
	domainMap     map[string]bool
	allowMap      map[string]bool
	allowDomains  map[string]bool
	stats         *BlockingStats
	allRules      []string
}

// NewBlockingEngine creates a new blocking engine with the given configuration
func NewBlockingEngine(config *Config) (*BlockingEngine, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	engine := &BlockingEngine{
		config:       config,
		domainMap:    make(map[string]bool),
		allowMap:     make(map[string]bool),
		allowDomains: make(map[string]bool),
		stats:        &BlockingStats{},
		allRules:     make([]string, 0),
	}

	// Compile regex patterns
	for _, pattern := range config.RegexPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, errors.Wrapf(err, "compiling regex pattern: %s", pattern)
		}
		engine.compiledRegex = append(engine.compiledRegex, regex)
	}

	// Build domain lookup maps
	for _, domain := range config.Domains {
		engine.domainMap[strings.ToLower(domain)] = true
	}

	for _, domain := range config.AllowDomains {
		engine.allowDomains[strings.ToLower(domain)] = true
	}

	for _, url := range config.AllowURLs {
		engine.allowMap[url] = true
	}

	// Load rules from file if specified
	if config.RuleFile != "" {
		if err := engine.loadRulesFromFile(config.RuleFile); err != nil {
			return nil, errors.Wrapf(err, "loading rules from file: %s", config.RuleFile)
		}
	}

	return engine, nil
}

// ShouldBlock determines whether a URL should be blocked
func (e *BlockingEngine) ShouldBlock(url string) bool {
	if !e.config.Enabled {
		e.stats.RequestsAllowed++
		return false
	}

	// Check allow list first (takes precedence)
	if e.allowMap[url] {
		e.stats.RequestsAllowed++
		return false
	}

	// Check allow domains
	domain := extractDomain(url)
	if e.allowDomains[strings.ToLower(domain)] {
		e.stats.RequestsAllowed++
		return false
	}

	// Check domain blocking
	if e.domainMap[strings.ToLower(domain)] {
		if e.config.Verbose {
			fmt.Printf("Blocking domain: %s (URL: %s)\n", domain, url)
		}
		e.stats.RequestsBlocked++
		e.stats.DomainsBlocked++
		return true
	}

	// Check URL pattern matching
	for _, pattern := range e.config.URLPatterns {
		if matchesPattern(url, pattern) {
			if e.config.Verbose {
				fmt.Printf("Blocking URL pattern: %s (URL: %s)\n", pattern, url)
			}
			e.stats.RequestsBlocked++
			e.stats.PatternsBlocked++
			return true
		}
	}

	// Check regex patterns
	for _, regex := range e.compiledRegex {
		if regex.MatchString(url) {
			if e.config.Verbose {
				fmt.Printf("Blocking regex: %s (URL: %s)\n", regex.String(), url)
			}
			e.stats.RequestsBlocked++
			e.stats.PatternsBlocked++
			return true
		}
	}

	e.stats.RequestsAllowed++
	return false
}

// loadRulesFromFile loads blocking rules from a file
func (e *BlockingEngine) loadRulesFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "opening rule file")
	}
	defer func() {
		_ = file.Close() // Ignore close error in defer
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		// Add to domain map
		e.domainMap[strings.ToLower(line)] = true
	}

	return scanner.Err()
}

// extractDomain extracts the domain from a URL
func extractDomain(url string) string {
	// Remove protocol
	if strings.HasPrefix(url, "http://") {
		url = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = url[8:]
	}

	// Find the end of the domain (before path, query, or port)
	for i, char := range url {
		if char == '/' || char == '?' || char == ':' {
			return url[:i]
		}
	}

	return url
}

// matchesPattern checks if a URL matches a pattern (supports wildcards)
func matchesPattern(url, pattern string) bool {
	// Convert glob-style wildcards to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = "^" + regexPattern + "$"

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return regex.MatchString(url)
}

// AddCommonAdBlockRules adds common ad blocking rules to the engine
func (e *BlockingEngine) AddCommonAdBlockRules() error {
	commonAdDomains := []string{
		"doubleclick.net",
		"googletagmanager.com",
		"googlesyndication.com",
		"googleadservices.com",
		"facebook.com/tr",
		"google-analytics.com",
		"googletagservices.com",
		"amazon-adsystem.com",
		"adsystem.amazon.com",
		"ads.yahoo.com",
	}

	for _, domain := range commonAdDomains {
		e.domainMap[strings.ToLower(domain)] = true
		e.allRules = append(e.allRules, "domain:"+domain)
	}

	commonAdPatterns := []string{
		"*/ads/*",
		"*/advertisement/*",
		"*/banner/*",
		"*/popup/*",
		"*/tracking/*",
	}

	for _, pattern := range commonAdPatterns {
		e.allRules = append(e.allRules, "pattern:"+pattern)
	}

	e.stats.TotalRules = len(e.allRules)
	return nil
}

// AddCommonTrackingBlockRules adds common tracking blocking rules to the engine
func (e *BlockingEngine) AddCommonTrackingBlockRules() error {
	commonTrackingDomains := []string{
		"google-analytics.com",
		"googletagmanager.com",
		"hotjar.com",
		"mixpanel.com",
		"segment.com",
		"amplitude.com",
		"fullstory.com",
		"logrocket.com",
		"mouseflow.com",
		"crazy-egg.com",
	}

	for _, domain := range commonTrackingDomains {
		e.domainMap[strings.ToLower(domain)] = true
		e.allRules = append(e.allRules, "tracking:"+domain)
	}

	commonTrackingPatterns := []string{
		"*/analytics/*",
		"*/tracking/*",
		"*/telemetry/*",
		"*/metrics/*",
		"*/collect/*",
	}

	for _, pattern := range commonTrackingPatterns {
		e.allRules = append(e.allRules, "tracking-pattern:"+pattern)
	}

	e.stats.TotalRules = len(e.allRules)
	return nil
}

// ListRules returns all blocking rules currently active
func (e *BlockingEngine) ListRules() []string {
	rules := make([]string, 0, len(e.allRules))

	// Add configured rules
	for _, domain := range e.config.Domains {
		rules = append(rules, "config-domain:"+domain)
	}

	for _, pattern := range e.config.URLPatterns {
		rules = append(rules, "config-pattern:"+pattern)
	}

	for _, regex := range e.config.RegexPatterns {
		rules = append(rules, "config-regex:"+regex)
	}

	// Add common rules
	rules = append(rules, e.allRules...)

	return rules
}

// GetStats returns current blocking statistics as processed and blocked counts
func (e *BlockingEngine) GetStats() (processed, blocked int64) {
	if e.stats == nil {
		return 0, 0
	}

	processed = int64(e.stats.RequestsAllowed + e.stats.RequestsBlocked)
	blocked = int64(e.stats.RequestsBlocked)
	return processed, blocked
}

// GetDetailedStats returns detailed blocking statistics
func (e *BlockingEngine) GetDetailedStats() *BlockingStats {
	if e.stats == nil {
		return &BlockingStats{}
	}

	// Update total rules count
	e.stats.TotalRules = len(e.ListRules())

	return e.stats
}