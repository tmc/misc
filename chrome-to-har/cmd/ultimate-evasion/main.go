// Ultimate evasion test - Maximum stealth with timing attacks
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
	fmt.Println("=== Ultimate Evasion Test ===")
	
	// Phase 1: Launch Chrome with a delay to appear more natural
	fmt.Println("Phase 1: Natural Chrome launch simulation...")
	
	exec.Command("pkill", "-f", "remote-debugging-port=9229").Run()
	time.Sleep(3 * time.Second) // Natural delay
	
	cmd := exec.Command(
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"--remote-debugging-port=9229",
		"--user-data-dir=/tmp/chrome-ultimate-test",
		"--no-first-run",
		"--no-default-browser-check",
		
		// AI flags
		"--enable-features=Gemini,AILanguageModelService,BuiltInAIAPI",
		"--optimization-guide-on-device-model=enabled", 
		"--prompt-api-for-gemini-nano=enabled",
		
		// Maximum evasion
		"--disable-blink-features=AutomationControlled",
		"--exclude-switches=enable-automation",
		"--disable-dev-shm-usage",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows", 
		"--disable-renderer-backgrounding",
		"--disable-features=TranslateUI,BlinkGenPropertyTrees",
		"--disable-ipc-flooding-protection",
		"--password-store=basic",
		"--use-mock-keychain",
		
		// Appear more like regular Chrome
		"--disable-extensions-except=/System/Library/Extensions",
		"--load-extension=/System/Library/Extensions", // Dummy path
		"--start-maximized",
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start Chrome: %v", err)
	}
	
	// Phase 2: Wait with realistic timing
	fmt.Println("Phase 2: Waiting for natural Chrome startup...")
	time.Sleep(12 * time.Second) // Longer natural delay
	
	// Phase 3: Connect with timing simulation
	fmt.Println("Phase 3: Establishing connection with natural timing...")
	
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://localhost:9229")
	defer cancel()
	
	// Simulate human-like connection timing
	time.Sleep(2 * time.Second)
	
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	
	// Phase 4: Multi-stage stealth application
	fmt.Println("Phase 4: Applying multi-stage stealth techniques...")
	
	// Stage 1: Basic navigation with delay
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Sleep(3*time.Second), // Human-like pause
	)
	
	if err != nil {
		log.Printf("Stage 1 error: %v", err)
		return
	}
	
	// Stage 2: Advanced stealth JavaScript injection
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Inject comprehensive stealth script
			script := `
				// Advanced webdriver removal
				Object.defineProperty(navigator, 'webdriver', {
					get: () => undefined,
					configurable: true
				});
				
				// Remove all automation indicators
				delete window.navigator.webdriver;
				delete window.callPhantom;
				delete window._phantom;
				delete window.__nightmare;
				delete window._selenium;
				delete window.webdriver;
				delete window.driver;
				delete window.domAutomation;
				delete window.domAutomationController;
				delete window.selenium;
				delete window.webdriver;
				delete window.fminer_selenium;
				
				// Override plugin detection
				Object.defineProperty(navigator, 'plugins', {
					get: () => new Array(5).fill().map((_, i) => ({ name: 'Plugin ' + i })),
				});
				
				// Spoof language preferences
				Object.defineProperty(navigator, 'languages', {
					get: () => ['en-US', 'en'],
				});
				
				// Override permissions
				const originalQuery = window.navigator.permissions.query;
				window.navigator.permissions.query = (parameters) => (
					parameters.name === 'notifications' ?
						Promise.resolve({ state: Notification.permission }) :
						originalQuery(parameters)
				);
				
				// Remove CDP indicators
				if (window.chrome && window.chrome.runtime) {
					delete window.chrome.runtime.onConnect;
					delete window.chrome.runtime.onMessage;
				}
				
				// Override timing to appear human
				const originalSetTimeout = window.setTimeout;
				window.setTimeout = function(fn, delay) {
					const humanDelay = delay + Math.random() * 50; // Add jitter
					return originalSetTimeout(fn, humanDelay);
				};
				
				console.log('Advanced stealth applied');
			`
			
			var result interface{}
			return chromedp.Evaluate(script, &result).Do(ctx)
		}),
		chromedp.Sleep(2*time.Second), // Let stealth settle
	)
	
	if err != nil {
		log.Printf("Stage 2 error: %v", err)
		return
	}
	
	// Stage 3: Human-like interaction simulation
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Simulate human interactions
			script := `
				// Simulate mouse movements
				document.dispatchEvent(new MouseEvent('mousemove', {
					clientX: Math.random() * window.innerWidth,
					clientY: Math.random() * window.innerHeight
				}));
				
				// Simulate scroll
				window.scrollTo(0, Math.random() * 100);
				
				// Simulate keyboard activity
				document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab' }));
				
				'human_simulation_complete'
			`
			
			var result interface{}
			return chromedp.Evaluate(script, &result).Do(ctx)
		}),
		chromedp.Sleep(1*time.Second),
	)
	
	if err != nil {
		log.Printf("Stage 3 error: %v", err)
		return
	}
	
	// Phase 5: Ultimate AI API test
	fmt.Println("Phase 5: Testing AI API with ultimate evasion...")
	
	var finalResult map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			({
				// Environment check
				timestamp: Date.now(),
				userAgent: navigator.userAgent,
				webdriverUndefined: typeof navigator.webdriver === 'undefined',
				automationFlags: {
					webdriver: navigator.webdriver,
					plugins: navigator.plugins.length,
					languages: navigator.languages.length,
					chromeRuntime: typeof chrome !== 'undefined' && typeof chrome.runtime !== 'undefined'
				},
				
				// AI API detection
				aiAPIs: {
					languageModel: typeof LanguageModel !== 'undefined',
					windowAi: typeof window.ai !== 'undefined',
					chromeAi: typeof chrome !== 'undefined' && typeof chrome.ai !== 'undefined'
				},
				
				// Security context
				security: {
					isSecureContext: window.isSecureContext,
					origin: window.location.origin,
					protocol: window.location.protocol
				},
				
				// Comprehensive global search
				suspiciousGlobals: Object.getOwnPropertyNames(window).filter(name => 
					name.includes('selenium') || 
					name.includes('webdriver') || 
					name.includes('phantom') || 
					name.includes('nightmare') ||
					name.includes('automation')
				),
				
				aiGlobals: Object.getOwnPropertyNames(window).filter(name => 
					name.toLowerCase().includes('ai') || 
					name.toLowerCase().includes('language') ||
					name.toLowerCase().includes('gemini') ||
					name.toLowerCase().includes('prompt')
				).slice(0, 25)
			})
		`, &finalResult),
	)
	
	if err != nil {
		log.Printf("Final test error: %v", err)
		return
	}
	
	resultJSON, _ := json.MarshalIndent(finalResult, "", "  ")
	fmt.Printf("🔬 Ultimate Evasion Result:\n%s\n", resultJSON)
	
	// Analyze results
	fmt.Println("\n📊 Analysis:")
	
	if automationFlags, ok := finalResult["automationFlags"].(map[string]interface{}); ok {
		if webdriver := automationFlags["webdriver"]; webdriver == nil {
			fmt.Println("✅ Webdriver successfully hidden")
		} else {
			fmt.Println("❌ Webdriver still detected")
		}
		
		if plugins, ok := automationFlags["plugins"].(float64); ok && plugins > 0 {
			fmt.Println("✅ Plugin fingerprint spoofed")
		}
		
		if chromeRuntime, ok := automationFlags["chromeRuntime"].(bool); ok && !chromeRuntime {
			fmt.Println("✅ Chrome runtime indicators removed")
		}
	}
	
	if aiAPIs, ok := finalResult["aiAPIs"].(map[string]interface{}); ok {
		hasAnyAPI := false
		for api, exists := range aiAPIs {
			if val, ok := exists.(bool); ok && val {
				fmt.Printf("🎉 %s API detected!\n", api)
				hasAnyAPI = true
			}
		}
		
		if hasAnyAPI {
			fmt.Println("\n🚀 Testing ultimate stealth AI functionality...")
			testUltimateAI(ctx)
		} else {
			fmt.Println("\n❌ No AI APIs detected even with ultimate evasion")
			fmt.Println("This confirms the restriction is at a deeper level than automation detection")
		}
	}
	
	// Cleanup
	fmt.Println("\nCleaning up...")
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill Chrome: %v", err)
	}
}

func testUltimateAI(ctx context.Context) {
	var result interface{}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			new Promise(async (resolve) => {
				try {
					// Add human-like delay before API call
					await new Promise(r => setTimeout(r, 1000 + Math.random() * 2000));
					
					if (typeof LanguageModel !== 'undefined') {
						console.log('Testing LanguageModel API...');
						const availability = await LanguageModel.availability();
						console.log('Availability:', availability);
						
						if (availability === 'available' || availability === 'downloadable') {
							const model = await LanguageModel.create();
							const response = await model.generate('Ultimate test successful!');
							resolve({ 
								success: true, 
								api: 'LanguageModel', 
								availability, 
								response,
								timestamp: Date.now()
							});
						} else {
							resolve({ api: 'LanguageModel', availability, downloadNeeded: true });
						}
					} else if (typeof window.ai !== 'undefined') {
						console.log('Testing window.ai API...');
						const capabilities = await window.ai.capabilities();
						console.log('Capabilities:', capabilities);
						
						const session = await window.ai.createTextSession();
						const response = await session.prompt('Ultimate test successful!');
						resolve({ 
							success: true, 
							api: 'window.ai', 
							capabilities, 
							response,
							timestamp: Date.now()
						});
					} else {
						resolve({ error: 'No AI API available after ultimate evasion' });
					}
				} catch (error) {
					resolve({ 
						error: error.message, 
						stack: error.stack,
						timestamp: Date.now()
					});
				}
			})
		`
		return chromedp.Evaluate(script, &result).Do(ctx)
	}))
	
	if err != nil {
		fmt.Printf("Ultimate AI test error: %v\n", err)
	} else {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("🏆 Ultimate AI Test Result:\n%s\n", resultJSON)
	}
}