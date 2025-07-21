package differential

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// StateTracker tracks page state changes during interactions
type StateTracker struct {
	mu          sync.RWMutex
	states      []*PageState
	currentState *PageState
	verbose     bool
	options     *StateTrackingOptions
}

// StateTrackingOptions configures state tracking behavior
type StateTrackingOptions struct {
	TrackDOM         bool          `json:"track_dom"`
	TrackLocalStorage bool          `json:"track_local_storage"`
	TrackSessionStorage bool        `json:"track_session_storage"`
	TrackCookies     bool          `json:"track_cookies"`
	TrackViewport    bool          `json:"track_viewport"`
	TrackURL         bool          `json:"track_url"`
	TrackPerformance bool          `json:"track_performance"`
	SnapshotInterval time.Duration `json:"snapshot_interval"`
	MaxSnapshots     int           `json:"max_snapshots"`
}

// PageState represents the state of a page at a specific point in time
type PageState struct {
	ID              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	URL             string                 `json:"url"`
	Title           string                 `json:"title"`
	DOM             *DOMState             `json:"dom,omitempty"`
	Storage         *StorageState         `json:"storage,omitempty"`
	Cookies         []*CookieState        `json:"cookies,omitempty"`
	Viewport        *ViewportState        `json:"viewport,omitempty"`
	Performance     *PerformanceState     `json:"performance,omitempty"`
	NetworkActivity *NetworkActivityState `json:"network_activity,omitempty"`
	Labels          map[string]string     `json:"labels,omitempty"`
	Description     string                `json:"description,omitempty"`
}

// DOMState represents the DOM state
type DOMState struct {
	ElementCount    int               `json:"element_count"`
	ContentLength   int               `json:"content_length"`
	Checksum        string            `json:"checksum"`
	KeyElements     map[string]string `json:"key_elements,omitempty"`
	VisibleElements []string          `json:"visible_elements,omitempty"`
}

// StorageState represents browser storage state
type StorageState struct {
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
	StorageSize    int64             `json:"storage_size"`
}

// CookieState represents cookie state
type CookieState struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
}

// ViewportState represents viewport dimensions and scroll position
type ViewportState struct {
	Width      int64 `json:"width"`
	Height     int64 `json:"height"`
	ScrollX    int64 `json:"scroll_x"`
	ScrollY    int64 `json:"scroll_y"`
	DeviceScale float64 `json:"device_scale"`
}

// PerformanceState represents performance metrics
type PerformanceState struct {
	NavigationStart time.Time `json:"navigation_start"`
	DOMReady        time.Time `json:"dom_ready"`
	LoadComplete    time.Time `json:"load_complete"`
	FirstPaint      time.Time `json:"first_paint,omitempty"`
	FirstContentful time.Time `json:"first_contentful,omitempty"`
	MemoryUsage     int64     `json:"memory_usage"`
	JSHeapSize      int64     `json:"js_heap_size"`
}

// NetworkActivityState represents network activity snapshot
type NetworkActivityState struct {
	ActiveRequests  int     `json:"active_requests"`
	TotalRequests   int     `json:"total_requests"`
	TotalTransferred int64   `json:"total_transferred"`
	AverageLatency  float64 `json:"average_latency"`
	ErrorCount      int     `json:"error_count"`
}

