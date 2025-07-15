package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestStabilityDetector(t *testing.T) {
	// Create a test server with dynamic content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Stability Test</title>
    <style>
        body { font-family: Arial, sans-serif; }
        #content { margin: 20px; }
        .loaded { color: green; }
    </style>
</head>
<body>
    <div id="content">
        <h1>Page Loading Test</h1>
        <div id="status">Loading...</div>
        <img id="test-image" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" alt="Test">
    </div>
    
    <script>
        // Simulate network activity
        let requestCount = 0;
        
        function makeRequest() {
            requestCount++;
            fetch('/api/test' + requestCount)
                .then(response => response.text())
                .catch(() => {})
                .finally(() => {
                    if (requestCount < 3) {
                        setTimeout(makeRequest, 200);
                    } else {
                        document.getElementById('status').textContent = 'All requests completed';
                        document.getElementById('status').className = 'loaded';
                        
                        // Simulate DOM mutations
                        setTimeout(() => {
                            const div = document.createElement('div');
                            div.textContent = 'Dynamic content added';
                            document.getElementById('content').appendChild(div);
                        }, 100);
                    }
                });
        }
        
        // Start the simulation after page load
        setTimeout(makeRequest, 100);
    </script>
</body>
</html>
		`))
	}))
	defer server.Close()

	// Handle API requests
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Stability Test</title>
    <style>
        body { font-family: Arial, sans-serif; }
        #content { margin: 20px; }
        .loaded { color: green; }
    </style>
</head>
<body>
    <div id="content">
        <h1>Page Loading Test</h1>
        <div id="status">Loading...</div>
        <img id="test-image" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" alt="Test">
    </div>
    
    <script>
        // Simulate network activity
        let requestCount = 0;
        
        function makeRequest() {
            requestCount++;
            fetch('/api/test' + requestCount)
                .then(response => response.text())
                .catch(() => {})
                .finally(() => {
                    if (requestCount < 3) {
                        setTimeout(makeRequest, 200);
                    } else {
                        document.getElementById('status').textContent = 'All requests completed';
                        document.getElementById('status').className = 'loaded';
                        
                        // Simulate DOM mutations
                        setTimeout(() => {
                            const div = document.createElement('div');
                            div.textContent = 'Dynamic content added';
                            document.getElementById('content').appendChild(div);
                        }, 100);
                    }
                });
        }
        
        // Start the simulation after page load
        setTimeout(makeRequest, 100);
    </script>
</body>
</html>
			`))
		} else {
			// API endpoints
			time.Sleep(100 * time.Millisecond) // Simulate processing time
			w.Write([]byte("OK"))
		}
	})

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Create a page
	page := &Page{
		ctx: ctx,
	}

	// Navigate to test page
	if err := chromedp.Run(ctx, chromedp.Navigate(server.URL)); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	t.Run("DefaultStabilityConfig", func(t *testing.T) {
		config := DefaultStabilityConfig()
		
		// Test default values
		if config.NetworkIdleThreshold != 0 {
			t.Errorf("Expected NetworkIdleThreshold 0, got %d", config.NetworkIdleThreshold)
		}
		
		if config.NetworkIdleTimeout != 500*time.Millisecond {
			t.Errorf("Expected NetworkIdleTimeout 500ms, got %v", config.NetworkIdleTimeout)
		}
		
		if !config.WaitForImages {
			t.Error("Expected WaitForImages to be true")
		}
	})

	t.Run("StabilityDetectorCreation", func(t *testing.T) {
		config := DefaultStabilityConfig()
		detector := NewStabilityDetector(page, config)
		
		if detector == nil {
			t.Fatal("Expected non-nil detector")
		}
		
		if detector.page != page {
			t.Error("Expected detector to reference the page")
		}
		
		if detector.config != config {
			t.Error("Expected detector to use the provided config")
		}
	})

	t.Run("NetworkIdleDetection", func(t *testing.T) {
		config := DefaultStabilityConfig()
		config.NetworkIdleThreshold = 0
		config.NetworkIdleTimeout = 300 * time.Millisecond
		config.MaxStabilityWait = 5 * time.Second
		config.Verbose = true
		
		// Disable other checks to focus on network idle
		config.WaitForImages = false
		config.WaitForFonts = false
		config.WaitForStylesheets = false
		config.WaitForScripts = false
		config.WaitForAnimationFrame = false
		config.WaitForIdleCallback = false
		config.DOMStableThreshold = -1 // Disable DOM check
		
		detector := NewStabilityDetector(page, config)
		
		testCtx, testCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer testCancel()
		
		start := time.Now()
		err := detector.WaitForStability(testCtx)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("Network idle detection failed: %v", err)
		}
		
		t.Logf("Network idle detection took %v", duration)
		
		// Should take at least the network idle timeout
		if duration < config.NetworkIdleTimeout {
			t.Errorf("Expected at least %v, got %v", config.NetworkIdleTimeout, duration)
		}
	})

	t.Run("ResourceLoadingDetection", func(t *testing.T) {
		config := DefaultStabilityConfig()
		config.WaitForImages = true
		config.WaitForFonts = false
		config.WaitForStylesheets = true
		config.WaitForScripts = false
		config.ResourceTimeout = 3 * time.Second
		config.MaxStabilityWait = 5 * time.Second
		config.Verbose = true
		
		// Disable other checks
		config.NetworkIdleThreshold = -1 // Disable network check
		config.DOMStableThreshold = -1   // Disable DOM check
		config.WaitForAnimationFrame = false
		config.WaitForIdleCallback = false
		
		detector := NewStabilityDetector(page, config)
		
		testCtx, testCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer testCancel()
		
		start := time.Now()
		err := detector.WaitForStability(testCtx)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("Resource loading detection failed: %v", err)
		}
		
		t.Logf("Resource loading detection took %v", duration)
	})

	t.Run("CustomStabilityCheck", func(t *testing.T) {
		config := DefaultStabilityConfig()
		config.MaxStabilityWait = 5 * time.Second
		config.Verbose = true
		
		// Disable other checks
		config.NetworkIdleThreshold = -1
		config.DOMStableThreshold = -1
		config.WaitForImages = false
		config.WaitForFonts = false
		config.WaitForStylesheets = false
		config.WaitForScripts = false
		config.WaitForAnimationFrame = false
		config.WaitForIdleCallback = false
		
		// Add custom check
		config.CustomChecks = []StabilityCheck{
			{
				Name:       "status-loaded",
				Expression: `document.getElementById('status').className === 'loaded'`,
				Timeout:    3 * time.Second,
			},
		}
		
		detector := NewStabilityDetector(page, config)
		
		testCtx, testCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer testCancel()
		
		start := time.Now()
		err := detector.WaitForStability(testCtx)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("Custom stability check failed: %v", err)
		}
		
		t.Logf("Custom stability check took %v", duration)
		
		// Verify the custom check was actually satisfied
		var statusClass string
		if err := chromedp.Run(ctx, chromedp.AttributeValue("#status", "className", &statusClass, nil)); err != nil {
			t.Errorf("Failed to get status class: %v", err)
		}
		
		if statusClass != "loaded" {
			t.Errorf("Expected status class 'loaded', got '%s'", statusClass)
		}
	})

	t.Run("StabilityOptions", func(t *testing.T) {
		config := DefaultStabilityConfig()
		
		// Test option functions
		WithNetworkIdleThreshold(2)(config)
		if config.NetworkIdleThreshold != 2 {
			t.Errorf("Expected NetworkIdleThreshold 2, got %d", config.NetworkIdleThreshold)
		}
		
		WithNetworkIdleTimeout(1 * time.Second)(config)
		if config.NetworkIdleTimeout != 1*time.Second {
			t.Errorf("Expected NetworkIdleTimeout 1s, got %v", config.NetworkIdleTimeout)
		}
		
		WithDOMStableTimeout(2 * time.Second)(config)
		if config.DOMStableTimeout != 2*time.Second {
			t.Errorf("Expected DOMStableTimeout 2s, got %v", config.DOMStableTimeout)
		}
		
		WithResourceWaiting(false, true, false, true)(config)
		if config.WaitForImages != false {
			t.Error("Expected WaitForImages to be false")
		}
		if config.WaitForFonts != true {
			t.Error("Expected WaitForFonts to be true")
		}
		if config.WaitForStylesheets != false {
			t.Error("Expected WaitForStylesheets to be false")
		}
		if config.WaitForScripts != true {
			t.Error("Expected WaitForScripts to be true")
		}
		
		WithMaxStabilityWait(10 * time.Second)(config)
		if config.MaxStabilityWait != 10*time.Second {
			t.Errorf("Expected MaxStabilityWait 10s, got %v", config.MaxStabilityWait)
		}
		
		WithCustomCheck("test", "true", 1*time.Second)(config)
		if len(config.CustomChecks) != 1 {
			t.Errorf("Expected 1 custom check, got %d", len(config.CustomChecks))
		}
		
		WithVerboseLogging(true)(config)
		if !config.Verbose {
			t.Error("Expected verbose logging to be enabled")
		}
	})

	t.Run("StabilityMetrics", func(t *testing.T) {
		config := DefaultStabilityConfig()
		config.Verbose = true
		
		detector := NewStabilityDetector(page, config)
		
		// Get initial metrics
		metrics := detector.GetMetrics()
		
		if metrics.NetworkRequests < 0 {
			t.Error("Expected non-negative network requests")
		}
		
		if metrics.PendingRequests == nil {
			t.Error("Expected non-nil pending requests map")
		}
		
		if metrics.LoadedResources == nil {
			t.Error("Expected non-nil loaded resources map")
		}
		
		if metrics.StabilityChecks == nil {
			t.Error("Expected non-nil stability checks map")
		}
	})

	t.Run("StabilityTimeout", func(t *testing.T) {
		config := DefaultStabilityConfig()
		config.MaxStabilityWait = 100 * time.Millisecond // Very short timeout
		config.Verbose = true
		
		// Set impossible conditions
		config.CustomChecks = []StabilityCheck{
			{
				Name:       "impossible",
				Expression: `false`, // Always fails
				Timeout:    1 * time.Second,
			},
		}
		
		detector := NewStabilityDetector(page, config)
		
		testCtx, testCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer testCancel()
		
		start := time.Now()
		err := detector.WaitForStability(testCtx)
		duration := time.Since(start)
		
		if err == nil {
			t.Error("Expected stability detection to fail due to timeout")
		}
		
		t.Logf("Stability timeout took %v (expected around 100ms)", duration)
	})
}

