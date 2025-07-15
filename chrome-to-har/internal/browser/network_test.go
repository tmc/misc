package browser_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// TestNetworkRoute tests request routing and interception
func TestNetworkRoute(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Track intercepted requests
	var interceptedURLs []string

	// Set up route to intercept API calls
	err = page.Route(".*\\/api\\/.*", func(req *browser.Request) error {
		interceptedURLs = append(interceptedURLs, req.URL)

		// Continue the request normally
		return req.Continue()
	})
	if err != nil {
		t.Errorf("Failed to set up route: %v", err)
	}

	// Navigate to page that makes API calls
	err = page.Navigate(ts.URL + "/network-test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for network activity
	time.Sleep(1 * time.Second)

	// Check that API call was intercepted
	found := false
	for _, url := range interceptedURLs {
		if strings.Contains(url, "/api/data") {
			found = true
			break
		}
	}

	if !found {
		t.Error("API call was not intercepted")
	}
}

// TestNetworkAbort tests request abortion
func TestNetworkAbort(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Block all image requests
	err = page.Route(".*\\.(png|jpg|jpeg|gif)", func(req *browser.Request) error {
		return req.Abort("failed")
	})
	if err != nil {
		t.Errorf("Failed to set up route: %v", err)
	}

	// Navigate to page with images
	err = page.Navigate(`data:text/html,
		<img id="test-img" src="` + ts.URL + `/test.png" 
		     onerror="this.setAttribute('data-error', 'true')" />
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for image load attempt
	time.Sleep(500 * time.Millisecond)

	// Check if image failed to load
	img, err := page.QuerySelector("#test-img")
	if err != nil {
		t.Fatal(err)
	}

	errorAttr, err := img.GetAttribute("data-error")
	if err != nil {
		t.Fatal(err)
	}

	if errorAttr != "true" {
		t.Error("Image should have failed to load")
	}
}

// TestNetworkFulfill tests custom response fulfillment
func TestNetworkFulfill(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Intercept and fulfill API requests with custom response
	err = page.Route(".*\\/api\\/custom", func(req *browser.Request) error {
		return req.Fulfill(
			browser.WithStatus(200),
			browser.WithContentType("application/json"),
			browser.WithBody([]byte(`{"custom": true, "message": "Intercepted!"}`)),
		)
	})
	if err != nil {
		t.Errorf("Failed to set up route: %v", err)
	}

	// Navigate to page that fetches custom API
	err = page.Navigate(`data:text/html,
		<div id="result">Loading...</div>
		<script>
			fetch('/api/custom')
				.then(r => r.json())
				.then(data => {
					document.getElementById('result').textContent = data.message;
				});
		</script>
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for result
	err = page.WaitForFunction(`document.getElementById('result').textContent !== 'Loading...'`, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	result, err := page.GetText("#result")
	if err != nil {
		t.Fatal(err)
	}

	if result != "Intercepted!" {
		t.Errorf("Unexpected result: got %s, want Intercepted!", result)
	}
}

// TestNetworkModifyRequest tests request modification
func TestNetworkModifyRequest(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Modify requests to add custom header
	err = page.Route(".*", func(req *browser.Request) error {
		headers := req.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["X-Custom-Header"] = "test-value"

		return req.Continue(browser.WithHeaders(headers))
	})
	if err != nil {
		t.Errorf("Failed to set up route: %v", err)
	}

	// Navigate to test server
	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Server should have received the custom header
	// (This would require modifying the test server to track headers)
}

// TestNetworkWaitForRequest tests waiting for specific requests
func TestNetworkWaitForRequest(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Start waiting for request in background
	requestChan := make(chan *browser.Request)
	errChan := make(chan error)

	go func() {
		req, err := page.WaitForRequest(".*\\/api\\/data", 5000)
		if err != nil {
			errChan <- err
			return
		}
		requestChan <- req
	}()

	// Navigate to page that makes API call
	err = page.Navigate(ts.URL + "/network-test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for request or error
	select {
	case req := <-requestChan:
		if !strings.Contains(req.URL, "/api/data") {
			t.Errorf("Wrong request captured: %s", req.URL)
		}
		if req.Method != "GET" {
			t.Errorf("Wrong method: got %s, want GET", req.Method)
		}
	case err := <-errChan:
		t.Errorf("Error waiting for request: %v", err)
	case <-time.After(6 * time.Second):
		t.Error("Timeout waiting for request")
	}
}

// TestNetworkWaitForResponse tests waiting for specific responses
func TestNetworkWaitForResponse(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Start waiting for response in background
	responseChan := make(chan *browser.Response)
	errChan := make(chan error)

	go func() {
		resp, err := page.WaitForResponse(".*\\/api\\/data", 5000)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- resp
	}()

	// Navigate to page that makes API call
	err = page.Navigate(ts.URL + "/network-test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for response or error
	select {
	case resp := <-responseChan:
		if !strings.Contains(resp.URL, "/api/data") {
			t.Errorf("Wrong response captured: %s", resp.URL)
		}
		if resp.Status != 200 {
			t.Errorf("Wrong status: got %d, want 200", resp.Status)
		}
	case err := <-errChan:
		t.Errorf("Error waiting for response: %v", err)
	case <-time.After(6 * time.Second):
		t.Error("Timeout waiting for response")
	}
}

// TestNetworkMultipleRoutes tests multiple route handlers
func TestNetworkMultipleRoutes(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	apiCalls := 0
	imageCalls := 0

	// Route for API calls
	err = page.Route(".*\\/api\\/.*", func(req *browser.Request) error {
		apiCalls++
		return req.Continue()
	})
	if err != nil {
		t.Fatal(err)
	}

	// Route for images
	err = page.Route(".*\\.(png|jpg)", func(req *browser.Request) error {
		imageCalls++
		return req.Abort("failed")
	})
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to page with both API calls and images
	err = page.Navigate(`data:text/html,
		<img src="` + ts.URL + `/test.png" />
		<script>
			fetch('` + ts.URL + `/api/data');
		</script>
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for network activity
	time.Sleep(1 * time.Second)

	if apiCalls == 0 {
		t.Error("No API calls intercepted")
	}
	if imageCalls == 0 {
		t.Error("No image calls intercepted")
	}
}

// TestNetworkPostData tests intercepting POST requests
func TestNetworkPostData(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	var capturedPostData string

	// Intercept POST requests
	err = page.Route(".*", func(req *browser.Request) error {
		if req.Method == "POST" {
			capturedPostData = req.PostData
		}
		return req.Continue()
	})
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to form page
	err = page.Navigate(ts.URL + "/form")
	if err != nil {
		t.Fatal(err)
	}

	// Fill and submit form
	err = page.Type("#name", "Test User")
	if err != nil {
		t.Fatal(err)
	}

	err = page.Click("#submit")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for form submission
	time.Sleep(500 * time.Millisecond)

	// Check captured POST data
	if !strings.Contains(capturedPostData, "name=Test+User") {
		t.Errorf("POST data not captured correctly: %s", capturedPostData)
	}
}
