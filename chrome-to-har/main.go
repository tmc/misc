package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/termmd"
)

// options configures the chrome-to-har tool
type options struct {
	profileDir     string
	outputFile     string
	differential   bool
	verbose        bool
	startURL       string
	cookiePattern  string
	urlPattern     string
	blockPattern   string
	omitPattern    string
	cookieDomains  string
	listProfiles   bool
	restoreSession bool
	streaming      bool
	headless       bool
	filter         string
	template       string
}

// Runner handles the main application logic
type Runner struct {
	pm chromeprofiles.ProfileManager
}

// NewRunner creates a new runner with the given profile manager
func NewRunner(pm chromeprofiles.ProfileManager) *Runner {
	return &Runner{pm: pm}
}

func init() {
	flag.Usage = func() {
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintf(w, "chrome-to-har - Chrome network activity capture tool\n\n")
		fmt.Fprintf(w, "Version: %s\n\n", Version)
		fmt.Fprintf(w, "Usage:\n")
		fmt.Fprintf(w, "  chrome-to-har [options]\n\n")
		fmt.Fprintf(w, "Options:\n")
		
		lines := make([]string, 0)
		flag.VisitAll(func(f *flag.Flag) {
			def := f.DefValue
			if def != "" {
				def = fmt.Sprintf(" (default: %s)", def)
			}
			
			typ := ""
			switch f.Value.String() {
			case "false", "true":
				typ = "bool"
			case "0":
				typ = "int"
			default:
				typ = "string"
			}
			
			lines = append(lines, fmt.Sprintf("  -%s\t%s\t%s%s\n", f.Name, typ, f.Usage, def))
		})
		
		for _, line := range lines {
			fmt.Fprint(w, line)
		}
		
		doc, err := termmd.RenderMarkdown(GetUsageDoc())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering documentation: %v\n", err)
			return
		}
		
		fmt.Fprintf(os.Stderr, "\nDetailed Documentation:\n\n%s\n", doc)
	}
}

func main() {
	opts := options{}

	flag.StringVar(&opts.profileDir, "profile", "", "Chrome profile directory to use")
	flag.StringVar(&opts.outputFile, "output", "output.har", "Output HAR file")
	flag.BoolVar(&opts.differential, "diff", false, "Enable differential HAR capture")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&opts.startURL, "url", "", "Starting URL to navigate to")
	flag.StringVar(&opts.cookiePattern, "cookies", "", "Regular expression to filter cookies in HAR output")
	flag.StringVar(&opts.urlPattern, "urls", "", "Regular expression to filter URLs")
	flag.StringVar(&opts.blockPattern, "block", "", "Regular expression of URLs to block from loading")
	flag.StringVar(&opts.omitPattern, "omit", "", "Regular expression of URLs to omit from HAR output")
	flag.StringVar(&opts.cookieDomains, "cookie-domains", "", "Comma-separated list of domains to include cookies from")
	flag.BoolVar(&opts.listProfiles, "list-profiles", false, "List available Chrome profiles")
	flag.BoolVar(&opts.restoreSession, "restore-session", false, "Restore previous session on startup")
	flag.BoolVar(&opts.streaming, "stream", false, "Stream HAR entries as they are captured (outputs NDJSON)")
	flag.BoolVar(&opts.headless, "headless", false, "Run Chrome in headless mode")
	flag.StringVar(&opts.filter, "filter", "", "JQ expression to filter HAR entries")
	flag.StringVar(&opts.template, "template", "", "Go template to transform HAR entries")

	flag.Parse()

	if opts.listProfiles {
		if err := listAvailableProfiles(opts.verbose); err != nil {
			log.Fatal(err)
		}
		return
	}

	ctx := context.Background()
	if err := run(ctx, opts); err != nil {
		log.Fatal(err)
	}
}

