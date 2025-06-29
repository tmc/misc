// Chrome AI Setup Verification Tool
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	fmt.Println("=== Chrome AI Setup Verification ===")
	fmt.Println("This tool verifies Chrome AI flags and model availability")
	fmt.Println()

	// Step 1: Check Chrome version
	fmt.Println("Step 1: Checking Chrome Canary version...")
	checkChromeVersion()

	// Step 2: Launch Chrome with AI flags
	fmt.Println("\nStep 2: Launching Chrome with AI flags...")
	cmd := launchChromeWithAIFlags()
	defer func() {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Step 3: Check chrome://flags setup
	fmt.Println("\nStep 3: Checking chrome://flags configuration...")
	checkChromeFlags()

	// Step 4: Check chrome://components for AI model
	fmt.Println("\nStep 4: Checking chrome://components for AI model...")
	checkChromeComponents()

	// Step 5: Test API availability in clean context
	fmt.Println("\nStep 5: Testing AI API availability...")
	testAIAvailability()

	fmt.Println("\n=== Setup Verification Complete ===")
}

func checkChromeVersion() {
	cmd := exec.Command("/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary", "--version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ Chrome Canary not found: %v\n", err)
		return
	}
	
	version := strings.TrimSpace(string(output))
	fmt.Printf("✅ Found: %s\n", version)
	
	// Extract version number
	if strings.Contains(version, "139.") || strings.Contains(version, "140.") {
		fmt.Println("✅ Version supports AI APIs")
	} else {
		fmt.Println("⚠️  Version may not support AI APIs (need 139+)")
	}
}

func launchChromeWithAIFlags() *exec.Cmd {
	// Kill existing Chrome
	exec.Command("pkill", "-f", "remote-debugging-port=9231").Run()
	time.Sleep(2 * time.Second)

	cmd := exec.Command(
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"--remote-debugging-port=9231",
		"--user-data-dir=/tmp/chrome-ai-setup-check",
		"--no-first-run",
		"--no-default-browser-check",
		// Essential AI flags
		"--enable-features=Gemini,AILanguageModelService,BuiltInAIAPI",
		"--enable-ai-language-model-service",
		"--optimization-guide-on-device-model=enabled",
		"--prompt-api-for-gemini-nano=enabled",
		"--prompt-api-for-gemini-nano-multimodal-input=enabled",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start Chrome: %v", err)
		return nil
	}

	fmt.Printf("✅ Chrome launched with PID: %d\n", cmd.Process.Pid)
	fmt.Println("⏳ Waiting for Chrome to initialize...")
	time.Sleep(10 * time.Second)

	return cmd
}

func checkChromeFlags() {
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9231")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var flagsContent string
	err := chromedp.Run(ctx,
		chromedp.Navigate("chrome://flags/"),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.body.innerText`, &flagsContent),
	)

	if err != nil {
		fmt.Printf("❌ Failed to check flags: %v\n", err)
		return
	}

	// Check for key flags
	flags := map[string]string{
		"prompt-api-for-gemini-nano":         "Prompt API for Gemini Nano",
		"optimization-guide-on-device-model": "Optimization Guide On Device Model",
	}

	for flagName, description := range flags {
		if strings.Contains(strings.ToLower(flagsContent), flagName) {
			fmt.Printf("✅ Found flag: %s\n", description)
		} else {
			fmt.Printf("❌ Missing flag: %s\n", description)
		}
	}
}

func checkChromeComponents() {
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9231")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var componentsContent string
	err := chromedp.Run(ctx,
		chromedp.Navigate("chrome://components/"),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`document.body.innerText`, &componentsContent),
	)

	if err != nil {
		fmt.Printf("❌ Failed to check components: %v\n", err)
		return
	}

	// Check for AI model component
	if strings.Contains(strings.ToLower(componentsContent), "optimization guide") {
		fmt.Println("✅ Found Optimization Guide component")
		
		// Check if it's downloaded
		if strings.Contains(componentsContent, "0.0.0.0") {
			fmt.Println("⚠️  AI model not downloaded (version 0.0.0.0)")
			fmt.Println("   Click 'Check for update' on Optimization Guide component")
		} else {
			fmt.Println("✅ AI model appears to be downloaded")
		}
	} else {
		fmt.Println("❌ Optimization Guide component not found")
	}
}

func testAIAvailability() {
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9231")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var result map[string]interface{}
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			({
				// Check for AI APIs
				languageModelExists: typeof LanguageModel !== 'undefined',
				windowAiExists: typeof window.ai !== 'undefined',
				chromeAiExists: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined',
				
				// Environment info
				userAgent: navigator.userAgent,
				isSecureContext: window.isSecureContext,
				
				// Check for any experimental features
				experimentalFeatures: Object.getOwnPropertyNames(window).filter(name => 
					name.toLowerCase().includes('ai') || 
					name.toLowerCase().includes('language') ||
					name.toLowerCase().includes('gemini') ||
					name.toLowerCase().includes('experimental')
				).slice(0, 20)
			})
		`, &result),
	)

	if err != nil {
		fmt.Printf("❌ Failed to test API availability: %v\n", err)
		return
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("API Availability Test:\n%s\n", resultJSON)

	// Analyze results
	if hasAnyAPI(result) {
		fmt.Println("✅ AI APIs detected in programmatic context!")
		testAPIFunctionality(ctx)
	} else {
		fmt.Println("❌ No AI APIs detected in programmatic context")
		fmt.Println("This confirms the DevTools protocol restriction")
	}
}

func hasAnyAPI(result map[string]interface{}) bool {
	checks := []string{"languageModelExists", "windowAiExists", "chromeAiExists"}
	for _, check := range checks {
		if val, ok := result[check].(bool); ok && val {
			return true
		}
	}
	return false
}

func testAPIFunctionality(ctx context.Context) {
	fmt.Println("🚀 Testing AI API functionality...")

	var testResult interface{}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			new Promise(async (resolve) => {
				try {
					if (typeof LanguageModel !== 'undefined') {
						const availability = await LanguageModel.availability();
						resolve({ api: 'LanguageModel', availability: availability });
					} else if (typeof window.ai !== 'undefined') {
						const capabilities = await window.ai.capabilities();
						resolve({ api: 'window.ai', capabilities: capabilities });
					} else {
						resolve({ error: 'No API available for testing' });
					}
				} catch (error) {
					resolve({ error: error.message });
				}
			})
		`
		return chromedp.Evaluate(script, &testResult).Do(ctx)
	}))

	if err != nil {
		fmt.Printf("❌ API functionality test failed: %v\n", err)
	} else {
		testJSON, _ := json.MarshalIndent(testResult, "", "  ")
		fmt.Printf("🎯 API Functionality Test:\n%s\n", testJSON)
	}
}