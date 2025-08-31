// AI Takeover - Connect to user-initiated Chrome with AI APIs
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	fmt.Println("=== AI Takeover Mode ===")
	fmt.Println()
	fmt.Println("This approach connects to Chrome that YOU launch manually.")
	fmt.Println("This bypasses automation detection since YOU started Chrome.")
	fmt.Println()
	fmt.Println("Step 1: Launch Chrome manually with these commands:")
	fmt.Println()
	fmt.Println("# Open Terminal and run:")
	fmt.Println(`"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" \`)
	fmt.Println(`  --remote-debugging-port=9227 \`)
	fmt.Println(`  --enable-features=Gemini,AILanguageModelService,BuiltInAIAPI \`)
	fmt.Println(`  --enable-ai-language-model-service \`)
	fmt.Println(`  --optimization-guide-on-device-model=enabled \`)
	fmt.Println(`  --prompt-api-for-gemini-nano=enabled \`)
	fmt.Println(`  --prompt-api-for-gemini-nano-multimodal-input=enabled`)
	fmt.Println()
	fmt.Println("Step 2: In Chrome, navigate to any website (e.g., https://example.com)")
	fmt.Println("Step 3: Press Enter here when Chrome is ready...")

	// Wait for user
	bufio.NewScanner(os.Stdin).Scan()

	fmt.Println("Attempting to connect to your Chrome session...")

	// Connect to user's Chrome
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9227")
	defer cancel()

	// Get list of available targets
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		log.Fatalf("Failed to get targets: %v", err)
	}

	fmt.Printf("Found %d Chrome targets:\n", len(targets))
	for i, target := range targets {
		fmt.Printf("  %d: %s (%s)\n", i, target.Title, target.URL)
	}

	// Connect to first available page target
	// NOTE: chromedp API has changed, this needs to be updated
	// var pageTarget *chromedp.Target
	for i, target := range targets {
		if target.Type == "page" && !strings.Contains(target.URL, "chrome://") {
			fmt.Printf("Would connect to target %d: %s (%s)\n", i, target.Title, target.URL)
			break
		}
	}

	fmt.Println("AI takeover functionality needs chromedp API update")

	// Create context for the specific target
	// NOTE: Commented out due to chromedp API changes
	// ctx, cancel = chromedp.NewContext(ctx, chromedp.WithTargetID(pageTarget.TargetID))
	// defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Test AI API access
	fmt.Println("Testing AI API access in user-initiated Chrome...")

	var result map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`
			({
				// Page info
				title: document.title,
				url: window.location.href,
				userAgent: navigator.userAgent,
				
				// AI API detection
				languageModelExists: typeof LanguageModel !== 'undefined',
				windowAiExists: typeof window.ai !== 'undefined',
				chromeAiExists: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined',
				
				// Security context
				isSecureContext: window.isSecureContext,
				origin: window.location.origin,
				
				// Check for any automation detection
				webdriver: navigator.webdriver,
				automationDetected: window.navigator.webdriver || 
				                   window.chrome && window.chrome.runtime && window.chrome.runtime.onConnect,
				
				// Global AI search
				aiGlobals: Object.getOwnPropertyNames(window).filter(name => 
					name.toLowerCase().includes('ai') || 
					name.toLowerCase().includes('language') ||
					name.toLowerCase().includes('gemini')
				).slice(0, 15)
			})
		`, &result),
	)

	if err != nil {
		log.Fatalf("Error testing AI API: %v", err)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("User-Initiated Chrome Test Result:\n%s\n", resultJSON)

	// If AI API is found, test it
	if hasAI(result) {
		fmt.Println("\n🎉 AI API detected! Testing functionality...")
		testAIFunctionality(ctx)
	} else {
		fmt.Println("\n❌ No AI API found even in user-initiated Chrome")
		fmt.Println("This suggests the API may not be available in this Chrome version")
		fmt.Println("or the required chrome://flags may not be set.")

		// Provide troubleshooting
		fmt.Println("\nTroubleshooting:")
		fmt.Println("1. Check chrome://flags/#prompt-api-for-gemini-nano")
		fmt.Println("2. Check chrome://flags/#optimization-guide-on-device-model")
		fmt.Println("3. Restart Chrome after enabling flags")
		fmt.Println("4. Check chrome://components/ for 'Optimization Guide On Device Model'")
	}
}

func hasAI(result map[string]interface{}) bool {
	if lm, ok := result["languageModelExists"].(bool); ok && lm {
		return true
	}
	if wa, ok := result["windowAiExists"].(bool); ok && wa {
		return true
	}
	if ca, ok := result["chromeAiExists"].(bool); ok && ca {
		return true
	}
	return false
}

func testAIFunctionality(ctx context.Context) {
	// Test API availability
	fmt.Println("Testing API availability...")

	var availability interface{}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			new Promise(async (resolve) => {
				try {
					if (typeof LanguageModel !== 'undefined') {
						const availability = await LanguageModel.availability();
						resolve({ api: 'LanguageModel', status: availability });
					} else if (typeof window.ai !== 'undefined') {
						const capabilities = await window.ai.capabilities();
						resolve({ api: 'window.ai', capabilities: capabilities });
					} else if (typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined') {
						resolve({ api: 'chrome.ai', status: 'detected' });
					} else {
						resolve({ error: 'No API available' });
					}
				} catch (error) {
					resolve({ error: error.message });
				}
			})
		`
		return chromedp.Evaluate(script, &availability).Do(ctx)
	}))

	if err != nil {
		fmt.Printf("Availability test error: %v\n", err)
		return
	}

	availJSON, _ := json.MarshalIndent(availability, "", "  ")
	fmt.Printf("Availability result:\n%s\n", availJSON)

	// If available, test text generation
	if !strings.Contains(fmt.Sprintf("%v", availability), "error") {
		fmt.Println("Testing text generation...")

		var genResult interface{}
		err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			script := `
				new Promise(async (resolve) => {
					try {
						if (typeof LanguageModel !== 'undefined') {
							const model = await LanguageModel.create();
							const response = await model.generate('Hello, how are you?');
							resolve({ success: true, response: response });
						} else if (typeof window.ai !== 'undefined') {
							const session = await window.ai.createTextSession();
							const response = await session.prompt('Hello, how are you?');
							resolve({ success: true, response: response });
						} else {
							resolve({ error: 'No generation API available' });
						}
					} catch (error) {
						resolve({ error: error.message });
					}
				})
			`
			return chromedp.Evaluate(script, &genResult).Do(ctx)
		}))

		if err != nil {
			fmt.Printf("Generation test error: %v\n", err)
		} else {
			genJSON, _ := json.MarshalIndent(genResult, "", "  ")
			fmt.Printf("🚀 Generation result:\n%s\n", genJSON)
		}
	}
}
