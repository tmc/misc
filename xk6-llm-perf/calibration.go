package llmperf

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync"
	"time"
)

// CalibrationOptions bounds explicitly requested network probes. The zero value
// is invalid. URL must be an HTTP(S) URL without credentials, query or fragment.
// Timeout and Interval use Go duration syntax; Interval may be empty for zero.
type CalibrationOptions struct {
	URL             string `json:"url"`
	Method          string `json:"method"`
	Count           int    `json:"count"`
	Timeout         string `json:"timeout"`
	Interval        string `json:"interval"`
	MaxBodyBytes    int64  `json:"maxBodyBytes"`
	TransportSource string `json:"transportSource"`
}

// CalibrationProbe preserves failed probes and unavailable timings. Durations
// are milliseconds. ErrorStage is bounded and omits potentially secret errors.
type CalibrationProbe struct {
	Index      int        `json:"index"`
	StartedAt  float64    `json:"startedAt"`
	Duration   float64    `json:"duration"`
	Success    bool       `json:"success"`
	Status     int        `json:"status"`
	ErrorStage string     `json:"errorStage,omitempty"`
	Bytes      int64      `json:"bytes"`
	FirstByte  *float64   `json:"firstByte"`
	RTT        *float64   `json:"rtt"`
	Trace      *HTTPTrace `json:"trace"`
}

// CalibrationReport records configuration, observations and probe overhead.
// Duration is elapsed wall time, not a measured change in benchmark latency.
// MeanRTT is available only from successful fresh TCP connections. It supports
// arithmetic subtraction, not a claim of measured server latency.
type CalibrationReport struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Options            CalibrationOptions `json:"options"`
	ProbeType          string             `json:"probeType"`
	Basis              string             `json:"basis"`
	StartedAt          float64            `json:"startedAt"`
	Duration           float64            `json:"duration"`
	ResolutionDuration *float64           `json:"resolutionDuration"`
	ResolvedAddress    string             `json:"resolvedAddress,omitempty"`
	Probes             []CalibrationProbe `json:"probes"`
	Successes          int                `json:"successes"`
	RTTSamples         int                `json:"rttSamples"`
	MeanRTT            *float64           `json:"meanRTT"`
	MeanFirstByte      *float64           `json:"meanFirstByte"`
	Complete           bool               `json:"complete"`
	Runtime            map[string]string  `json:"runtime"`
	ClientTimeout      *float64           `json:"clientTimeout"`
}

