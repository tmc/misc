package llmperf

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

// HTTPTrace preserves completed client transport stages in milliseconds.
// Missing stages remain null, including connection setup on a reused socket.
// Waiting starts after the request was written; it is not server latency.
type HTTPTrace struct {
	DNS       *float64 `json:"dns"`
	Connect   *float64 `json:"connect"`
	TLS       *float64 `json:"tls"`
	Sending   *float64 `json:"sending"`
	Waiting   *float64 `json:"waiting"`
	Receiving *float64 `json:"receiving"`
	Reused    *bool    `json:"reused"`
}

type transportTrace struct {
	mu                                   sync.Mutex
	value                                HTTPTrace
	dns, connect, tls, got, wrote, first time.Time
}

func stage(start, end time.Time) *float64 {
	if start.IsZero() || end.Before(start) {
		return nil
	}
	v := ms(end.Sub(start))
	return &v
}

func attachTrace(req *http.Request) (*http.Request, *transportTrace) {
	t := &transportTrace{}
	update := func(f func(time.Time)) { t.mu.Lock(); defer t.mu.Unlock(); f(time.Now()) }
	hooks := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { update(func(now time.Time) { t.dns = now }) },
		DNSDone: func(v httptrace.DNSDoneInfo) {
			update(func(now time.Time) {
				if v.Err == nil {
					t.value.DNS = stage(t.dns, now)
				}
			})
		},
		ConnectStart: func(_, _ string) {
			update(func(now time.Time) {
				if t.connect.IsZero() {
					t.connect = now
				}
			})
		},
		ConnectDone: func(_, _ string, err error) {
			update(func(now time.Time) {
				if err == nil && t.value.Connect == nil {
					t.value.Connect = stage(t.connect, now)
				}
			})
		},
		TLSHandshakeStart: func() { update(func(now time.Time) { t.tls = now }) },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			update(func(now time.Time) {
				if err == nil {
					t.value.TLS = stage(t.tls, now)
				}
			})
		},
		GotConn: func(v httptrace.GotConnInfo) {
			update(func(now time.Time) { t.got = now; b := v.Reused; t.value.Reused = &b })
		},
		WroteRequest: func(v httptrace.WroteRequestInfo) {
			update(func(now time.Time) {
				if v.Err == nil {
					t.wrote = now
					t.value.Sending = stage(t.got, now)
				}
			})
		},
		GotFirstResponseByte: func() { update(func(now time.Time) { t.first = now; t.value.Waiting = stage(t.wrote, now) }) },
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), hooks)), t
}

func (t *transportTrace) finish(end time.Time) HTTPTrace {
	t.mu.Lock()
	defer t.mu.Unlock()
	v := t.value
	v.Receiving = stage(t.first, end)
	return v
}