func TestStabilityConfigValidation(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		page := &Page{}
		detector := NewStabilityDetector(page, nil)
		
		if detector.config == nil {
			t.Error("Expected detector to create default config when nil is provided")
		}
	})

	t.Run("ValidConfig", func(t *testing.T) {
		config := &StabilityConfig{
			NetworkIdleThreshold: 1,
			NetworkIdleTimeout:   1 * time.Second,
			MaxStabilityWait:     30 * time.Second,
		}
		
		page := &Page{}
		detector := NewStabilityDetector(page, config)
		
		if detector.config.NetworkIdleThreshold != 1 {
			t.Errorf("Expected NetworkIdleThreshold 1, got %d", detector.config.NetworkIdleThreshold)
		}
		
		if detector.config.NetworkIdleTimeout != 1*time.Second {
			t.Errorf("Expected NetworkIdleTimeout 1s, got %v", detector.config.NetworkIdleTimeout)
		}
	})
}

func TestStabilityDetectorLifecycle(t *testing.T) {
	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	page := &Page{ctx: ctx}
	config := DefaultStabilityConfig()
	detector := NewStabilityDetector(page, config)

	t.Run("StartStop", func(t *testing.T) {
		// Start detector
		if err := detector.Start(); err != nil {
			t.Errorf("Failed to start detector: %v", err)
		}
		
		if !detector.started {
			t.Error("Expected detector to be started")
		}
		
		// Stop detector
		detector.Stop()
		
		if detector.started {
			t.Error("Expected detector to be stopped")
		}
	})

	t.Run("MultipleStarts", func(t *testing.T) {
		// Starting multiple times should be safe
		if err := detector.Start(); err != nil {
			t.Errorf("Failed to start detector: %v", err)
		}
		
		if err := detector.Start(); err != nil {
			t.Errorf("Failed to start detector again: %v", err)
		}
		
		detector.Stop()
	})
}