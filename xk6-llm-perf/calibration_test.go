package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func calibrationOptions(url string) CalibrationOptions {
	return CalibrationOptions{URL: url, Method: "GET", Count: 2, Timeout: "1s", MaxBodyBytes: 100, TransportSource: "test standard library transport"}
}

func TestCalibrationHTTPReuseAndTLS(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(fmt.Sprint(tls), func(t *testing.T) {
			s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
			if tls {
				s.StartTLS()
			} else {
				s.Start()
			}
			defer s.Close()
			c := s.Client()
			defer c.CloseIdleConnections()
			r, err := CalibrateHTTP(context.Background(), c, calibrationOptions(s.URL))
			if err != nil {
				t.Fatal(err)
			}
			if !r.Complete || r.Successes != 2 || r.RTTSamples != 1 || r.MeanRTT == nil || r.MeanFirstByte == nil || len(r.Probes) != 2 {
				t.Fatalf("report: %+v", r)
			}
			if r.Probes[0].Trace.Connect == nil || r.Probes[1].RTT != nil || r.Probes[1].Trace.Reused == nil || !*r.Probes[1].Trace.Reused {
				t.Fatalf("reuse: %+v", r.Probes)
			}
			if tls && r.Probes[0].Trace.TLS == nil {
				t.Fatal("TLS unavailable")
			}
			if !tls && r.Probes[0].Trace.TLS != nil {
				t.Fatal("invented TLS")
			}
		})
	}
}

func TestCalibrationFailureRetention(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		stage   string
	}{
		{"status", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, "http_status"},
		{"budget", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", 101)) }, "body_budget"},
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://must-not-be-contacted.invalid", http.StatusFound)
		}, "http_status"},
		{"timeout", func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, "transport"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(tc.handler)
			defer s.Close()
			o := calibrationOptions(s.URL)
			if tc.name == "timeout" {
				o.Timeout = "20ms"
			}
			r, err := CalibrateHTTP(context.Background(), s.Client(), o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Complete || r.Successes != 0 || len(r.Probes) != 2 || r.MeanRTT != nil || r.MeanFirstByte != nil {
				t.Fatalf("report: %+v", r)
			}
			for _, p := range r.Probes {
				if p.Success || p.ErrorStage != tc.stage {
					t.Fatalf("probe: %+v", p)
				}
			}
		})
	}
}

func TestCalibrationCancellation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	o := calibrationOptions(s.URL)
	o.Count = 3
	r, err := CalibrateHTTP(ctx, s.Client(), o)
	if err == nil || len(r.Probes) != 1 || r.Complete || r.Probes[0].Success {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestCalibrationTCP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := s.URL
	r, err := CalibrateTCP(context.Background(), CalibrationOptions{URL: url, Count: 3, Timeout: "100ms"})
	if err != nil || !r.Complete || r.Successes != 3 || r.RTTSamples != 3 || r.MeanRTT == nil || r.MeanFirstByte != nil || r.ResolutionDuration == nil || r.ResolvedAddress == "" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	s.Close()
	r, err = CalibrateTCP(context.Background(), CalibrationOptions{URL: url, Count: 3, Timeout: "100ms"})
	if err != nil || r.Complete || r.Successes != 0 || len(r.Probes) != 3 || r.MeanRTT != nil {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	for _, p := range r.Probes {
		if p.ErrorStage != "connect" || p.RTT != nil {
			t.Fatalf("probe: %+v", p)
		}
	}
}

func TestCalibrationValidation(t *testing.T) {
	for _, mutate := range []func(*CalibrationOptions){
		func(o *CalibrationOptions) { o.URL = "http://user:secret@localhost" },
		func(o *CalibrationOptions) { o.URL = "http://localhost?secret=yes" },
		func(o *CalibrationOptions) { o.URL = "file:///etc/passwd" },
		func(o *CalibrationOptions) { o.Count = 0 },
		func(o *CalibrationOptions) { o.Count = 1001 },
		func(o *CalibrationOptions) { o.Timeout = "31s" },
		func(o *CalibrationOptions) { o.Count = 1000; o.Timeout = "1s" },
		func(o *CalibrationOptions) { o.Interval = "-1s" },
		func(o *CalibrationOptions) { o.Method = "POST" },
		func(o *CalibrationOptions) { o.MaxBodyBytes = 1 << 21 },
		func(o *CalibrationOptions) { o.TransportSource = "" },
	} {
		o := calibrationOptions("http://localhost")
		mutate(&o)
		if _, err := CalibrateHTTP(context.Background(), http.DefaultClient, o); err == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
}

func ExampleCalibrateHTTP() {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer s.Close()
	r, err := CalibrateHTTP(context.Background(), s.Client(), CalibrationOptions{URL: s.URL, Method: "HEAD", Count: 2, Timeout: "1s", MaxBodyBytes: 1024, TransportSource: "example transport"})
	fmt.Println(err, r.ProbeType, r.Successes, r.Complete)
	// Output: <nil> http_health 2 true
}

func ExampleCalibrateTCP() {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()
	r, err := CalibrateTCP(context.Background(), CalibrationOptions{URL: s.URL, Count: 2, Timeout: "1s"})
	fmt.Println(err, r.ProbeType, r.RTTSamples, r.MeanRTT != nil)
	// Output: <nil> tcp_connect 2 true
}

func ExampleCalibrationOptions() {
	o := CalibrationOptions{URL: "https://localhost:8443/health", Count: 3, Timeout: "100ms"}
	fmt.Println(o.Count, o.Timeout)
	// Output: 3 100ms
}

func ExampleCalibrationProbe() {
	p := CalibrationProbe{Index: 0, ErrorStage: "connect"}
	fmt.Println(p.Success, p.RTT == nil)
	// Output: false true
}

func ExampleCalibrationReport() {
	r := CalibrationReport{SchemaVersion: 1, ProbeType: "tcp_connect", Probes: []CalibrationProbe{{Index: 0, ErrorStage: "connect"}}}
	b, _ := json.Marshal(r.Probes[0].RTT)
	fmt.Println(r.SchemaVersion, r.Complete, string(b))
	// Output: 1 false null
}
