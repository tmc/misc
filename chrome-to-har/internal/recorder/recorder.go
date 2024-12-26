package recorder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
	"github.com/pkg/errors"
)

// FilterOption configures entry filtering
type FilterOption struct {
	JQExpr   string
	Template string
}

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
	streaming       bool
	filter          *FilterOption
}

func (r *Recorder) logf(format string, args ...interface{}) {
	if r.verbose {
		log.Printf(format, args...)
	}
}

func truncateURL(url string) string {
	const maxLength = 80
	if len(url) <= maxLength {
		return url
	}
	return url[:maxLength/2] + "..." + url[len(url)-maxLength/2:]
}

type Option func(*Recorder) error

func WithVerbose(verbose bool) Option {
	return func(r *Recorder) error {
		r.verbose = verbose
		return nil
	}
}

func WithStreaming(stream bool) Option {
	return func(r *Recorder) error {
		r.streaming = stream
		return nil
	}
}

func WithFilter(expr string) Option {
	return func(r *Recorder) error {
		if expr == "" {
			return nil
		}
		r.filter = &FilterOption{JQExpr: expr}
		return nil
	}
}

func WithTemplate(tmpl string) Option {
	return func(r *Recorder) error {
		if tmpl == "" {
			return nil
		}
		// Validate template
		_, err := template.New("har").Parse(tmpl)
		if err != nil {
			return fmt.Errorf("invalid template: %w", err)
		}
		r.filter = &FilterOption{Template: tmpl}
		return nil
	}
}

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

func (r *Recorder) SetCookies(cookies []*network.Cookie) {
	r.Lock()
	defer r.Unlock()
	r.cookies = cookies
	r.logf("Loaded %d cookies from profile", len(cookies))
}

func (r *Recorder) createHAREntry(reqID network.RequestID) *har.Entry {
	req := r.requests[reqID]
	if req == nil {
		return nil
	}

	resp := r.responses[reqID]
	if resp == nil {
		return nil
	}

	timing := r.timings[reqID]
	if timing == nil {
		return nil
	}

	if r.omitFilter != nil && r.omitFilter.MatchString(req.URL) {
		return nil
	}

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

	return entry
}

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
			go func(requestID network.RequestID) {
				body, err := network.GetResponseBody(requestID).Do(ctx)
				if err != nil {
					r.logf("Error fetching response body: %v", err)
					return
				}
				r.Lock()
				r.bodies[requestID] = body
				if r.streaming {
					if entry := r.createHAREntry(requestID); entry != nil {
						if r.filter != nil {
							var err error
							if r.filter.JQExpr != "" {
								entry, err = r.applyJQFilter(entry)
							} else if r.filter.Template != "" {
								entry, err = r.applyTemplate(entry)
							}
							if err != nil {
								r.logf("Error applying filter: %v", err)
								return
							}
						}
						if entry != nil {
							entryJSON, err := json.Marshal(entry)
							if err != nil {
								r.logf("Error marshalling HAR entry: %v", err)
								return
							}
							fmt.Println(string(entryJSON))
						}
					}
				}
				r.Unlock()
			}(e.RequestID)
		}
	}
}

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
	for reqID := range r.requests {
		if entry := r.createHAREntry(reqID); entry != nil {
			h.Log.Entries = append(h.Log.Entries, entry)
			processedRequests++
		}
	}

	r.logf("Successfully processed %d/%d requests", processedRequests, len(r.requests))

	jsonBytes, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshalling HAR to JSON")
	}

	fmt.Println(string(jsonBytes))
	r.logf("Successfully wrote HAR file to: %s", filename)
	return nil
}

func (r *Recorder) applyTemplate(entry *har.Entry) (*har.Entry, error) {
	tmpl, err := template.New("har").Parse(r.filter.Template)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, entry); err != nil {
		return nil, err
	}

	// Convert template output to a new entry
	result := &har.Entry{
		Response: &har.Response{
			Content: &har.Content{
				Text: buf.String(),
			},
		},
	}
	return result, nil
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