func listAvailableProfiles(verbose bool) error {
	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(verbose),
	)
	if err != nil {
		return err
	}

	profiles, err := pm.ListProfiles()
	if err != nil {
		return err
	}

	fmt.Println("Available Chrome profiles:")
	for _, p := range profiles {
		fmt.Printf("  - %s\n", p)
	}
	return nil
}

func run(ctx context.Context, opts options) error {
	if opts.profileDir == "" {
		return fmt.Errorf("profile directory is required")
	}

	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(opts.verbose),
	)
	if err != nil {
		return fmt.Errorf("creating profile manager: %w", err)
	}

	runner := NewRunner(pm)
	return runner.Run(ctx, opts)
}

func (r *Runner) Run(ctx context.Context, opts options) error {
	if err := r.pm.SetupWorkdir(); err != nil {
		return fmt.Errorf("setting up working directory: %w", err)
	}
	defer r.pm.Cleanup()

	var cookieDomains []string
	if opts.cookieDomains != "" {
		cookieDomains = splitAndTrim(opts.cookieDomains, ",")
	}

	if err := r.pm.CopyProfile(opts.profileDir, cookieDomains); err != nil {
		return fmt.Errorf("copying profile: %w", err)
	}

	// TODO: Implement Chrome launch and HAR capture
	log.Printf("Starting Chrome with profile %s", opts.profileDir)
	if opts.startURL != "" {
		log.Printf("Navigating to %s", opts.startURL)
	}

	if opts.streaming {
		// Simulate some HAR entries for testing
		entries := r.generateTestHAREntries()
		for _, entry := range entries {
			if err := r.processStreamingEntry(entry, opts); err != nil {
				return fmt.Errorf("processing streaming entry: %w", err)
			}
		}
	}

	return nil
}

func (r *Runner) generateTestHAREntries() []*har.Entry {
	now := time.Now()
	return []*har.Entry{
		{
			StartedDateTime: now.Format(time.RFC3339),
			Request: &har.Request{
				Method: "GET",
				URL:    "https://example.com",
			},
			Response: &har.Response{
				Status: 200,
			},
		},
		{
			StartedDateTime: now.Add(time.Second).Format(time.RFC3339),
			Request: &har.Request{
				Method: "POST",
				URL:    "https://api.example.com/data",
			},
			Response: &har.Response{
				Status: 404,
			},
		},
		{
			StartedDateTime: now.Add(2 * time.Second).Format(time.RFC3339),
			Request: &har.Request{
				Method: "GET",
				URL:    "https://example.com/error",
			},
			Response: &har.Response{
				Status: 500,
			},
		},
	}
}

func (r *Runner) processStreamingEntry(entry *har.Entry, opts options) error {
	if opts.filter != "" {
		// Apply JQ filter if specified
		filtered, err := applyJQFilter(entry, opts.filter)
		if err != nil {
			return fmt.Errorf("applying filter: %w", err)
		}
		if filtered == nil {
			return nil // Entry filtered out
		}
		entry = filtered
	}

	if opts.template != "" {
		// Apply template if specified
		templated, err := applyTemplate(entry, opts.template)
		if err != nil {
			return fmt.Errorf("applying template: %w", err)
		}
		entry = templated
	}

	// Output the entry as JSON
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

// splitAndTrim splits a string by separator and trims spaces from parts
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// applyJQFilter applies a JQ filter to a HAR entry
func applyJQFilter(entry *har.Entry, filter string) (*har.Entry, error) {
	// TODO: Implement JQ filtering
	// For testing, just pass through entries with status >= 400 if filter contains that condition
	if strings.Contains(filter, "status >= 400") && entry.Response.Status < 400 {
		return nil, nil
	}
	return entry, nil
}

// applyTemplate applies a Go template to a HAR entry
func applyTemplate(entry *har.Entry, tmpl string) (*har.Entry, error) {
	// TODO: Implement template transformation
	// For testing, just return the original entry
	return entry, nil
}

