// Example demonstrating Playwright-like capabilities
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
	// Create context
	ctx := context.Background()
	
	// Create profile manager
	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		log.Fatal(err)
	}
	
	// Create browser with options
	b, err := browser.New(ctx, pm,
		browser.WithHeadless(false),
		browser.WithVerbose(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	
	// Launch browser
	fmt.Println("Launching browser...")
	if err := b.Launch(ctx); err != nil {
		log.Fatal(err)
	}
	defer b.Close()
	
	// Create a new page
	fmt.Println("Creating new page...")
	page, err := b.NewPage()
	if err != nil {
		log.Fatal(err)
	}
	defer page.Close()
	
	// Example 1: Basic navigation and interaction
	fmt.Println("\n=== Example 1: Basic Navigation ===")
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatal(err)
	}
	
	title, _ := page.Title()
	fmt.Printf("Page title: %s\n", title)
	
	// Example 2: Element interaction with selectors
	fmt.Println("\n=== Example 2: Element Selectors ===")
	
	// Using CSS selector
	if link, err := page.QuerySelector("a"); link != nil && err == nil {
		text, _ := link.GetText()
		fmt.Printf("First link text: %s\n", text)
	}
	
	// Using XPath selector
	if heading, err := page.QuerySelector("xpath=//h1"); heading != nil && err == nil {
		text, _ := heading.GetText()
		fmt.Printf("H1 text: %s\n", text)
	}
	
	// Example 3: Using locators
	fmt.Println("\n=== Example 3: Locators ===")
	
	// Create a locator for all paragraphs
	paragraphs := page.Locator("p")
	count, _ := paragraphs.Count()
	fmt.Printf("Found %d paragraphs\n", count)
	
	// Get text from first paragraph
	if text, err := paragraphs.GetText(); err == nil {
		fmt.Printf("First paragraph: %s\n", text)
	}
	
	// Example 4: Waiting for elements
	fmt.Println("\n=== Example 4: Waiting ===")
	
	// Navigate to a dynamic page
	if err := page.Navigate("https://httpbin.org/delay/2"); err == nil {
		fmt.Println("Waiting for body to be visible...")
		if err := page.WaitForSelector("body", browser.WithWaitTimeout(5*time.Second)); err == nil {
			fmt.Println("Body is visible!")
		}
	}
	
	// Example 5: JavaScript evaluation
	fmt.Println("\n=== Example 5: JavaScript Evaluation ===")
	
	// Evaluate simple expression
	var result interface{}
	if err := page.Evaluate("1 + 2", &result); err == nil {
		fmt.Printf("1 + 2 = %v\n", result)
	}
	
	// Get window dimensions
	var dimensions map[string]interface{}
	if err := page.Evaluate(`({
		width: window.innerWidth,
		height: window.innerHeight,
		devicePixelRatio: window.devicePixelRatio
	})`, &dimensions); err == nil {
		fmt.Printf("Window dimensions: %v\n", dimensions)
	}
	
	// Example 6: Screenshots
	fmt.Println("\n=== Example 6: Screenshots ===")
	
	// Take full page screenshot
	if buf, err := page.Screenshot(browser.WithFullPage()); err == nil {
		fmt.Printf("Full page screenshot: %d bytes\n", len(buf))
	}
	
	// Take element screenshot
	if element, err := page.QuerySelector("h1"); element != nil && err == nil {
		if buf, err := element.Screenshot(); err == nil {
			fmt.Printf("H1 element screenshot: %d bytes\n", len(buf))
		}
	}
	
	// Example 7: Network interception
	fmt.Println("\n=== Example 7: Network Interception ===")
	
	// Intercept and log all image requests
	page.Route(".*\\.(png|jpg|jpeg|gif)", func(req *browser.Request) error {
		fmt.Printf("Intercepted image: %s\n", req.URL)
		// Continue the request normally
		return req.Continue()
	})
	
	// Block specific requests
	page.Route(".*google-analytics.*", func(req *browser.Request) error {
		fmt.Printf("Blocking analytics: %s\n", req.URL)
		return req.Abort("blocked")
	})
	
	// Navigate to a page with images
	page.Navigate("https://example.com")
	
	// Example 8: Complex interactions
	fmt.Println("\n=== Example 8: Complex Interactions ===")
	
	// Fill a form (if it existed)
	formExample := func() {
		// This is example code - would work on a real form
		page.Type("input[name='username']", "testuser")
		page.Type("input[name='password']", "testpass")
		page.Click("button[type='submit']")
		
		// Wait for navigation
		page.WaitForSelector(".success-message", browser.WithWaitTimeout(5*time.Second))
	}
	_ = formExample // Suppress unused warning
	
	// Example 9: Multiple pages
	fmt.Println("\n=== Example 9: Multiple Pages ===")
	
	// Create another page
	page2, err := b.NewPage()
	if err == nil {
		defer page2.Close()
		
		page2.Navigate("https://httpbin.org/headers")
		
		// List all pages
		if pages, err := b.Pages(); err == nil {
			fmt.Printf("Total pages open: %d\n", len(pages))
		}
	}
	
	// Example 10: Wait for specific conditions
	fmt.Println("\n=== Example 10: Advanced Waiting ===")
	
	// Wait for a function to return true
	if err := page.WaitForFunction("document.readyState === 'complete'", 5*time.Second); err == nil {
		fmt.Println("Page fully loaded!")
	}
	
	fmt.Println("\nDemo complete!")
}

// Additional examples that could be implemented:

// File upload example
func fileUploadExample(page *browser.Page) {
	// Navigate to upload page
	page.Navigate("https://example.com/upload")
	
	// Set file input
	if input, err := page.QuerySelector("input[type='file']"); err == nil {
		// Would need to implement SetInputFiles method
		// input.SetInputFiles("/path/to/file.pdf")
		_ = input
	}
}

// Drag and drop example
func dragDropExample(page *browser.Page) {
	// Would need to implement drag and drop
	// page.DragAndDrop("#source", "#target")
}

// Keyboard shortcuts example
func keyboardExample(page *browser.Page) {
	// Press keyboard shortcuts
	page.Press("Control+A") // Select all
	page.Press("Control+C") // Copy
	page.Press("Escape")    // Escape
}

// Frame handling example
func frameExample(page *browser.Page) {
	// Would need to implement frame support
	// frame := page.Frame("iframe[name='content']")
	// frame.Click("button")
}