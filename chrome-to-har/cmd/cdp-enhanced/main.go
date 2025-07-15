// Enhanced CDP command-line tool with Playwright-like capabilities
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

// Command represents a CDP command
type Command struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, page *browser.Page, args []string) error
}

var commands = map[string]Command{
	// Navigation
	"goto": {
		Name:        "goto",
		Description: "Navigate to URL",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("URL required")
			}
			return page.Navigate(args[0])
		},
	},
	"reload": {
		Name:        "reload",
		Description: "Reload the page",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			return page.Navigate(page.URL())
		},
	},

	// Page info
	"title": {
		Name:        "title",
		Description: "Get page title",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			title, err := page.Title()
			if err != nil {
				return err
			}
			fmt.Println("Title:", title)
			return nil
		},
	},
	"url": {
		Name:        "url",
		Description: "Get current URL",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			url, err := page.URL()
			if err != nil {
				return err
			}
			fmt.Println("URL:", url)
			return nil
		},
	},

	// Element interaction
	"click": {
		Name:        "click",
		Description: "Click an element",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return page.Click(args[0])
		},
	},
	"type": {
		Name:        "type",
		Description: "Type text into an element",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 2 {
				return errors.New("selector and text required")
			}
			return page.Type(args[0], args[1])
		},
	},
	"hover": {
		Name:        "hover",
		Description: "Hover over an element",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return page.Hover(args[0])
		},
	},

	// Content extraction
	"text": {
		Name:        "text",
		Description: "Get text content of an element",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			text, err := page.GetText(args[0])
			if err != nil {
				return err
			}
			fmt.Println("Text:", text)
			return nil
		},
	},
	"html": {
		Name:        "html",
		Description: "Get page HTML",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			html, err := page.Content()
			if err != nil {
				return err
			}
			fmt.Println(html)
			return nil
		},
	},

	// Wait commands
	"wait": {
		Name:        "wait",
		Description: "Wait for selector",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return page.WaitForSelector(args[0])
		},
	},
	"waitfor": {
		Name:        "waitfor",
		Description: "Wait for time in milliseconds",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("milliseconds required")
			}
			var ms int
			fmt.Sscanf(args[0], "%d", &ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			return nil
		},
	},

	// Screenshot
	"screenshot": {
		Name:        "screenshot",
		Description: "Take a screenshot",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			filename := "screenshot.png"
			if len(args) > 0 {
				filename = args[0]
			}

			opts := []browser.ScreenshotOption{}
			if len(args) > 1 && args[1] == "fullpage" {
				opts = append(opts, browser.WithFullPage())
			}

			buf, err := page.Screenshot(opts...)
			if err != nil {
				return err
			}

			if err := os.WriteFile(filename, buf, 0644); err != nil {
				return err
			}

			fmt.Println("Screenshot saved to:", filename)
			return nil
		},
	},

	// JavaScript evaluation
	"eval": {
		Name:        "eval",
		Description: "Evaluate JavaScript",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("expression required")
			}

			var result interface{}
			if err := page.Evaluate(args[0], &result); err != nil {
				return err
			}

			// Pretty print result
			if result != nil {
				if data, err := json.MarshalIndent(result, "", "  "); err == nil {
					fmt.Println("Result:", string(data))
				} else {
					fmt.Println("Result:", result)
				}
			}
			return nil
		},
	},

	// Locator-based commands
	"find": {
		Name:        "find",
		Description: "Find elements matching selector",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}

			elements, err := page.QuerySelectorAll(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Found %d element(s)\n", len(elements))
			for i, el := range elements {
				text, _ := el.GetText()
				fmt.Printf("[%d] %s\n", i, text)
			}
			return nil
		},
	},

	// Network commands
	"route": {
		Name:        "route",
		Description: "Intercept network requests",
		Handler: func(ctx context.Context, page *browser.Page, args []string) error {
			if len(args) < 2 {
				return errors.New("pattern and action required")
			}

			pattern := args[0]
			action := args[1]

			return page.Route(pattern, func(req *browser.Request) error {
				fmt.Printf("Intercepted: %s %s\n", req.Method, req.URL)

				switch action {
				case "abort":
					return req.Abort("aborted")
				case "log":
					fmt.Printf("Headers: %v\n", req.Headers)
					return req.Continue()
				default:
					return req.Continue()
				}
			})
		},
	},
}

