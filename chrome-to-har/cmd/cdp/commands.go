package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/pkg/errors"
)

// Command represents a CDP command with metadata
type Command struct {
	Name        string
	Category    string
	Description string
	Usage       string
	Examples    []string
	Handler     func(ctx context.Context, args []string) error
	Aliases     []string
}

// CommandCategory represents a group of related commands
type CommandCategory struct {
	Name        string
	Description string
	Commands    []*Command
}

// CommandRegistry holds all available commands organized by category
type CommandRegistry struct {
	categories map[string]*CommandCategory
	commands   map[string]*Command
	aliases    map[string]string
}

// NewCommandRegistry creates a new command registry with all built-in commands
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		categories: make(map[string]*CommandCategory),
		commands:   make(map[string]*Command),
		aliases:    make(map[string]string),
	}

	// Register all command categories
	r.registerNavigationCommands()
	r.registerDOMCommands()
	r.registerNetworkCommands()
	r.registerDebugCommands()
	r.registerPerformanceCommands()
	r.registerEmulationCommands()
	r.registerStorageCommands()
	r.registerSecurityCommands()
	r.registerPageCommands()
	r.registerConsoleCommands()

	return r
}

// RegisterCommand adds a command to the registry
func (r *CommandRegistry) RegisterCommand(cmd *Command) {
	// Get or create category
	cat, exists := r.categories[cmd.Category]
	if !exists {
		cat = &CommandCategory{
			Name:     cmd.Category,
			Commands: []*Command{},
		}
		r.categories[cmd.Category] = cat
	}

	// Add command to category
	cat.Commands = append(cat.Commands, cmd)

	// Register command by name
	r.commands[cmd.Name] = cmd

	// Register aliases
	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd.Name
	}
}

// GetCommand retrieves a command by name or alias
func (r *CommandRegistry) GetCommand(name string) (*Command, bool) {
	// Check direct command
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}

	// Check alias
	if cmdName, ok := r.aliases[name]; ok {
		return r.commands[cmdName], true
	}

	return nil, false
}

// ListCategories returns all command categories
func (r *CommandRegistry) ListCategories() []*CommandCategory {
	var categories []*CommandCategory
	for _, cat := range r.categories {
		categories = append(categories, cat)
	}
	return categories
}

// registerNavigationCommands adds navigation-related commands
func (r *CommandRegistry) registerNavigationCommands() {
	r.RegisterCommand(&Command{
		Name:        "navigate",
		Category:    "Navigation",
		Description: "Navigate to a URL",
		Usage:       "navigate <url>",
		Examples:    []string{"navigate https://example.com", "goto https://google.com"},
		Aliases:     []string{"goto", "go", "nav"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("URL required")
			}
			return chromedp.Run(ctx, chromedp.Navigate(args[0]))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "reload",
		Category:    "Navigation",
		Description: "Reload the current page",
		Usage:       "reload [hard]",
		Examples:    []string{"reload", "reload hard", "refresh"},
		Aliases:     []string{"refresh", "r"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.Reload())
		},
	})

	r.RegisterCommand(&Command{
		Name:        "back",
		Category:    "Navigation",
		Description: "Go back in history",
		Usage:       "back",
		Examples:    []string{"back", "b"},
		Aliases:     []string{"b"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.NavigateBack())
		},
	})

	r.RegisterCommand(&Command{
		Name:        "forward",
		Category:    "Navigation",
		Description: "Go forward in history",
		Usage:       "forward",
		Examples:    []string{"forward", "f"},
		Aliases:     []string{"f", "fwd"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.NavigateForward())
		},
	})

	r.RegisterCommand(&Command{
		Name:        "stop",
		Category:    "Navigation",
		Description: "Stop page loading",
		Usage:       "stop",
		Examples:    []string{"stop"},
		Aliases:     []string{"s"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.Stop())
		},
	})
}

