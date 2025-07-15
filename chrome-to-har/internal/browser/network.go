package browser

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// Request represents an intercepted network request
type Request struct {
	ID       string
	URL      string
	Method   string
	Headers  map[string]string
	PostData string

	page        *Page
	requestID   network.RequestID
	intercepted bool
	mu          sync.Mutex
}

// Response represents a network response
type Response struct {
	URL        string
	Status     int
	StatusText string
	Headers    map[string]string
	Body       []byte
}

// Route represents a network route handler
type Route struct {
	pattern *regexp.Regexp
	handler RouteHandler
}

// RouteHandler handles intercepted requests
type RouteHandler func(*Request) error

// NetworkManager manages network interception and monitoring
type NetworkManager struct {
	page    *Page
	routes  []Route
	enabled bool
	mu      sync.RWMutex

	// Request tracking
	requests  map[network.RequestID]*Request
	responses map[network.RequestID]*Response
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(page *Page) *NetworkManager {
	return &NetworkManager{
		page:      page,
		routes:    make([]Route, 0),
		requests:  make(map[network.RequestID]*Request),
		responses: make(map[network.RequestID]*Response),
	}
}

// Enable enables network interception
func (nm *NetworkManager) Enable() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.enabled {
		return nil
	}

	// Enable network domain
	if err := chromedp.Run(nm.page.ctx,
		network.Enable(),
		fetch.Enable(),
	); err != nil {
		return errors.Wrap(err, "enabling network interception")
	}

	// Set up event handlers
	chromedp.ListenTarget(nm.page.ctx, nm.handleNetworkEvent)

	nm.enabled = true
	return nil
}

// Disable disables network interception
func (nm *NetworkManager) Disable() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.enabled {
		return nil
	}

	if err := chromedp.Run(nm.page.ctx,
		network.Disable(),
		fetch.Disable(),
	); err != nil {
		return errors.Wrap(err, "disabling network interception")
	}

	nm.enabled = false
	return nil
}

// Route adds a route handler for matching URLs
func (p *Page) Route(pattern string, handler RouteHandler) error {
	// Ensure network manager exists
	if p.networkManager == nil {
		p.networkManager = NewNetworkManager(p)
	}

	// Compile pattern as regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return errors.Wrap(err, "compiling route pattern")
	}

	p.networkManager.mu.Lock()
	p.networkManager.routes = append(p.networkManager.routes, Route{
		pattern: re,
		handler: handler,
	})
	p.networkManager.mu.Unlock()

	// Enable network interception if not already enabled
	return p.networkManager.Enable()
}

// handleNetworkEvent processes network events
func (nm *NetworkManager) handleNetworkEvent(ev interface{}) {
	switch ev := ev.(type) {
	case *fetch.EventRequestPaused:
		nm.handleRequestPaused(ev)
	case *network.EventRequestWillBeSent:
		// TODO: Implement handleRequestWillBeSent
	case *network.EventResponseReceived:
		// TODO: Implement handleResponseReceived
	}
}

// handleRequestPaused handles intercepted requests
func (nm *NetworkManager) handleRequestPaused(ev *fetch.EventRequestPaused) {
	req := &Request{
		ID:          string(ev.RequestID),
		URL:         ev.Request.URL,
		Method:      ev.Request.Method,
		Headers:     make(map[string]string),
		page:        nm.page,
		requestID:   network.RequestID(ev.RequestID),
		intercepted: true,
	}

	// Copy headers
	for name, value := range ev.Request.Headers {
		req.Headers[name] = value.(string)
	}

	// Get post data if available
	if ev.Request.HasPostData && len(ev.Request.PostDataEntries) > 0 {
		// Concatenate post data entries
		var postData strings.Builder
		for _, entry := range ev.Request.PostDataEntries {
			postData.WriteString(entry.Bytes)
		}
		req.PostData = postData.String()
	}

	// Store request
	nm.mu.Lock()
	nm.requests[network.RequestID(ev.RequestID)] = req
	nm.mu.Unlock()

	// Check if any route matches
	nm.mu.RLock()
	routes := nm.routes
	nm.mu.RUnlock()

	for _, route := range routes {
		if route.pattern.MatchString(req.URL) {
			// Call handler
			if err := route.handler(req); err == nil {
				return // Handler processed the request
			}
		}
	}

	// No handler matched, continue request normally
	req.Continue()
}

