package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// RemoteDebuggingInfo represents information about Chrome's remote debugging endpoint
type RemoteDebuggingInfo struct {
	Browser              string `json:"Browser"`
	ProtocolVersion      string `json:"Protocol-Version"`
	UserAgent            string `json:"User-Agent"`
	V8Version            string `json:"V8-Version"`
	WebKitVersion        string `json:"WebKit-Version"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// ChromeTab represents a Chrome tab/page
type ChromeTab struct {
	Description          string `json:"description"`
	DevtoolsFrontendURL  string `json:"devtoolsFrontendUrl"`
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// GetRemoteDebuggingInfo retrieves Chrome's remote debugging information
func GetRemoteDebuggingInfo(host string, port int) (*RemoteDebuggingInfo, error) {
	url := fmt.Sprintf("http://%s:%d/json/version", host, port)
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Chrome at %s:%d", host, port)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response body")
	}

	var info RemoteDebuggingInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, errors.Wrap(err, "parsing JSON response")
	}

	return &info, nil
}

// ListTabs returns a list of all open tabs in Chrome
func ListTabs(host string, port int) ([]ChromeTab, error) {
	url := fmt.Sprintf("http://%s:%d/json", host, port)
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Chrome at %s:%d", host, port)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response body")
	}

	var tabs []ChromeTab
	if err := json.Unmarshal(body, &tabs); err != nil {
		return nil, errors.Wrap(err, "parsing JSON response")
	}

	return tabs, nil
}

// ConnectToTab connects to an existing Chrome tab by its ID
func (b *Browser) ConnectToTab(ctx context.Context, host string, port int, tabID string) error {
	// Find the tab
	tabs, err := ListTabs(host, port)
	if err != nil {
		return errors.Wrap(err, "listing tabs")
	}

	var targetTab *ChromeTab
	for _, tab := range tabs {
		if tab.ID == tabID || tab.URL == tabID {
			targetTab = &tab
			break
		}
	}

	if targetTab == nil {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Connect to the tab's WebSocket endpoint
	return b.ConnectToWebSocket(ctx, targetTab.WebSocketDebuggerURL)
}

// ConnectToWebSocket connects to a Chrome instance via WebSocket URL
func (b *Browser) ConnectToWebSocket(ctx context.Context, wsURL string) error {
	// Create a new context with the WebSocket URL
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, wsURL)
	
	// Create the browser context
	var browserCtx context.Context
	var browserCancel context.CancelFunc

	if b.opts.Verbose {
		browserCtx, browserCancel = chromedp.NewContext(
			allocCtx,
			chromedp.WithLogf(log.Printf),
		)
	} else {
		browserCtx, browserCancel = chromedp.NewContext(allocCtx)
	}

	// Store context and cancel functions
	b.ctx = browserCtx
	b.cancelFunc = func() {
		browserCancel()
		allocCancel()
	}

	return nil
}

// ConnectToRunningChrome connects to an already running Chrome instance
func (b *Browser) ConnectToRunningChrome(ctx context.Context, host string, port int) error {
	// Get debugging info
	info, err := GetRemoteDebuggingInfo(host, port)
	if err != nil {
		return err
	}

	if info.WebSocketDebuggerURL == "" {
		return errors.New("Chrome instance does not have remote debugging enabled")
	}

	// Connect via WebSocket
	return b.ConnectToWebSocket(ctx, info.WebSocketDebuggerURL)
}