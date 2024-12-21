/*
chrome-to-har launches a headed Chrome browser and captures network activity to HAR format.

Usage:

	chrome-to-har [flags]

Flags:

	-profile string       Chrome profile directory to use (e.g., "Default" or "Profile 1")
	-output string       Output HAR file (default "output.har")
	-url string         Starting URL to navigate to
	-verbose           Enable verbose logging
	-cookies regexp    Regular expression to filter cookies in HAR output
	-urls regexp       Regular expression to filter URLs
	-block regexp      Regular expression of URLs to block from loading
	-omit regexp      Regular expression of URLs to omit from HAR output
	-cookie-domains    Comma-separated list of domains to include cookies from
	-diff             Enable differential HAR capture
	-list-profiles    List available Chrome profiles
	-restore-session  Restore previous session on startup (default false)

Examples:

	# List available profiles
	chrome-to-har -list-profiles

	# Use default profile and capture traffic
	chrome-to-har -profile=Default -output=capture.har

	# Start with specific URL and filter cookies
	chrome-to-har -profile=Default -url=https://example.com -cookies="session.*"

	# Use profile with specific cookie domains
	chrome-to-har -profile=Default -cookie-domains="example.com,api.example.com"
*/
package main