func main() {
	var (
		remoteHost string
		remotePort int
		remoteTab  string
		listTabs   bool
		headless   bool
		timeout    int
		verbose    bool
	)

	flag.StringVar(&remoteHost, "remote-host", "", "Connect to remote Chrome at this host")
	flag.IntVar(&remotePort, "remote-port", 9222, "Remote Chrome debugging port")
	flag.StringVar(&remoteTab, "remote-tab", "", "Connect to specific tab ID")
	flag.BoolVar(&listTabs, "list-tabs", false, "List available tabs")
	flag.BoolVar(&headless, "headless", false, "Run Chrome in headless mode")
	flag.IntVar(&timeout, "timeout", 60, "Timeout in seconds")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	flag.Parse()

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Create profile manager
	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(verbose),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create browser options
	browserOpts := []browser.Option{
		browser.WithHeadless(headless),
		browser.WithTimeout(timeout),
		browser.WithVerbose(verbose),
	}

	if remoteHost != "" {
		browserOpts = append(browserOpts, browser.WithRemoteChrome(remoteHost, remotePort))
		if remoteTab != "" {
			browserOpts = append(browserOpts, browser.WithRemoteTab(remoteTab))
		}
	}

	// Create browser
	b, err := browser.New(ctx, pm, browserOpts...)
	if err != nil {
		log.Fatal(err)
	}

	// Handle list tabs
	if listTabs && remoteHost != "" {
		tabs, err := browser.ListTabs(remoteHost, remotePort)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Available tabs on %s:%d:\n\n", remoteHost, remotePort)
		for i, tab := range tabs {
			fmt.Printf("[%d] %s\n", i, tab.Title)
			fmt.Printf("    URL: %s\n", tab.URL)
			fmt.Printf("    Type: %s\n", tab.Type)
			fmt.Printf("    ID: %s\n\n", tab.ID)
		}
		return
	}

	// Launch browser
	if err := b.Launch(ctx); err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	// Get page
	var page *browser.Page
	if remoteTab != "" {
		page, err = b.AttachToTarget(remoteTab)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		pages, err := b.Pages()
		if err != nil || len(pages) == 0 {
			page, err = b.NewPage()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			page = pages[0]
		}
	}

	// Interactive REPL
	fmt.Println("Enhanced CDP - Chrome DevTools Protocol CLI")
	fmt.Println("Type 'help' for available commands")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("cdp> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "exit" || line == "quit" {
			break
		}

		if line == "help" {
			printHelp()
			continue
		}

		// Parse command
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmdName := parts[0]
		args := parts[1:]

		// Execute command
		if cmd, ok := commands[cmdName]; ok {
			if err := cmd.Handler(ctx, page, args); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			fmt.Printf("Unknown command: %s\n", cmdName)
			fmt.Println("Type 'help' for available commands")
		}
	}

	fmt.Println("\nGoodbye!")
}

func printHelp() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("\nNavigation:")
	fmt.Println("  goto <url>          Navigate to URL")
	fmt.Println("  reload              Reload the page")

	fmt.Println("\nPage Info:")
	fmt.Println("  title               Get page title")
	fmt.Println("  url                 Get current URL")
	fmt.Println("  html                Get page HTML")

	fmt.Println("\nElement Interaction:")
	fmt.Println("  click <selector>    Click an element")
	fmt.Println("  type <sel> <text>   Type text into element")
	fmt.Println("  hover <selector>    Hover over element")
	fmt.Println("  text <selector>     Get element text")

	fmt.Println("\nWaiting:")
	fmt.Println("  wait <selector>     Wait for element")
	fmt.Println("  waitfor <ms>        Wait for milliseconds")

	fmt.Println("\nJavaScript:")
	fmt.Println("  eval <expression>   Evaluate JavaScript")

	fmt.Println("\nScreenshot:")
	fmt.Println("  screenshot [file] [fullpage]  Take screenshot")

	fmt.Println("\nSearch:")
	fmt.Println("  find <selector>     Find all matching elements")

	fmt.Println("\nNetwork:")
	fmt.Println("  route <pattern> <action>  Intercept requests (actions: abort, log)")

	fmt.Println("\nOther:")
	fmt.Println("  help                Show this help")
	fmt.Println("  exit/quit           Exit the program")

	fmt.Println("\nSelector Examples:")
	fmt.Println("  css=.class          CSS selector")
	fmt.Println("  xpath=//div         XPath selector")
	fmt.Println("  text=Hello          Text selector")
	fmt.Println("  role=button         ARIA role selector")
}
