// Stealth mode test - Chrome with maximum evasion
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	fmt.Println("=== Stealth Mode AI Access Test ===")
	
	// Create Chrome policy directory and file
	policyDir := "/tmp/chrome-policies/managed"
	err := os.MkdirAll(policyDir, 0755)
	if err != nil {
		log.Printf("Warning: Could not create policy directory: %v", err)
	}
	
	// Create enterprise policy to disable automation warnings
	policyContent := `{
		"CommandLineFlagSecurityWarningsEnabled": false,
		"ExperimentalPoliciesEnabled": true,
		"BrowserAddPersonEnabled": false,
		"AutofillAddressEnabled": false,
		"AutofillCreditCardEnabled": false,
		"PasswordManagerEnabled": false,
		"SafeBrowsingEnabled": false,
		"MetricsReportingEnabled": false,
		"SpellcheckEnabled": false,
		"TranslateEnabled": false,
		"DefaultSearchProviderEnabled": false
	}`
	
	policyFile := policyDir + "/managed_policies.json"
	err = os.WriteFile(policyFile, []byte(policyContent), 0644)
	if err != nil {
		log.Printf("Warning: Could not write policy file: %v", err)
	}
	
	fmt.Printf("Created policy file at: %s\n", policyFile)
	
	// Kill existing Chrome processes
	exec.Command("pkill", "-f", "remote-debugging-port=9228").Run()
	time.Sleep(2 * time.Second)
	
	// Launch Chrome with maximum stealth flags
	fmt.Println("Launching Chrome with stealth configuration...")
	
	cmd := exec.Command(
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"--remote-debugging-port=9228",
		"--user-data-dir=/tmp/chrome-stealth-test",
		"--no-first-run",
		"--no-default-browser-check",
		
		// AI API flags
		"--enable-features=Gemini,AILanguageModelService,BuiltInAIAPI",
		"--enable-ai-language-model-service",
		"--optimization-guide-on-device-model=enabled",
		"--prompt-api-for-gemini-nano=enabled",
		"--prompt-api-for-gemini-nano-multimodal-input=enabled",
		
		// Anti-detection flags
		"--disable-blink-features=AutomationControlled",
		"--exclude-switches=enable-automation",
		"--disable-client-side-phishing-detection",
		"--disable-component-extensions-with-background-pages",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-features=TranslateUI",
		"--disable-ipc-flooding-protection",
		"--no-service-autorun",
		"--password-store=basic",
		"--use-mock-keychain",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-field-trial-config",
		
		// Policy flags
		fmt.Sprintf("--policy-directory=%s", policyDir),
		"--force-fieldtrials=*BackgroundTracing/default/",
		
		// User agent spoofing preparation
		"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Regular",
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start Chrome: %v", err)
	}
	
	fmt.Println("Waiting for Chrome to initialize...")
	time.Sleep(8 * time.Second)
	
	// Connect with additional stealth measures
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9228")
	defer cancel()
	
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	
	// Apply stealth JavaScript modifications
	fmt.Println("Applying stealth JavaScript modifications...")
	
	var result map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Sleep(2*time.Second),
		
		// Remove webdriver flag and automation indicators
		chromedp.Evaluate(`
			// Remove webdriver flag
			Object.defineProperty(navigator, 'webdriver', {
				get: () => undefined,
			});
			
			// Remove automation indicators
			delete window.navigator.webdriver;
			delete window.navigator.plugins.namedItem;
			
			// Override automation flags
			Object.defineProperty(navigator, 'plugins', {
				get: () => [1, 2, 3, 4, 5],
			});
			
			// Remove CDP runtime
			delete window.chrome.runtime;
			
			// Override user agent if needed
			Object.defineProperty(navigator, 'userAgent', {
				get: () => 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36',
			});
			
			'stealth_applied'
		`, nil),
		
		chromedp.Sleep(1*time.Second),
		
		// Test AI API availability
		chromedp.Evaluate(`
			({
				// Basic info
				userAgent: navigator.userAgent,
				webdriver: navigator.webdriver,
				automationControlled: window.navigator.webdriver,
				
				// AI API detection
				languageModelExists: typeof LanguageModel !== 'undefined',
				windowAiExists: typeof window.ai !== 'undefined',
				chromeAiExists: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined',
				
				// Detection indicators
				plugins: navigator.plugins.length,
				chromeRuntime: typeof chrome !== 'undefined' && typeof chrome.runtime !== 'undefined',
				
				// Global AI search
				aiGlobals: Object.getOwnPropertyNames(window).filter(name => 
					name.toLowerCase().includes('ai') || 
					name.toLowerCase().includes('language') ||
					name.toLowerCase().includes('gemini') ||
					name.toLowerCase().includes('prompt')
				).slice(0, 20)
			})
		`, &result),
	)
	
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Stealth Test Result:\n%s\n", resultJSON)
		
		// Check if stealth worked
		if webdriver, ok := result["webdriver"]; ok && webdriver != nil {
			fmt.Println("⚠️  Webdriver flag still detected")
		} else {
			fmt.Println("✅ Webdriver flag successfully hidden")
		}
		
		// Test AI APIs if found
		if hasAnyAI(result) {
			fmt.Println("\n🎉 AI API detected with stealth mode!")
			testStealthAI(ctx)
		} else {
			fmt.Println("\n❌ AI API still not accessible with stealth mode")
		}
	}
	
	// Cleanup
	fmt.Println("\nCleaning up...")
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill Chrome: %v", err)
	}
	
	// Remove policy file
	os.Remove(policyFile)
}

func hasAnyAI(result map[string]interface{}) bool {
	checks := []string{"languageModelExists", "windowAiExists", "chromeAiExists"}
	for _, check := range checks {
		if val, ok := result[check].(bool); ok && val {
			return true
		}
	}
	return false
}

func testStealthAI(ctx context.Context) {
	fmt.Println("Testing stealth AI functionality...")
	
	var testResult interface{}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			new Promise(async (resolve) => {
				try {
					if (typeof LanguageModel !== 'undefined') {
						const availability = await LanguageModel.availability();
						if (availability === 'available' || availability === 'downloadable') {
							const model = await LanguageModel.create();
							const response = await model.generate('Hello from stealth mode!');
							resolve({ success: true, api: 'LanguageModel', response: response });
						} else {
							resolve({ api: 'LanguageModel', availability: availability });
						}
					} else if (typeof window.ai !== 'undefined') {
						const capabilities = await window.ai.capabilities();
						const session = await window.ai.createTextSession();
						const response = await session.prompt('Hello from stealth mode!');
						resolve({ success: true, api: 'window.ai', response: response });
					} else {
						resolve({ error: 'No AI API available' });
					}
				} catch (error) {
					resolve({ error: error.message });
				}
			})
		`
		return chromedp.Evaluate(script, &testResult).Do(ctx)
	}))
	
	if err != nil {
		fmt.Printf("Stealth AI test error: %v\n", err)
	} else {
		testJSON, _ := json.MarshalIndent(testResult, "", "  ")
		fmt.Printf("🚀 Stealth AI Test Result:\n%s\n", testJSON)
	}
}