// InteractionEvent represents a user interaction or system event
type InteractionEvent struct {
	ID          string                 `json:"id"`
	Type        InteractionType        `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Target      string                 `json:"target,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	BeforeState *PageState            `json:"before_state,omitempty"`
	AfterState  *PageState            `json:"after_state,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// InteractionType represents the type of interaction
type InteractionType string

const (
	InteractionTypeClick      InteractionType = "click"
	InteractionTypeScroll     InteractionType = "scroll"
	InteractionTypeInput      InteractionType = "input"
	InteractionTypeNavigation InteractionType = "navigation"
	InteractionTypeLoad       InteractionType = "load"
	InteractionTypeCustom     InteractionType = "custom"
)

// NewStateTracker creates a new state tracker
func NewStateTracker(verbose bool, options *StateTrackingOptions) *StateTracker {
	if options == nil {
		options = &StateTrackingOptions{
			TrackDOM:         true,
			TrackLocalStorage: true,
			TrackSessionStorage: true,
			TrackCookies:     true,
			TrackViewport:    true,
			TrackURL:         true,
			TrackPerformance: true,
			SnapshotInterval: 1 * time.Second,
			MaxSnapshots:     100,
		}
	}

	return &StateTracker{
		states:  make([]*PageState, 0),
		verbose: verbose,
		options: options,
	}
}

// CaptureCurrentState captures the current state of the page
func (st *StateTracker) CaptureCurrentState(ctx context.Context, description string, labels map[string]string) (*PageState, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	state := &PageState{
		ID:          fmt.Sprintf("state-%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Description: description,
		Labels:      labels,
	}

	// Capture URL and title
	if st.options.TrackURL {
		var url, title string
		if err := chromedp.Run(ctx,
			chromedp.Location(&url),
			chromedp.Title(&title),
		); err != nil {
			return nil, errors.Wrap(err, "capturing URL and title")
		}
		state.URL = url
		state.Title = title
	}

	// Capture DOM state
	if st.options.TrackDOM {
		domState, err := st.captureDOMState(ctx)
		if err != nil {
			if st.verbose {
				fmt.Printf("Warning: failed to capture DOM state: %v\n", err)
			}
		} else {
			state.DOM = domState
		}
	}

	// Capture storage state
	if st.options.TrackLocalStorage || st.options.TrackSessionStorage {
		storageState, err := st.captureStorageState(ctx)
		if err != nil {
			if st.verbose {
				fmt.Printf("Warning: failed to capture storage state: %v\n", err)
			}
		} else {
			state.Storage = storageState
		}
	}

	// Capture cookies
	if st.options.TrackCookies {
		cookies, err := st.captureCookies(ctx)
		if err != nil {
			if st.verbose {
				fmt.Printf("Warning: failed to capture cookies: %v\n", err)
			}
		} else {
			state.Cookies = cookies
		}
	}

	// Capture viewport
	if st.options.TrackViewport {
		viewport, err := st.captureViewport(ctx)
		if err != nil {
			if st.verbose {
				fmt.Printf("Warning: failed to capture viewport: %v\n", err)
			}
		} else {
			state.Viewport = viewport
		}
	}

	// Capture performance
	if st.options.TrackPerformance {
		performance, err := st.capturePerformance(ctx)
		if err != nil {
			if st.verbose {
				fmt.Printf("Warning: failed to capture performance: %v\n", err)
			}
		} else {
			state.Performance = performance
		}
	}

	// Add to states list
	st.states = append(st.states, state)

	// Limit the number of snapshots
	if len(st.states) > st.options.MaxSnapshots {
		st.states = st.states[len(st.states)-st.options.MaxSnapshots:]
	}

	st.currentState = state

	if st.verbose {
		fmt.Printf("Captured state: %s at %s\n", state.ID, state.Timestamp.Format("15:04:05"))
	}

	return state, nil
}

// captureDOMState captures the current DOM state
func (st *StateTracker) captureDOMState(ctx context.Context) (*DOMState, error) {
	var elementCount int
	var contentLength int
	var innerHTML string

	// Get element count
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('*').length`, &elementCount),
	); err != nil {
		return nil, errors.Wrap(err, "getting element count")
	}

	// Get content length
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.documentElement.innerHTML.length`, &contentLength),
	); err != nil {
		return nil, errors.Wrap(err, "getting content length")
	}

	// Get innerHTML for checksum
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.documentElement.innerHTML`, &innerHTML),
	); err != nil {
		return nil, errors.Wrap(err, "getting innerHTML")
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", hashString(innerHTML))

	// Get key elements
	keyElements := make(map[string]string)
	selectors := []string{"title", "h1", "h2", "[data-testid]", "#main", ".main"}
	
	for _, selector := range selectors {
		var text string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					const el = document.querySelector('%s');
					return el ? el.textContent.trim().substring(0, 100) : '';
				})()
			`, selector), &text),
		); err == nil && text != "" {
			keyElements[selector] = text
		}
	}

	// Get visible elements
	var visibleElements []string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('*')).filter(el => {
				const rect = el.getBoundingClientRect();
				return rect.width > 0 && rect.height > 0 && 
					   rect.top >= 0 && rect.left >= 0 && 
					   rect.bottom <= window.innerHeight && 
					   rect.right <= window.innerWidth;
			}).slice(0, 20).map(el => el.tagName + (el.id ? '#' + el.id : '') + (el.className ? '.' + el.className.split(' ').join('.') : ''))
		`, &visibleElements),
	); err != nil {
		visibleElements = []string{}
	}

	return &DOMState{
		ElementCount:    elementCount,
		ContentLength:   contentLength,
		Checksum:        checksum,
		KeyElements:     keyElements,
		VisibleElements: visibleElements,
	}, nil
}

