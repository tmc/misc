// Extension runner - Test the browser extension approach
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
	fmt.Println("=== Extension Runner - Testing Browser Extension Approach ===")
	fmt.Println()

	// Connect to Chrome with extension loaded
	fmt.Println("Connecting to Chrome with extension...")

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9230")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// First, check if our extension is loaded
	fmt.Println("Checking if extension is loaded...")

	var extensionCheck map[string]interface{}
	err := chromedp.Run(ctx,
		chromedp.Navigate("chrome://extensions/"),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			({
				url: window.location.href,
				title: document.title,
				extensionsVisible: document.body.innerText.includes('AI API Bridge')
			})
		`, &extensionCheck),
	)

	if err != nil {
		log.Printf("Extension check error: %v", err)
		return
	}

	checkJSON, _ := json.MarshalIndent(extensionCheck, "", "  ")
	fmt.Printf("Extension check result:\n%s\n", checkJSON)

	// Navigate to a test page where we can use the extension
	fmt.Println("Navigating to test page...")

	err = chromedp.Run(ctx,
		chromedp.Navigate("https://example.com"),
		chromedp.Sleep(4*time.Second),
	)

	if err != nil {
		log.Printf("Navigation error: %v", err)
		return
	}

	// Test if our extension injected content script is working
	fmt.Println("Testing extension content script...")

	var contentScriptResult map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			({
				url: window.location.href,
				title: document.title,
				
				// Check if our extension's content script loaded
				extensionDetected: typeof window.postMessage !== 'undefined',
				
				// Try to detect AI APIs (extension should expose these)
				directAI: {
					languageModel: typeof LanguageModel !== 'undefined',
					windowAi: typeof window.ai !== 'undefined',
					chromeAi: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
				},
				
				// Check for extension-injected globals
				aiGlobals: Object.getOwnPropertyNames(window).filter(name => 
					name.toLowerCase().includes('ai') || 
					name.toLowerCase().includes('language') ||
					name.toLowerCase().includes('gemini') ||
					name.toLowerCase().includes('bridge')
				),
				
				// Check Chrome extension APIs
				chromeExtension: typeof chrome !== 'undefined' && typeof chrome.runtime !== 'undefined',
				extensionId: typeof chrome !== 'undefined' && chrome.runtime ? chrome.runtime.id : null
			})
		`, &contentScriptResult),
	)

	if err != nil {
		log.Printf("Content script test error: %v", err)
		return
	}

	resultJSON, _ := json.MarshalIndent(contentScriptResult, "", "  ")
	fmt.Printf("Content script test result:\n%s\n", resultJSON)

	// If extension is working, test AI API access through it
	if extensionWorking(contentScriptResult) {
		fmt.Println("\n🎉 Extension appears to be working! Testing AI API access...")
		testExtensionAI(ctx)
	} else {
		fmt.Println("\n❌ Extension not detected or not working properly")
		fmt.Println("Make sure to:")
		fmt.Println("1. Load the extension manually in Chrome")
		fmt.Println("2. Navigate to chrome://extensions and enable it")
		fmt.Println("3. Ensure Chrome has the AI flags enabled")
	}

	fmt.Println("\nManual testing instructions:")
	fmt.Println("1. Click the 'AI API Bridge' extension icon in Chrome")
	fmt.Println("2. Check if it shows 'AI API Available'")
	fmt.Println("3. Try the 'Test Generation' button")
	fmt.Println("4. Report back what you see!")
}

func extensionWorking(result map[string]interface{}) bool {
	// Check if Chrome extension APIs are available
	if chromeExt, ok := result["chromeExtension"].(bool); ok && chromeExt {
		return true
	}

	// Check if any AI APIs are directly available (extension might expose them)
	if directAI, ok := result["directAI"].(map[string]interface{}); ok {
		for _, available := range directAI {
			if val, ok := available.(bool); ok && val {
				return true
			}
		}
	}

	return false
}

func testExtensionAI(ctx context.Context) {
	fmt.Println("Testing AI API through extension bridge...")

	var testResult interface{}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		// Try to communicate with our extension
		script := `
			new Promise(async (resolve) => {
				try {
					// Method 1: Try direct AI API access (extension might have exposed it)
					if (typeof LanguageModel !== 'undefined') {
						console.log('Direct LanguageModel access available');
						const availability = await LanguageModel.availability();
						resolve({ method: 'direct_LanguageModel', availability: availability });
						return;
					}
					
					if (typeof window.ai !== 'undefined') {
						console.log('Direct window.ai access available');
						const capabilities = await window.ai.capabilities();
						resolve({ method: 'direct_window_ai', capabilities: capabilities });
						return;
					}
					
					// Method 2: Try extension communication
					if (typeof chrome !== 'undefined' && chrome.runtime) {
						console.log('Trying extension communication...');
						chrome.runtime.sendMessage({
							type: 'GET_AI_STATUS'
						}, (response) => {
							resolve({ method: 'extension_communication', response: response });
						});
						return;
					}
					
					// Method 3: Try postMessage to content script
					console.log('Trying content script communication...');
					window.postMessage({ type: 'CHECK_AI_API' }, '*');
					
					// Listen for response
					const messageHandler = (event) => {
						if (event.source === window && event.data.type === 'AI_API_STATUS_UPDATE') {
							window.removeEventListener('message', messageHandler);
							resolve({ method: 'content_script', status: event.data.status });
						}
					};
					
					window.addEventListener('message', messageHandler);
					
					// Timeout after 5 seconds
					setTimeout(() => {
						window.removeEventListener('message', messageHandler);
						resolve({ method: 'timeout', error: 'No response from extension' });
					}, 5000);
					
				} catch (error) {
					resolve({ error: error.message });
				}
			})
		`
		return chromedp.Evaluate(script, &testResult).Do(ctx)
	}))

	if err != nil {
		fmt.Printf("Extension AI test error: %v\n", err)
	} else {
		testJSON, _ := json.MarshalIndent(testResult, "", "  ")
		fmt.Printf("🚀 Extension AI Test Result:\n%s\n", testJSON)
	}
}
