package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
	"github.com/pkg/errors"
)

// Recorder captures and manages network traffic for HAR generation.
type Recorder struct {
	sync.Mutex
	requests        map[network.RequestID]*network.Request
	responses       map[network.RequestID]*network.Response
	bodies          map[network.RequestID][]byte
	timings         map[network.RequestID]*network.EventLoadingFinished
	cookies         []*network.Cookie
	verbose         bool
	startTime       time.Time
	navigationStart time.Time
	cookieFilter    *regexp.Regexp
	urlFilter       *regexp.Regexp
	blockFilter     *regexp.Regexp
	omitFilter      *regexp.Regexp
}

// Option configures a Recorder.
type Option func(*Recorder) error

// WithVerbose enables verbose logging.
func WithVerbose(verbose bool) Option {
	return func(r *Recorder) error {
		r.verbose = verbose
		return nil
	}
}

// WithCookiePattern sets a regexp pattern for filtering cookies.
func WithCookiePattern(pattern string) Option {
	return func(r *Recorder) error {
		if pattern == "" {
			return nil
		}
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid cookie pattern: %w", err)
		}
		r.cookieFilter = filter
		return nil
	}
}

// WithURLPattern sets a regexp pattern for filtering URLs.
func WithURLPattern(pattern string) Option {
	return func(r *Recorder) error {
		if pattern == "" {
			return nil
		}
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid URL pattern: %w", err)
		}
		r.urlFilter = filter
		return nil
	}
}

// WithBlockPattern sets a regexp pattern for blocking URLs.
func WithBlockPattern(pattern string) Option {
	return func(r *Recorder) error {
		if pattern == "" {
			return nil
		}
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid block pattern: %w", err)
		}
		r.blockFilter = filter
		return nil
	}
}

// WithOmitPattern sets a regexp pattern for omitting URLs from HAR output.
func WithOmitPattern(pattern string) Option {
	return func(r *Recorder) error {
		if pattern == "" {
			return nil
		}
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid omit pattern: %w", err)
		}
		r.omitFilter = filter
		return nil
	}
}

// New creates a new Recorder with the given options.
func New(opts ...Option) (*Recorder, error) {
	r := &Recorder{
		requests:  make(map[network.RequestID]*network.Request),
		responses: make(map[network.RequestID]*network.Response),
		bodies:    make(map[network.RequestID][]byte),
		timings:   make(map[network.RequestID]*network.EventLoadingFinished),
		startTime: time.Now(),
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Recorder) logf(format string, args ...interface{}) {
	if r.verbose {
		log.Printf(format, args...)
	}
}

// SetCookies updates the recorder's cookies.
func (r *Recorder) SetCookies(cookies []*network.Cookie) {
	r.Lock()
	defer r.Unlock()
	r.cookies = cookies
	r.logf("Loaded %d cookies from profile", len(cookies))
}

// HandleNetworkEvent processes Chrome DevTools Protocol network events.
func (r *Recorder) HandleNetworkEvent(ctx context.Context) func(interface{}) {
	return func(ev interface{}) {
		r.Lock()
		defer r.Unlock()

		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if r.navigationStart.IsZero() {
				r.navigationStart = time.Now()
			}
			if r.blockFilter != nil && r.blockFilter.MatchString(e.Request.URL) {
				r.logf("Blocking request: %s", truncateURL(e.Request.URL))
				network.SetBlockedURLS([]string{e.Request.URL}).Do(ctx)
				return
			}
			if r.urlFilter != nil && !r.urlFilter.MatchString(e.Request.URL) {
				return
			}
			r.requests[e.RequestID] = e.Request
			r.logf("Request: %s %s", e.Request.Method, truncateURL(e.Request.URL))

		case *network.EventResponseReceived:
			if r.urlFilter != nil && !r.urlFilter.MatchString(e.Response.URL) {
				return
			}
			r.responses[e.RequestID] = e.Response
			r.logf("Response: %d %s", e.Response.Status, truncateURL(e.Response.URL))

		case *network.EventLoadingFinished:
			r.timings[e.RequestID] = e
			// Fetch response body in a separate goroutine
			go func(requestID network.RequestID) {
				body, err := network.GetResponseBody(requestID).Do(ctx)
				if err != nil {
					r.logf("Error fetching response body: %v", err)
					return
				}
				r.Lock()
				r.bodies[requestID] = body
				r.Unlock()
			}(e.RequestID)
		}
	}
}