// Continue continues the request
func (r *Request) Continue(opts ...ContinueOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.intercepted {
		return errors.New("request not intercepted")
	}

	options := &ContinueOptions{
		Headers:  r.Headers,
		Method:   r.Method,
		PostData: r.PostData,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Continue request
	return fetch.ContinueRequest(fetch.RequestID(r.requestID)).
		WithMethod(options.Method).
		WithPostData(options.PostData).
		Do(r.page.ctx)
}

// Abort aborts the request
func (r *Request) Abort(reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.intercepted {
		return errors.New("request not intercepted")
	}

	errorReason := network.ErrorReasonAborted
	switch reason {
	case "failed":
		errorReason = network.ErrorReasonFailed
	case "timedout":
		errorReason = network.ErrorReasonTimedOut
	case "accessdenied":
		errorReason = network.ErrorReasonAccessDenied
	}

	return fetch.FailRequest(fetch.RequestID(r.requestID), errorReason).Do(r.page.ctx)
}

// Fulfill fulfills the request with a custom response
func (r *Request) Fulfill(opts ...FulfillOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.intercepted {
		return errors.New("request not intercepted")
	}

	options := &FulfillOptions{
		Status:  200,
		Headers: make(map[string]string),
		Body:    []byte{},
	}

	for _, opt := range opts {
		opt(options)
	}

	// Build response headers
	responseHeaders := make([]*fetch.HeaderEntry, 0, len(options.Headers))
	for name, value := range options.Headers {
		responseHeaders = append(responseHeaders, &fetch.HeaderEntry{
			Name:  name,
			Value: value,
		})
	}

	// Fulfill request
	return fetch.FulfillRequest(fetch.RequestID(r.requestID), int64(options.Status)).
		WithResponseHeaders(responseHeaders).
		WithBody(base64.StdEncoding.EncodeToString(options.Body)).
		Do(r.page.ctx)
}

// ContinueOptions configures request continuation
type ContinueOptions struct {
	URL      string
	Method   string
	PostData string
	Headers  map[string]string
}

// ContinueOption modifies continue options
type ContinueOption func(*ContinueOptions)

// WithURL changes the request URL
func WithURL(url string) ContinueOption {
	return func(o *ContinueOptions) {
		o.URL = url
	}
}

// WithMethod changes the request method
func WithMethod(method string) ContinueOption {
	return func(o *ContinueOptions) {
		o.Method = method
	}
}

// WithPostData changes the post data
func WithPostData(data string) ContinueOption {
	return func(o *ContinueOptions) {
		o.PostData = data
	}
}

// WithHeaders sets request headers
func WithHeaders(headers map[string]string) ContinueOption {
	return func(o *ContinueOptions) {
		o.Headers = headers
	}
}

// FulfillOptions configures response fulfillment
type FulfillOptions struct {
	Status      int
	Headers     map[string]string
	Body        []byte
	ContentType string
}

// FulfillOption modifies fulfill options
type FulfillOption func(*FulfillOptions)

// WithStatus sets response status
func WithStatus(status int) FulfillOption {
	return func(o *FulfillOptions) {
		o.Status = status
	}
}

// WithResponseHeaders sets response headers
func WithResponseHeaders(headers map[string]string) FulfillOption {
	return func(o *FulfillOptions) {
		o.Headers = headers
	}
}

// WithBody sets response body
func WithBody(body []byte) FulfillOption {
	return func(o *FulfillOptions) {
		o.Body = body
	}
}

// WithContentType sets content type
func WithContentType(contentType string) FulfillOption {
	return func(o *FulfillOptions) {
		o.ContentType = contentType
		o.Headers["Content-Type"] = contentType
	}
}

// WaitForRequest waits for a request matching the pattern
func (p *Page) WaitForRequest(pattern string, timeout ...int) (*Request, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "compiling pattern")
	}

	// Default timeout
	timeoutMs := 30000
	if len(timeout) > 0 {
		timeoutMs = timeout[0]
	}

	ctx, cancel := context.WithTimeout(p.ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Enable network if needed
	if p.networkManager == nil {
		p.networkManager = NewNetworkManager(p)
		if err := p.networkManager.Enable(); err != nil {
			return nil, err
		}
	}

	// Wait for matching request
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for request")
		case <-ticker.C:
			p.networkManager.mu.RLock()
			for _, req := range p.networkManager.requests {
				if re.MatchString(req.URL) {
					p.networkManager.mu.RUnlock()
					return req, nil
				}
			}
			p.networkManager.mu.RUnlock()
		}
	}
}

// WaitForResponse waits for a response matching the pattern
func (p *Page) WaitForResponse(pattern string, timeout ...int) (*Response, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "compiling pattern")
	}

	// Default timeout
	timeoutMs := 30000
	if len(timeout) > 0 {
		timeoutMs = timeout[0]
	}

	ctx, cancel := context.WithTimeout(p.ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Enable network if needed
	if p.networkManager == nil {
		p.networkManager = NewNetworkManager(p)
		if err := p.networkManager.Enable(); err != nil {
			return nil, err
		}
	}

	// Wait for matching response
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for response")
		case <-ticker.C:
			p.networkManager.mu.RLock()
			for _, resp := range p.networkManager.responses {
				if re.MatchString(resp.URL) {
					p.networkManager.mu.RUnlock()
					return resp, nil
				}
			}
			p.networkManager.mu.RUnlock()
		}
	}
}