func validateCalibration(o CalibrationOptions, httpMode bool) (*url.URL, time.Duration, time.Duration, error) {
	u, err := url.Parse(o.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(o.URL) > 2048 {
		return nil, 0, 0, fmt.Errorf("calibration URL must be credential-free HTTP(S)")
	}
	if o.Count < 1 || o.Count > 1000 {
		return nil, 0, 0, fmt.Errorf("calibration count must be between 1 and 1000")
	}
	timeout, err := time.ParseDuration(o.Timeout)
	if err != nil || timeout <= 0 || timeout > 30*time.Second {
		return nil, 0, 0, fmt.Errorf("calibration timeout must be positive and at most 30s")
	}
	var interval time.Duration
	if o.Interval != "" {
		interval, err = time.ParseDuration(o.Interval)
		if err != nil || interval < 0 || interval > time.Second {
			return nil, 0, 0, fmt.Errorf("calibration interval must be between 0 and 1s")
		}
	}
	worst := time.Duration(o.Count) * (timeout + interval)
	if !httpMode {
		worst += timeout // Resolution has its own deadline.
	}
	if worst > 5*time.Minute {
		return nil, 0, 0, fmt.Errorf("calibration worst-case duration exceeds 5m")
	}
	if httpMode && (o.Method != "HEAD" && o.Method != "GET") {
		return nil, 0, 0, fmt.Errorf("calibration method must be HEAD or GET")
	}
	if httpMode && (o.MaxBodyBytes < 1 || o.MaxBodyBytes > 1<<20 || int64(o.Count)*o.MaxBodyBytes > 16<<20) {
		return nil, 0, 0, fmt.Errorf("calibration body budget exceeds bounds")
	}
	if httpMode && (o.TransportSource == "" || len(o.TransportSource) > 256) {
		return nil, 0, 0, fmt.Errorf("calibration transport source is required and at most 256 bytes")
	}
	if !httpMode && (o.Method != "" || o.MaxBodyBytes != 0 || o.TransportSource != "") {
		return nil, 0, 0, fmt.Errorf("TCP calibration does not accept HTTP options")
	}
	return u, timeout, interval, nil
}

func calibrationWait(ctx context.Context, interval time.Duration) error {
	t := time.NewTimer(interval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func finishCalibration(r *CalibrationReport, start time.Time) {
	r.Duration = ms(time.Since(start))
	var tcp, first float64
	var firstN int
	for _, p := range r.Probes {
		if !p.Success {
			continue
		}
		r.Successes++
		if p.RTT != nil {
			tcp += *p.RTT
			r.RTTSamples++
		}
		if p.FirstByte != nil {
			first += *p.FirstByte
			firstN++
		}
	}
	if r.RTTSamples > 0 {
		v := tcp / float64(r.RTTSamples)
		r.MeanRTT = &v
	}
	if firstN > 0 {
		v := first / float64(firstN)
		r.MeanFirstByte = &v
	}
	r.Complete = len(r.Probes) == r.Options.Count && r.Successes == r.Options.Count
}

// CalibrateHTTP issues bounded serial HTTP health probes using the supplied
// transport. Redirects are refused. HTTP first-byte timing includes server work;
// connection reuse leaves TCP RTT unavailable. No inference metrics are emitted.
func CalibrateHTTP(ctx context.Context, client *http.Client, o CalibrationOptions) (CalibrationReport, error) {
	_, timeout, interval, err := validateCalibration(o, true)
	if err != nil {
		return CalibrationReport{}, err
	}
	if client == nil {
		return CalibrationReport{}, fmt.Errorf("calibration HTTP client is required")
	}
	start := time.Now()
	r := CalibrationReport{SchemaVersion: 1, Options: o, ProbeType: "http_health", Basis: "HTTP first-byte includes server processing; TCP connect is a transport proxy", StartedAt: millis(start), Runtime: buildVersions()}
	c := *client
	clientTimeout := ms(c.Timeout)
	r.ClientTimeout = &clientTimeout
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for i := 0; i < o.Count; i++ {
		if err := ctx.Err(); err != nil {
			finishCalibration(&r, start)
			return r, err
		}
		if i > 0 {
			if err := calibrationWait(ctx, interval); err != nil {
				finishCalibration(&r, start)
				return r, err
			}
		}
		pstart := time.Now()
		p := CalibrationProbe{Index: i, StartedAt: millis(pstart)}
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(probeCtx, o.Method, o.URL, nil)
		if err != nil {
			cancel()
			finishCalibration(&r, start)
			return r, fmt.Errorf("create calibration probe: %w", err)
		}
		req, tr := attachTrace(req)
		var mu sync.Mutex
		var first *float64
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{GotFirstResponseByte: func() { mu.Lock(); v := ms(time.Since(pstart)); first = &v; mu.Unlock() }}))
		resp, err := c.Do(req)
		if err != nil {
			p.ErrorStage = "transport"
		} else {
			p.Status = resp.StatusCode
			p.Bytes, err = io.Copy(io.Discard, io.LimitReader(resp.Body, o.MaxBodyBytes+1))
			closeErr := resp.Body.Close()
			switch {
			case p.Bytes > o.MaxBodyBytes:
				p.ErrorStage = "body_budget"
			case err != nil || closeErr != nil:
				p.ErrorStage = "body"
			case p.Status < 200 || p.Status >= 300:
				p.ErrorStage = "http_status"
			default:
				p.Success = true
			}
		}
		p.Duration = ms(time.Since(pstart))
		trace := tr.finish(time.Now())
		p.Trace = &trace
		mu.Lock()
		p.FirstByte = first
		mu.Unlock()
		if p.Success {
			p.RTT = trace.Connect
		}
		cancel()
		r.Probes = append(r.Probes, p)
	}
	finishCalibration(&r, start)
	return r, nil
}

// CalibrateTCP resolves the target once and times fresh TCP connects, excluding
// DNS and TLS. HTTPS uses plain TCP to its port, matching the pinned AIPerf probe.
// Connect timing remains a network proxy affected by scheduling and host load.
func CalibrateTCP(ctx context.Context, o CalibrationOptions) (CalibrationReport, error) {
	u, timeout, interval, err := validateCalibration(o, false)
	if err != nil {
		return CalibrationReport{}, err
	}
	start := time.Now()
	r := CalibrationReport{SchemaVersion: 1, Options: o, ProbeType: "tcp_connect", Basis: "fresh TCP connect excluding DNS/TLS; arithmetic subtraction only", StartedAt: millis(start), Runtime: buildVersions()}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	resolveStart := time.Now()
	resolveCtx, resolveCancel := context.WithTimeout(ctx, timeout)
	addresses, resolveErr := net.DefaultResolver.LookupIPAddr(resolveCtx, u.Hostname())
	resolveCancel()
	v := ms(time.Since(resolveStart))
	r.ResolutionDuration = &v
	if resolveErr == nil && len(addresses) > 0 {
		r.ResolvedAddress = net.JoinHostPort(addresses[0].String(), port)
	} else {
		resolveErr = fmt.Errorf("calibration resolution unavailable")
	}
	for i := 0; i < o.Count; i++ {
		if err := ctx.Err(); err != nil {
			finishCalibration(&r, start)
			return r, err
		}
		if i > 0 {
			if err := calibrationWait(ctx, interval); err != nil {
				finishCalibration(&r, start)
				return r, err
			}
		}
		pstart := time.Now()
		p := CalibrationProbe{Index: i, StartedAt: millis(pstart)}
		if resolveErr != nil {
			p.ErrorStage = "resolution"
		} else {
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			conn, err := (&net.Dialer{}).DialContext(probeCtx, "tcp", r.ResolvedAddress)
			elapsed := ms(time.Since(pstart))
			if err != nil {
				p.ErrorStage = "connect"
			} else {
				p.Success = true
				p.RTT = &elapsed
				_ = conn.Close()
			}
			cancel()
		}
		p.Duration = ms(time.Since(pstart))
		r.Probes = append(r.Probes, p)
	}
	finishCalibration(&r, start)
	return r, nil
}
