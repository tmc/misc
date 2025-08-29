package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
	chromeErrors "github.com/tmc/misc/chrome-to-har/internal/errors"
)

type Recorder struct {
	sync.Mutex
	requests  map[network.RequestID]*network.Request
	responses map[network.RequestID]*network.Response
	bodies    map[network.RequestID][]byte
	timings   map[network.RequestID]*network.EventLoadingFinished
	verbose   bool
	streaming bool
	filter    *FilterOption
	template  string
}

type FilterOption struct {
	JQExpr   string
	Template string
}

type Option func(*Recorder) error

func WithVerbose(verbose bool) Option {
	return func(r *Recorder) error {
		r.verbose = verbose
		return nil
	}
}

func WithStreaming(streaming bool) Option {
	return func(r *Recorder) error {
		r.streaming = streaming
		return nil
	}
}

func WithFilter(filter string) Option {
	return func(r *Recorder) error {
		if filter != "" {
			r.filter = &FilterOption{JQExpr: filter}
		}
		return nil
	}
}

func WithTemplate(template string) Option {
	return func(r *Recorder) error {
		r.template = template
		return nil
	}
}

func New(opts ...Option) (*Recorder, error) {
	r := &Recorder{
		requests:  make(map[network.RequestID]*network.Request),
		responses: make(map[network.RequestID]*network.Response),
		bodies:    make(map[network.RequestID][]byte),
		timings:   make(map[network.RequestID]*network.EventLoadingFinished),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *Recorder) HandleNetworkEvent(ctx context.Context) func(interface{}) {
	return func(ev interface{}) {
		r.Lock()
		defer r.Unlock()

		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if r.verbose {
				log.Printf("Request: %s %s", e.Request.Method, e.Request.URL)
			}
			r.requests[e.RequestID] = e.Request

			if r.streaming {
				entry := &har.Entry{
					StartedDateTime: time.Now().Format(time.RFC3339),
					Request: &har.Request{
						Method:      e.Request.Method,
						URL:         e.Request.URL,
						HTTPVersion: "HTTP/1.1", // Default to HTTP/1.1 as Protocol isn't available
						Headers:     convertHeaders(e.Request.Headers),
					},
				}
				r.streamEntry(entry)
			}

		case *network.EventResponseReceived:
			if r.verbose {
				log.Printf("Response: %d %s", e.Response.Status, e.Response.URL)
			}
			r.responses[e.RequestID] = e.Response

			if r.streaming {
				entry := &har.Entry{
					StartedDateTime: time.Now().Format(time.RFC3339),
					Request: &har.Request{
						Method: r.requests[e.RequestID].Method,
						URL:    e.Response.URL,
					},
					Response: &har.Response{
						Status:      int64(e.Response.Status),
						StatusText:  e.Response.StatusText,
						HTTPVersion: e.Response.Protocol,
						Headers:     convertHeaders(e.Response.Headers),
						Content: &har.Content{
							MimeType: e.Response.MimeType,
							Size:     int64(e.Response.EncodedDataLength),
						},
					},
				}
				r.streamEntry(entry)
			}

		case *network.EventLoadingFinished:
			r.timings[e.RequestID] = e

			if r.streaming {
				go func(reqID network.RequestID) {
					body, err := network.GetResponseBody(reqID).Do(ctx)
					if err != nil {
						if r.verbose {
							log.Printf("Error getting response body: %v", err)
						}
						return
					}
					r.Lock()
					r.bodies[reqID] = body
					r.Unlock()
				}(e.RequestID)
			}
		}
	}
}

func (r *Recorder) streamEntry(entry *har.Entry) {
	if r.filter != nil && r.filter.JQExpr != "" {
		filtered, err := r.applyJQFilter(entry)
		if err != nil {
			if r.verbose {
				log.Printf("Error applying filter: %v", err)
			}
			return
		}
		if filtered == nil {
			return // Entry filtered out
		}
		entry = filtered
	}

	if r.template != "" {
		templated, err := r.applyTemplate(entry)
		if err != nil {
			if r.verbose {
				log.Printf("Error applying template: %v", err)
			}
			return
		}
		entry = templated
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		if r.verbose {
			log.Printf("Error marshaling entry: %v", err)
		}
		return
	}
	fmt.Println(string(jsonBytes))
}

func (r *Recorder) WriteHAR(filename string) error {
	r.Lock()
	defer r.Unlock()

	if r.verbose {
		log.Printf("Writing HAR file to %s", filename)
	}

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

	for reqID, req := range r.requests {
		resp := r.responses[reqID]
		if resp == nil {
			continue
		}

		timing := r.timings[reqID]
		if timing == nil {
			continue
		}

		entry := &har.Entry{
			StartedDateTime: time.Now().Format(time.RFC3339),
			Request: &har.Request{
				Method:      req.Method,
				URL:         req.URL,
				HTTPVersion: "HTTP/1.1", // Default to HTTP/1.1
				Headers:     convertHeaders(req.Headers),
				Cookies:     r.convertCookies(req.Headers),
			},
			Response: &har.Response{
				Status:      int64(resp.Status),
				StatusText:  resp.StatusText,
				HTTPVersion: resp.Protocol,
				Headers:     convertHeaders(resp.Headers),
				Content: &har.Content{
					Size:     int64(resp.EncodedDataLength),
					MimeType: resp.MimeType,
				},
			},
			Time: float64(timing.Timestamp.Time().UnixNano()) / float64(time.Millisecond),
		}

		if body, ok := r.bodies[reqID]; ok {
			entry.Response.Content.Text = string(body)
		}

		h.Log.Entries = append(h.Log.Entries, entry)
	}

	jsonBytes, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.NetworkRecordError, "failed to marshal HAR data")
	}

	if err := os.WriteFile(filename, jsonBytes, 0644); err != nil {
		return chromeErrors.WithContext(
			chromeErrors.FileError("write", filename, err),
			"format", "har",
		)
	}

	return nil
}

func convertHeaders(headers map[string]interface{}) []*har.NameValuePair {
	pairs := make([]*har.NameValuePair, 0, len(headers))
	for name, value := range headers {
		pairs = append(pairs, &har.NameValuePair{
			Name:  name,
			Value: fmt.Sprint(value),
		})
	}
	return pairs
}

func (r *Recorder) convertCookies(headers map[string]interface{}) []*har.Cookie {
	if cookieHeader, ok := headers["Cookie"]; ok {
		cookies := make([]*har.Cookie, 0)
		for _, cookie := range strings.Split(fmt.Sprint(cookieHeader), ";") {
			parts := strings.SplitN(strings.TrimSpace(cookie), "=", 2)
			if len(parts) != 2 {
				continue
			}
			cookies = append(cookies, &har.Cookie{
				Name:  parts[0],
				Value: parts[1],
			})
		}
		return cookies
	}
	return nil
}

func (r *Recorder) applyTemplate(entry *har.Entry) (*har.Entry, error) {
	t, err := template.New("har").Parse(r.template)
	if err != nil {
		return nil, chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ValidationError, "failed to parse template"),
			"template", r.template,
		)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, entry); err != nil {
		return nil, chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ValidationError, "failed to execute template"),
			"template", r.template,
		)
	}

	return &har.Entry{
		StartedDateTime: entry.StartedDateTime,
		Response: &har.Response{
			Content: &har.Content{
				Text: buf.String(),
			},
		},
	}, nil
}
