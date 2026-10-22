package llmperf

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func receiptFixture() *TraceContext {
	return &TraceContext{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", SpanID: "00f067aa0ba902b7", ParentSpanID: "1111111111111111", TraceFlags: "01", TraceParent: testParent, TraceState: "vendor=value"}
}

func TestTraceReceiptRoundTrip(t *testing.T) {
	for _, name := range []string{"legacy", "sampled", "unsampled", "root"} {
		t.Run(name, func(t *testing.T) {
			run := formulaRun()
			c := receiptFixture()
			switch name {
			case "legacy":
				c = nil
			case "unsampled":
				c.TraceFlags = "00"
				c.TraceParent = strings.TrimSuffix(c.TraceParent, "01") + "00"
			case "root":
				c.ParentSpanID, c.TraceState = "", ""
			}
			run.Records[0].TraceContext = c
			var manifest, records bytes.Buffer
			if err := WriteRun(&manifest, run); err != nil {
				t.Fatal(err)
			}
			checked, err := ReadRun(&manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteRecords(&records, checked.Records); err != nil {
				t.Fatal(err)
			}
			rows, err := ReadRecords(&records)
			if err != nil {
				t.Fatal(err)
			}
			if err := ReconcileRecords(checked, rows); err != nil {
				t.Fatal(err)
			}
			if c == nil {
				if rows[0].TraceContext != nil {
					t.Fatal("legacy absence changed")
				}
			} else if *rows[0].TraceContext != *c {
				t.Fatal("receipt changed")
			}
		})
	}
}

func TestTraceReceiptRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*TraceContext)
	}{
		{"zero trace", func(c *TraceContext) { c.TraceID = strings.Repeat("0", 32) }},
		{"uppercase ID", func(c *TraceContext) { c.TraceID = strings.ToUpper(c.TraceID) }},
		{"mismatched span", func(c *TraceContext) { c.SpanID = c.ParentSpanID }},
		{"flags", func(c *TraceContext) { c.TraceFlags = "00" }},
		{"missing header", func(c *TraceContext) { c.TraceParent = "" }},
		{"future header", func(c *TraceContext) { c.TraceParent = "01" + c.TraceParent[2:] + "-extra" }},
		{"invalid state", func(c *TraceContext) { c.TraceState = "bad state" }},
		{"noncanonical state", func(c *TraceContext) { c.TraceState = "vendor=value, other=value" }},
		{"zero parent", func(c *TraceContext) { c.ParentSpanID = strings.Repeat("0", 16) }},
		{"self parent", func(c *TraceContext) { c.ParentSpanID = c.SpanID }},
		{"uppercase parent", func(c *TraceContext) { c.ParentSpanID = "ABCDEF0123456789" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := formulaRun()
			c := receiptFixture()
			tc.edit(c)
			run.Records[0].TraceContext = c
			if err := WriteRun(&bytes.Buffer{}, run); err == nil {
				t.Fatal("writer admitted invalid receipt")
			}
			if err := readFormulaRun(run); err == nil {
				t.Fatal("replay admitted invalid receipt")
			}
			if err := WriteRecords(&bytes.Buffer{}, run.Records); err == nil {
				t.Fatal("JSONL writer admitted invalid receipt")
			}
			row, err := json.Marshal(run.Records[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRecords(bytes.NewReader(row)); err == nil {
				t.Fatal("JSONL reader admitted invalid receipt")
			}
		})
	}
}

func TestTraceReceiptStrictJSON(t *testing.T) {
	valid, err := json.Marshal(receiptFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, raw string }{
		{"duplicate", `"trace_id":"bad",`},
		{"escaped duplicate", `"trace_\u0069d":"bad",`},
		{"case alias", `"TRACE_ID":"bad",`},
		{"unknown", `"other":"bad",`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]byte("{"+tc.raw), valid[1:]...)
			var c TraceContext
			if err := json.Unmarshal(bad, &c); err == nil {
				t.Fatal("ambiguous receipt accepted")
			}
			run := formulaRun()
			run.Records[0].TraceContext = receiptFixture()
			manifest, err := json.Marshal(run)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(manifest, valid) {
				t.Fatal("fixture lost receipt")
			}
			if _, err := ReadRun(bytes.NewReader(bytes.Replace(manifest, valid, bad, 1))); err == nil {
				t.Fatal("run reader accepted ambiguous receipt")
			}
			row, err := json.Marshal(run.Records[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRecords(bytes.NewReader(bytes.Replace(row, valid, bad, 1))); err == nil {
				t.Fatal("JSONL reader accepted ambiguous receipt")
			}
		})
	}
	for _, field := range []string{"trace_id", "span_id", "trace_flags", "traceparent"} {
		for _, mode := range []string{"null", "missing"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(valid, &fields); err != nil {
					t.Fatal(err)
				}
				if mode == "null" {
					fields[field] = json.RawMessage("null")
				} else {
					delete(fields, field)
				}
				bad, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				var c TraceContext
				if err := json.Unmarshal(bad, &c); err == nil {
					t.Fatal("required field absent")
				}
			})
		}
	}
	for _, raw := range []string{"null", "[]", "{}"} {
		var c TraceContext
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
