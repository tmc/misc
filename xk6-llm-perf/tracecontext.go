package llmperf

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContext records the client span context sent on the wire. Exporters can
// use these IDs with Measurement's dispatch/finish times and outcome. The
// extension does not send spans to a collector. Unsampled records must not be
// exported. A logical request, including video polling, uses one client span.
type TraceContext struct {
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	TraceFlags   string `json:"trace_flags"`
	TraceParent  string `json:"traceparent"`
	TraceState   string `json:"tracestate,omitempty"`
}

func configuredTraceContext(config Config) (trace.SpanContext, error) {
	if config.TraceParent == "" {
		if config.TraceState != "" {
			return trace.SpanContext{}, fmt.Errorf("tracestate requires traceparent")
		}
		return trace.SpanContext{}, nil
	}
	if len(config.TraceParent) > 1024 || len(config.TraceState) > 512 {
		return trace.SpanContext{}, fmt.Errorf("trace context exceeds header size limit")
	}
	// The propagator deliberately ignores future-version suffixes. Check the
	// whole header so ignored bytes cannot hide invalid HTTP field values.
	for _, c := range config.TraceParent {
		if c <= ' ' || c == 0x7f {
			return trace.SpanContext{}, fmt.Errorf("invalid traceparent character")
		}
	}
	if _, err := trace.ParseTraceState(config.TraceState); err != nil {
		return trace.SpanContext{}, fmt.Errorf("invalid tracestate: %w", err)
	}
	carrier := propagation.MapCarrier{"traceparent": config.TraceParent, "tracestate": config.TraceState}
	ctx := propagation.TraceContext{}.Extract(context.Background(), carrier)
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return sc, fmt.Errorf("invalid traceparent")
	}
	return sc, nil
}

// attachTraceContext runs after admission and before dispatch. IDs are unrelated
// to workload seeds, so concurrent VUs and repeated runs cannot reuse them.
func attachTraceContext(req *http.Request, config Config) (*http.Request, *TraceContext) {
	if !config.Tracing && config.TraceParent == "" {
		return req, nil
	}
	parent, _ := configuredTraceContext(config) // Already checked by validateConfig.
	if !parent.IsValid() {
		parent = trace.SpanContextFromContext(req.Context())
	}
	tid, flags := parent.TraceID(), parent.TraceFlags()
	if !parent.IsValid() {
		for !tid.IsValid() {
			rand.Read(tid[:])
		}
		flags = trace.FlagsSampled
	}
	var sid trace.SpanID
	for !sid.IsValid() || sid == parent.SpanID() {
		rand.Read(sid[:])
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: flags, TraceState: parent.TraceState(),
	})
	req = req.WithContext(trace.ContextWithSpanContext(req.Context(), sc))
	propagation.TraceContext{}.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	record := &TraceContext{TraceID: tid.String(), SpanID: sid.String(),
		TraceFlags: flags.String(), TraceParent: req.Header.Get("traceparent"),
		TraceState: req.Header.Get("tracestate")}
	if parent.IsValid() {
		record.ParentSpanID = parent.SpanID().String()
	}
	return req, record
}