// captureStorageState captures browser storage state
func (st *StateTracker) captureStorageState(ctx context.Context) (*StorageState, error) {
	storage := &StorageState{}

	// Capture localStorage
	if st.options.TrackLocalStorage {
		var localStorageData map[string]string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					const data = {};
					for (let i = 0; i < localStorage.length; i++) {
						const key = localStorage.key(i);
						data[key] = localStorage.getItem(key);
					}
					return data;
				})()
			`, &localStorageData),
		); err == nil {
			storage.LocalStorage = localStorageData
		}
	}

	// Capture sessionStorage
	if st.options.TrackSessionStorage {
		var sessionStorageData map[string]string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					const data = {};
					for (let i = 0; i < sessionStorage.length; i++) {
						const key = sessionStorage.key(i);
						data[key] = sessionStorage.getItem(key);
					}
					return data;
				})()
			`, &sessionStorageData),
		); err == nil {
			storage.SessionStorage = sessionStorageData
		}
	}

	// Calculate storage size
	var storageSize int64
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			JSON.stringify(localStorage).length + JSON.stringify(sessionStorage).length
		`, &storageSize),
	); err == nil {
		storage.StorageSize = storageSize
	}

	return storage, nil
}

// captureCookies captures browser cookies
func (st *StateTracker) captureCookies(ctx context.Context) ([]*CookieState, error) {
	var cookieData []map[string]interface{}
	
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			document.cookie.split(';').map(cookie => {
				const [name, value] = cookie.trim().split('=');
				return { name: name, value: value || '' };
			}).filter(cookie => cookie.name)
		`, &cookieData),
	); err != nil {
		return nil, errors.Wrap(err, "capturing cookies")
	}

	cookies := make([]*CookieState, len(cookieData))
	for i, cookie := range cookieData {
		cookies[i] = &CookieState{
			Name:  fmt.Sprintf("%v", cookie["name"]),
			Value: fmt.Sprintf("%v", cookie["value"]),
		}
	}

	return cookies, nil
}

// captureViewport captures viewport information
func (st *StateTracker) captureViewport(ctx context.Context) (*ViewportState, error) {
	var viewportData map[string]interface{}
	
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`({
			width: window.innerWidth,
			height: window.innerHeight,
			scrollX: window.scrollX,
			scrollY: window.scrollY,
			deviceScale: window.devicePixelRatio
		})`, &viewportData),
	); err != nil {
		return nil, errors.Wrap(err, "capturing viewport")
	}

	return &ViewportState{
		Width:       int64(viewportData["width"].(float64)),
		Height:      int64(viewportData["height"].(float64)),
		ScrollX:     int64(viewportData["scrollX"].(float64)),
		ScrollY:     int64(viewportData["scrollY"].(float64)),
		DeviceScale: viewportData["deviceScale"].(float64),
	}, nil
}

