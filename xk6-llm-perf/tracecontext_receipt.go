package llmperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// UnmarshalJSON admits only canonical, internally consistent trace receipts.
func (c *TraceContext) UnmarshalJSON(data []byte) error {
	if err := jsoncheck.Check(json.NewDecoder(bytes.NewReader(data)), jsoncheck.Options{RejectNull: true}); err != nil {
		return fmt.Errorf("trace context: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("trace context: %w", err)
	}
	for name := range fields {
		switch name {
		case "trace_id", "span_id", "parent_span_id", "trace_flags", "traceparent", "tracestate":
		default:
			return fmt.Errorf("unknown trace context field %q", name)
		}
	}
	type receipt TraceContext
	var value receipt
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("trace context: %w", err)
	}
	if err := validateTraceContext((*TraceContext)(&value)); err != nil {
		return err
	}
	*c = TraceContext(value)
	return nil
}

func validateTraceContext(c *TraceContext) error {
	if c == nil {
		return nil
	}
	sc, err := configuredTraceContext(Config{TraceParent: c.TraceParent, TraceState: c.TraceState})
	if err != nil || !sc.IsValid() {
		return fmt.Errorf("invalid retained trace context")
	}
	if c.TraceID != sc.TraceID().String() || c.SpanID != sc.SpanID().String() || c.TraceFlags != sc.TraceFlags().String() {
		return fmt.Errorf("trace context IDs or flags disagree with traceparent")
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(trace.ContextWithSpanContext(context.Background(), sc), carrier)
	if c.TraceParent != carrier.Get("traceparent") || c.TraceState != carrier.Get("tracestate") {
		return fmt.Errorf("trace context headers are not canonical")
	}
	if c.ParentSpanID != "" {
		parent, err := trace.SpanIDFromHex(c.ParentSpanID)
		if err != nil || !parent.IsValid() || parent == sc.SpanID() || parent.String() != c.ParentSpanID {
			return fmt.Errorf("invalid trace context parent span ID")
		}
	}
	return nil
}
