// Test Chrome extension approach for AI API access
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	fmt.Println("=== Testing Chrome Extension Approach ===")
	fmt.Println("1. Load the extension from ./extension/ folder in Chrome")
	fmt.Println("2. Navigate to any webpage")
	fmt.Println("3. Press Enter when ready...")
	fmt.Scanln()

	// Connect to Chrome with the extension loaded
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9222")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Test extension communication
	fmt.Println("Testing extension communication...")

	var result interface{}
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://example.com"),
		chromedp.Sleep(3*time.Second),
		// Try to communicate with the extension
		chromedp.Evaluate(`
			new Promise(async (resolve) => {
				try {
					// Check if extension is loaded
					if (typeof chrome !== 'undefined' && chrome.runtime) {
						// Try to get AI status from extension
						chrome.runtime.sendMessage({
							type: 'GET_AI_STATUS'
						}, (response) => {
							resolve({ extensionLoaded: true, aiStatus: response });
						});
					} else {
						resolve({ extensionLoaded: false, error: 'Extension not detected' });
					}
				} catch (error) {
					resolve({ error: error.message });
				}
			})
		`, &result),
	)

	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Extension Test Result:\n%s\n", resultJSON)
	}
}