// registerDOMCommands adds DOM manipulation commands
func (r *CommandRegistry) registerDOMCommands() {
	r.RegisterCommand(&Command{
		Name:        "click",
		Category:    "DOM",
		Description: "Click an element",
		Usage:       "click <selector>",
		Examples:    []string{"click #submit", "click button.primary"},
		Aliases:     []string{"c"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return chromedp.Run(ctx, chromedp.Click(args[0]))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "type",
		Category:    "DOM",
		Description: "Type text into an element",
		Usage:       "type <selector> <text>",
		Examples:    []string{"type #username john@example.com", "type input[name=password] secret123"},
		Aliases:     []string{"input", "fill"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return errors.New("selector and text required")
			}
			text := strings.Join(args[1:], " ")
			return chromedp.Run(ctx, chromedp.SendKeys(args[0], text))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "clear",
		Category:    "DOM",
		Description: "Clear an input field",
		Usage:       "clear <selector>",
		Examples:    []string{"clear #username", "clear input.search"},
		Aliases:     []string{"clr"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return chromedp.Run(ctx, chromedp.Clear(args[0]))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "focus",
		Category:    "DOM",
		Description: "Focus an element",
		Usage:       "focus <selector>",
		Examples:    []string{"focus #search", "focus input:first-child"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return chromedp.Run(ctx, chromedp.Focus(args[0]))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "submit",
		Category:    "DOM",
		Description: "Submit a form",
		Usage:       "submit <selector>",
		Examples:    []string{"submit #loginForm", "submit form"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			return chromedp.Run(ctx, chromedp.Submit(args[0]))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "text",
		Category:    "DOM",
		Description: "Get text content of an element",
		Usage:       "text <selector>",
		Examples:    []string{"text h1", "text .article-content"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("selector required")
			}
			var text string
			if err := chromedp.Run(ctx, chromedp.Text(args[0], &text)); err != nil {
				return err
			}
			fmt.Println(text)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "html",
		Category:    "DOM",
		Description: "Get HTML content",
		Usage:       "html [selector]",
		Examples:    []string{"html", "html #content", "html body"},
		Handler: func(ctx context.Context, args []string) error {
			var html string
			selector := "html"
			if len(args) > 0 {
				selector = args[0]
			}
			if err := chromedp.Run(ctx, chromedp.OuterHTML(selector, &html)); err != nil {
				return err
			}
			fmt.Println(html)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "attr",
		Category:    "DOM",
		Description: "Get attribute value",
		Usage:       "attr <selector> <attribute>",
		Examples:    []string{"attr #logo src", "attr a href"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return errors.New("selector and attribute name required")
			}
			var value string
			var ok bool
			if err := chromedp.Run(ctx, chromedp.AttributeValue(args[0], args[1], &value, &ok)); err != nil {
				return err
			}
			if !ok {
				fmt.Printf("Attribute %s not found\n", args[1])
			} else {
				fmt.Println(value)
			}
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "setattr",
		Category:    "DOM",
		Description: "Set attribute value",
		Usage:       "setattr <selector> <attribute> <value>",
		Examples:    []string{"setattr #myInput value newValue", "setattr button disabled true"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 3 {
				return errors.New("selector, attribute name and value required")
			}
			value := strings.Join(args[2:], " ")
			return chromedp.Run(ctx, chromedp.SetAttributeValue(args[0], args[1], value))
		},
	})
}

// registerNetworkCommands adds network-related commands
func (r *CommandRegistry) registerNetworkCommands() {
	r.RegisterCommand(&Command{
		Name:        "cookies",
		Category:    "Network",
		Description: "Get all cookies",
		Usage:       "cookies",
		Examples:    []string{"cookies"},
		Handler: func(ctx context.Context, args []string) error {
			var cookies []*network.Cookie
			if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				cookies, err = network.GetCookies().Do(ctx)
				return err
			})); err != nil {
				return err
			}
			data, _ := json.MarshalIndent(cookies, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "setcookie",
		Category:    "Network",
		Description: "Set a cookie",
		Usage:       "setcookie <name> <value> [domain]",
		Examples:    []string{"setcookie session abc123", "setcookie token xyz789 .example.com"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return errors.New("name and value required")
			}
			// Simple cookie set via JavaScript
			js := fmt.Sprintf(`document.cookie = "%s=%s"`, args[0], args[1])
			if len(args) > 2 {
				js = fmt.Sprintf(`document.cookie = "%s=%s; domain=%s"`, args[0], args[1], args[2])
			}
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "deletecookie",
		Category:    "Network",
		Description: "Delete a cookie",
		Usage:       "deletecookie <name>",
		Examples:    []string{"deletecookie session", "delcookie token"},
		Aliases:     []string{"delcookie"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("cookie name required")
			}
			// Delete cookie by setting it to expire in the past
			js := fmt.Sprintf(`document.cookie = "%s=; expires=Thu, 01 Jan 1970 00:00:01 GMT"`, args[0])
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "clearcookies",
		Category:    "Network",
		Description: "Clear all cookies",
		Usage:       "clearcookies",
		Examples:    []string{"clearcookies"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				expr := `
					document.cookie.split(";").forEach(function(c) {
						document.cookie = c.replace(/^ +/, "").replace(/=.*/, "=;expires=" + new Date().toUTCString() + ";path=/");
					});
				`
				return chromedp.Evaluate(expr, nil).Do(ctx)
			}))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "headers",
		Category:    "Network",
		Description: "Show response headers (requires HAR recording)",
		Usage:       "headers",
		Examples:    []string{"headers"},
		Handler: func(ctx context.Context, args []string) error {
			fmt.Println("Note: This requires HAR recording to be enabled")
			return nil
		},
	})
}

// registerDebugCommands adds debugging commands
func (r *CommandRegistry) registerDebugCommands() {
	r.RegisterCommand(&Command{
		Name:        "pause",
		Category:    "Debug",
		Description: "Pause JavaScript execution",
		Usage:       "pause",
		Examples:    []string{"pause"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.Evaluate("debugger", nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "sources",
		Category:    "Debug",
		Description: "List page sources",
		Usage:       "sources",
		Examples:    []string{"sources"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				Array.from(document.querySelectorAll('script[src]')).map(s => s.src)
			`, &result))
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	})
}

// registerPerformanceCommands adds performance monitoring commands
func (r *CommandRegistry) registerPerformanceCommands() {
	r.RegisterCommand(&Command{
		Name:        "metrics",
		Category:    "Performance",
		Description: "Get performance metrics",
		Usage:       "metrics",
		Examples:    []string{"metrics", "perf"},
		Aliases:     []string{"perf"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				JSON.stringify(performance.timing, null, 2)
			`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "memory",
		Category:    "Performance",
		Description: "Get memory usage",
		Usage:       "memory",
		Examples:    []string{"memory", "mem"},
		Aliases:     []string{"mem"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				performance.memory ? JSON.stringify(performance.memory, null, 2) : "Memory API not available"
			`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})
}

// registerEmulationCommands adds device emulation commands
func (r *CommandRegistry) registerEmulationCommands() {
	r.RegisterCommand(&Command{
		Name:        "mobile",
		Category:    "Emulation",
		Description: "Emulate mobile device",
		Usage:       "mobile [device]",
		Examples:    []string{"mobile", "mobile iphone", "mobile pixel"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx,
				chromedp.EmulateViewport(375, 812, chromedp.EmulateScale(3), chromedp.EmulateMobile),
			)
		},
	})

	r.RegisterCommand(&Command{
		Name:        "desktop",
		Category:    "Emulation",
		Description: "Reset to desktop view",
		Usage:       "desktop",
		Examples:    []string{"desktop"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
			)
		},
	})

	r.RegisterCommand(&Command{
		Name:        "viewport",
		Category:    "Emulation",
		Description: "Set viewport size",
		Usage:       "viewport <width> <height>",
		Examples:    []string{"viewport 1024 768", "viewport 1920 1080"},
		Aliases:     []string{"vp", "size"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return errors.New("width and height required")
			}
			var width, height int
			if _, err := fmt.Sscanf(args[0], "%d", &width); err != nil {
				return errors.Wrap(err, "invalid width")
			}
			if _, err := fmt.Sscanf(args[1], "%d", &height); err != nil {
				return errors.Wrap(err, "invalid height")
			}
			return chromedp.Run(ctx, chromedp.EmulateViewport(int64(width), int64(height)))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "offline",
		Category:    "Emulation",
		Description: "Emulate offline mode",
		Usage:       "offline",
		Examples:    []string{"offline"},
		Handler: func(ctx context.Context, args []string) error {
			// Use JavaScript to simulate offline
			return chromedp.Run(ctx, chromedp.Evaluate(`
				Object.defineProperty(navigator, 'onLine', {
					get: function() { return false; }
				});
				window.dispatchEvent(new Event('offline'));
			`, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "online",
		Category:    "Emulation",
		Description: "Reset to online mode",
		Usage:       "online",
		Examples:    []string{"online"},
		Handler: func(ctx context.Context, args []string) error {
			// Use JavaScript to simulate online
			return chromedp.Run(ctx, chromedp.Evaluate(`
				Object.defineProperty(navigator, 'onLine', {
					get: function() { return true; }
				});
				window.dispatchEvent(new Event('online'));
			`, nil))
		},
	})
}

// registerStorageCommands adds local storage and session storage commands
func (r *CommandRegistry) registerStorageCommands() {
	r.RegisterCommand(&Command{
		Name:        "localStorage",
		Category:    "Storage",
		Description: "Get all localStorage items",
		Usage:       "localStorage",
		Examples:    []string{"localStorage", "ls"},
		Aliases:     []string{"ls"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				JSON.stringify(Object.entries(localStorage), null, 2)
			`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "setLocal",
		Category:    "Storage",
		Description: "Set localStorage item",
		Usage:       "setLocal <key> <value>",
		Examples:    []string{"setLocal theme dark", "setLocal user john"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return errors.New("key and value required")
			}
			value := strings.Join(args[1:], " ")
			js := fmt.Sprintf(`localStorage.setItem('%s', '%s')`, args[0], value)
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "getLocal",
		Category:    "Storage",
		Description: "Get localStorage item",
		Usage:       "getLocal <key>",
		Examples:    []string{"getLocal theme", "getLocal user"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("key required")
			}
			var result interface{}
			js := fmt.Sprintf(`localStorage.getItem('%s')`, args[0])
			if err := chromedp.Run(ctx, chromedp.Evaluate(js, &result)); err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "clearLocal",
		Category:    "Storage",
		Description: "Clear all localStorage",
		Usage:       "clearLocal",
		Examples:    []string{"clearLocal"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.Evaluate(`localStorage.clear()`, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "sessionStorage",
		Category:    "Storage",
		Description: "Get all sessionStorage items",
		Usage:       "sessionStorage",
		Examples:    []string{"sessionStorage", "ss"},
		Aliases:     []string{"ss"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				JSON.stringify(Object.entries(sessionStorage), null, 2)
			`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})
}

// registerSecurityCommands adds security-related commands
func (r *CommandRegistry) registerSecurityCommands() {
	r.RegisterCommand(&Command{
		Name:        "csp",
		Category:    "Security",
		Description: "Get Content Security Policy",
		Usage:       "csp",
		Examples:    []string{"csp"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`
				Array.from(document.querySelectorAll('meta[http-equiv="Content-Security-Policy"]'))
					.map(m => m.content).join('\n') || 'No CSP meta tags found'
			`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "origin",
		Category:    "Security",
		Description: "Get page origin",
		Usage:       "origin",
		Examples:    []string{"origin"},
		Handler: func(ctx context.Context, args []string) error {
			var result interface{}
			err := chromedp.Run(ctx, chromedp.Evaluate(`window.location.origin`, &result))
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})
}

// registerPageCommands adds page information commands
func (r *CommandRegistry) registerPageCommands() {
	r.RegisterCommand(&Command{
		Name:        "title",
		Category:    "Page",
		Description: "Get page title",
		Usage:       "title",
		Examples:    []string{"title"},
		Handler: func(ctx context.Context, args []string) error {
			var title string
			if err := chromedp.Run(ctx, chromedp.Title(&title)); err != nil {
				return err
			}
			fmt.Println(title)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "url",
		Category:    "Page",
		Description: "Get current URL",
		Usage:       "url",
		Examples:    []string{"url"},
		Handler: func(ctx context.Context, args []string) error {
			var url string
			if err := chromedp.Run(ctx, chromedp.Location(&url)); err != nil {
				return err
			}
			fmt.Println(url)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "screenshot",
		Category:    "Page",
		Description: "Take a screenshot",
		Usage:       "screenshot [filename]",
		Examples:    []string{"screenshot", "screenshot page.png"},
		Aliases:     []string{"snap", "capture"},
		Handler: func(ctx context.Context, args []string) error {
			var buf []byte
			if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
				return err
			}

			filename := fmt.Sprintf("screenshot-%d.png", time.Now().Unix())
			if len(args) > 0 {
				filename = args[0]
			}

			if err := os.WriteFile(filename, buf, 0644); err != nil {
				return errors.Wrap(err, "saving screenshot")
			}

			fmt.Printf("Screenshot saved to: %s\n", filename)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "pdf",
		Category:    "Page",
		Description: "Save page as PDF",
		Usage:       "pdf [filename]",
		Examples:    []string{"pdf", "pdf page.pdf"},
		Handler: func(ctx context.Context, args []string) error {
			var buf []byte
			if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				buf, _, err = page.PrintToPDF().Do(ctx)
				return err
			})); err != nil {
				return err
			}

			filename := fmt.Sprintf("page-%d.pdf", time.Now().Unix())
			if len(args) > 0 {
				filename = args[0]
			}

			if err := os.WriteFile(filename, buf, 0644); err != nil {
				return errors.Wrap(err, "saving PDF")
			}

			fmt.Printf("PDF saved to: %s\n", filename)
			return nil
		},
	})

	r.RegisterCommand(&Command{
		Name:        "source",
		Category:    "Page",
		Description: "Get page source",
		Usage:       "source",
		Examples:    []string{"source"},
		Handler: func(ctx context.Context, args []string) error {
			var source string
			if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &source)); err != nil {
				return err
			}
			fmt.Println(source)
			return nil
		},
	})
}

// registerConsoleCommands adds console/logging commands
func (r *CommandRegistry) registerConsoleCommands() {
	r.RegisterCommand(&Command{
		Name:        "log",
		Category:    "Console",
		Description: "Log a message to console",
		Usage:       "log <message>",
		Examples:    []string{"log Hello World", "log Debug message"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("message required")
			}
			msg := strings.Join(args, " ")
			js := fmt.Sprintf(`console.log('%s')`, msg)
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "error",
		Category:    "Console",
		Description: "Log an error to console",
		Usage:       "error <message>",
		Examples:    []string{"error Something went wrong"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("message required")
			}
			msg := strings.Join(args, " ")
			js := fmt.Sprintf(`console.error('%s')`, msg)
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "warn",
		Category:    "Console",
		Description: "Log a warning to console",
		Usage:       "warn <message>",
		Examples:    []string{"warn Deprecated feature"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("message required")
			}
			msg := strings.Join(args, " ")
			js := fmt.Sprintf(`console.warn('%s')`, msg)
			return chromedp.Run(ctx, chromedp.Evaluate(js, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "clear_console",
		Category:    "Console",
		Description: "Clear the console",
		Usage:       "clear_console",
		Examples:    []string{"clear_console"},
		Handler: func(ctx context.Context, args []string) error {
			return chromedp.Run(ctx, chromedp.Evaluate(`console.clear()`, nil))
		},
	})

	r.RegisterCommand(&Command{
		Name:        "eval",
		Category:    "Console",
		Description: "Evaluate JavaScript expression",
		Usage:       "eval <expression>",
		Examples:    []string{"eval document.title", "eval window.location.href"},
		Aliases:     []string{"js", "exec"},
		Handler: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return errors.New("expression required")
			}
			expr := strings.Join(args, " ")
			var result interface{}
			if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &result)); err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	})
}