// WriteHAR writes the recorded network traffic to a HAR file.
func (r *Recorder) WriteHAR(filename string) error {
	r.Lock()
	defer r.Unlock()

	r.logf("Starting HAR file generation...")
	r.logf("Found %d requests to process", len(r.requests))

	h := &har.HAR{
		Log: &har.Log{
			Version: "1.2",
			Creator: &har.Creator{
				Name:    "chrome-to-har",
				Version: "1.0",
			},
			Pages:   make([]*har.Page, 0),
			Entries: make([]*har.Entry, 0),
		},
	}

	// Add profile cookies as a separate page entry
	if len(r.cookies) > 0 {
		r.logf("Adding %d profile cookies", len(r.cookies))
		cookiePage := &har.Page{
			ID:    "profile_cookies",
			Title: "Profile Cookies",
		}
		h.Log.Pages = append(h.Log.Pages, cookiePage)

		for _, c := range r.cookies {
			if r.cookieFilter != nil && !r.cookieFilter.MatchString(c.Name) {
				continue
			}
			h.Log.Entries = append(h.Log.Entries, &har.Entry{
				Pageref: cookiePage.ID,
				Request: &har.Request{
					Cookies: []*har.Cookie{{
						Name:     c.Name,
						Value:    c.Value,
						Path:     c.Path,
						Domain:   c.Domain,
						HTTPOnly: c.HTTPOnly,
						Secure:   c.Secure,
					}},
				},
			})
		}
	}

	processedRequests := 0
	for reqID, req := range r.requests {
		// Skip if URL matches omit pattern
		if r.omitFilter != nil && r.omitFilter.MatchString(req.URL) {
			r.logf("Omitting request from HAR: %s", truncateURL(req.URL))
			continue
		}

		resp := r.responses[reqID]
		if resp == nil {
			r.logf("Skipping request %s: no response", reqID)
			continue
		}

		timing := r.timings[reqID]
		if timing == nil {
			r.logf("Skipping request %s: no timing", reqID)
			continue
		}

		// Convert MonotonicTime to milliseconds since navigation start
		waitTime := float64(0)
		if timing.Timestamp != nil {
			elapsed := timing.Timestamp.Time().Sub(r.navigationStart)
			waitTime = float64(elapsed.Milliseconds())
		}

		entry := &har.Entry{
			StartedDateTime: r.navigationStart.Format(time.RFC3339),
			Request: &har.Request{
				Method:      req.Method,
				URL:         req.URL,
				HTTPVersion: "HTTP/1.1",
				Headers:     convertHeaders(req.Headers),
				Cookies:     convertNetworkCookies(req.Headers["Cookie"]),
			},
			Response: &har.Response{
				Status:      int64(resp.Status),
				StatusText:  resp.StatusText,
				HTTPVersion: resp.Protocol,
				Headers:     convertHeaders(resp.Headers),
				Cookies:     convertNetworkCookies(resp.Headers["Set-Cookie"]),
				Content: &har.Content{
					Size:     int64(resp.EncodedDataLength),
					MimeType: resp.MimeType,
				},
			},
			Cache: &har.Cache{},
			Timings: &har.Timings{
				Send:    0,
				Wait:    waitTime,
				Receive: 0,
			},
		}

		if body, ok := r.bodies[reqID]; ok {
			entry.Response.Content.Text = string(body)
		}

		h.Log.Entries = append(h.Log.Entries, entry)
		processedRequests++
	}

	r.logf("Successfully processed %d/%d requests", processedRequests, len(r.requests))

	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrap(err, "creating output file")
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(h); err != nil {
		return errors.Wrap(err, "encoding HAR")
	}

	r.logf("Successfully wrote HAR file to: %s", filename)
	return nil
}

// Helper functions

func truncateURL(url string) string {
	const maxLength = 80
	if len(url) <= maxLength {
		return url
	}
	return url[:maxLength/2] + "..." + url[len(url)-maxLength/2:]
}

func convertHeaders(headers map[string]interface{}) []*har.NameValuePair {
	var result []*har.NameValuePair
	for k, v := range headers {
		result = append(result, &har.NameValuePair{
			Name:  k,
			Value: fmt.Sprint(v),
		})
	}
	return result
}

func convertNetworkCookies(cookieHeader interface{}) []*har.Cookie {
	if cookieHeader == nil {
		return nil
	}

	cookies := make([]*har.Cookie, 0)
	cookieStr := fmt.Sprint(cookieHeader)

	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		nameValue := strings.SplitN(part, "=", 2)
		if len(nameValue) != 2 {
			continue
		}

		cookies = append(cookies, &har.Cookie{
			Name:  strings.TrimSpace(nameValue[0]),
			Value: strings.TrimSpace(nameValue[1]),
		})
	}

	return cookies
}