// capturePerformance captures performance metrics
func (st *StateTracker) capturePerformance(ctx context.Context) (*PerformanceState, error) {
	var perfData map[string]interface{}
	
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const perf = performance.timing;
				const memory = performance.memory || {};
				return {
					navigationStart: perf.navigationStart,
					domReady: perf.domContentLoadedEventEnd,
					loadComplete: perf.loadEventEnd,
					jsHeapSize: memory.usedJSHeapSize || 0,
					memoryUsage: memory.totalJSHeapSize || 0
				};
			})()
		`, &perfData),
	); err != nil {
		return nil, errors.Wrap(err, "capturing performance")
	}

	perf := &PerformanceState{}
	
	if navStart, ok := perfData["navigationStart"].(float64); ok && navStart > 0 {
		perf.NavigationStart = time.Unix(int64(navStart/1000), 0)
	}
	
	if domReady, ok := perfData["domReady"].(float64); ok && domReady > 0 {
		perf.DOMReady = time.Unix(int64(domReady/1000), 0)
	}
	
	if loadComplete, ok := perfData["loadComplete"].(float64); ok && loadComplete > 0 {
		perf.LoadComplete = time.Unix(int64(loadComplete/1000), 0)
	}
	
	if jsHeap, ok := perfData["jsHeapSize"].(float64); ok {
		perf.JSHeapSize = int64(jsHeap)
	}
	
	if memUsage, ok := perfData["memoryUsage"].(float64); ok {
		perf.MemoryUsage = int64(memUsage)
	}

	return perf, nil
}

// GetCurrentState returns the current state
func (st *StateTracker) GetCurrentState() *PageState {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.currentState
}

// GetStates returns all captured states
func (st *StateTracker) GetStates() []*PageState {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	states := make([]*PageState, len(st.states))
	copy(states, st.states)
	return states
}

// GetStateByID returns a state by its ID
func (st *StateTracker) GetStateByID(id string) *PageState {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	for _, state := range st.states {
		if state.ID == id {
			return state
		}
	}
	return nil
}

// CompareStates compares two page states and returns differences
func (st *StateTracker) CompareStates(state1, state2 *PageState) []string {
	var differences []string

	// Compare URLs
	if state1.URL != state2.URL {
		differences = append(differences, fmt.Sprintf("URL: %s -> %s", state1.URL, state2.URL))
	}

	// Compare titles
	if state1.Title != state2.Title {
		differences = append(differences, fmt.Sprintf("Title: %s -> %s", state1.Title, state2.Title))
	}

	// Compare DOM
	if state1.DOM != nil && state2.DOM != nil {
		if state1.DOM.ElementCount != state2.DOM.ElementCount {
			differences = append(differences, fmt.Sprintf("Element count: %d -> %d", state1.DOM.ElementCount, state2.DOM.ElementCount))
		}
		if state1.DOM.ContentLength != state2.DOM.ContentLength {
			differences = append(differences, fmt.Sprintf("Content length: %d -> %d", state1.DOM.ContentLength, state2.DOM.ContentLength))
		}
		if state1.DOM.Checksum != state2.DOM.Checksum {
			differences = append(differences, "DOM content changed")
		}
	}

	// Compare storage
	if state1.Storage != nil && state2.Storage != nil {
		if state1.Storage.StorageSize != state2.Storage.StorageSize {
			differences = append(differences, fmt.Sprintf("Storage size: %d -> %d", state1.Storage.StorageSize, state2.Storage.StorageSize))
		}
	}

	// Compare viewport
	if state1.Viewport != nil && state2.Viewport != nil {
		if state1.Viewport.ScrollX != state2.Viewport.ScrollX || state1.Viewport.ScrollY != state2.Viewport.ScrollY {
			differences = append(differences, fmt.Sprintf("Scroll position: (%d,%d) -> (%d,%d)", 
				state1.Viewport.ScrollX, state1.Viewport.ScrollY, 
				state2.Viewport.ScrollX, state2.Viewport.ScrollY))
		}
	}

	return differences
}

// RecordInteraction records a user interaction with before/after states
func (st *StateTracker) RecordInteraction(ctx context.Context, interactionType InteractionType, target string, data map[string]interface{}) (*InteractionEvent, error) {
	// Capture before state
	beforeState, err := st.CaptureCurrentState(ctx, fmt.Sprintf("Before %s", interactionType), nil)
	if err != nil {
		return nil, errors.Wrap(err, "capturing before state")
	}

	event := &InteractionEvent{
		ID:          fmt.Sprintf("interaction-%d", time.Now().UnixNano()),
		Type:        interactionType,
		Timestamp:   time.Now(),
		Target:      target,
		Data:        data,
		BeforeState: beforeState,
	}

	// The caller should call CompleteInteraction after the interaction is finished
	return event, nil
}

// CompleteInteraction completes an interaction by capturing the after state
func (st *StateTracker) CompleteInteraction(ctx context.Context, event *InteractionEvent) error {
	// Capture after state
	afterState, err := st.CaptureCurrentState(ctx, fmt.Sprintf("After %s", event.Type), nil)
	if err != nil {
		return errors.Wrap(err, "capturing after state")
	}

	event.AfterState = afterState
	event.Duration = time.Since(event.Timestamp)

	if st.verbose {
		differences := st.CompareStates(event.BeforeState, event.AfterState)
		fmt.Printf("Interaction %s completed with %d changes\n", event.Type, len(differences))
		for _, diff := range differences {
			fmt.Printf("  - %s\n", diff)
		}
	}

	return nil
}

// ExportStates exports all states to JSON
func (st *StateTracker) ExportStates(filename string) error {
	st.mu.RLock()
	defer st.mu.RUnlock()

	data, err := json.MarshalIndent(st.states, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling states")
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return errors.Wrap(err, "writing states file")
	}

	return nil
}

// hashString creates a simple hash of a string
func hashString(s string) []byte {
	// Simple hash function for demonstration
	// In production, you might want to use a more robust hash function
	h := make([]byte, 8)
	for i, b := range []byte(s) {
		h[i%8] ^= b
	}
	return h
}