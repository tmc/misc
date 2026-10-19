package traceworkload

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

const header = "Timestamp,Request tokens,Response tokens\n"

func TestBurstGPTGolden(t *testing.T) {
	// Pinned AIPerf burst_gpt.py _preprocess_trace multiplies seconds by1000.
	// This independently authored CSV also exercises quoted ignored columns.
	source := "Timestamp,Request tokens,Response tokens,Model\r\n1.25,0,4,\"a,b\"\r\n1.25,11,8,c\r\n2,11,16,c\r\n"
	trace, err := ReadBurstGPT(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	if trace.SourceHash != hash || trace.Format != "burstgpt-csv-v1" || trace.TimestampUnit != "seconds" || trace.OffsetUnit != "milliseconds" || trace.CountPolicy != "declared-input-target-and-output-budget" {
		t.Fatalf("metadata: %+v", trace)
	}
	want := []Row{{hash + ":1", 1250, 0, 4, 1}, {hash + ":2", 1250, 11, 8, 2}, {hash + ":3", 2000, 11, 16, 3}}
	if len(trace.Rows) != len(want) {
		t.Fatal(trace.Rows)
	}
	for i, row := range trace.Rows {
		if row != want[i] {
			t.Fatalf("row %d: %+v want %+v", i, row, want[i])
		}
	}
	changed, err := ReadBurstGPT(strings.NewReader(strings.ReplaceAll(source, "c\r\n", "d\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if changed.SourceHash == trace.SourceHash || changed.Rows[0].ID == trace.Rows[0].ID {
		t.Fatal("ignored bytes lost from source identity")
	}
}

func TestBurstGPTReject(t *testing.T) {
	for _, tt := range []struct{ name, source string }{
		{"empty", ""}, {"no rows", header}, {"missing header", "Timestamp,Request tokens\n0,1\n"},
		{"duplicate header", "Timestamp,Request tokens,Response tokens,Timestamp\n0,1,1,0\n"},
		{"unit header", "Timestamp ms,Request tokens,Response tokens\n0,1,1\n"},
		{"missing count", header + "0,,1\n"}, {"zero output", header + "0,1,0\n"},
		{"fractional count", header + "0,1.5,2\n"}, {"negative count", header + "0,-1,2\n"},
		{"overflow count", header + "0,4294967296,2\n"}, {"token cap", header + "0,1000001,2\n"},
		{"hex time", header + "0x1p0,1,1\n"}, {"negative time", header + "-1,1,1\n"}, {"nonfinite", header + "NaN,1,1\n"},
		{"infinite", header + "1e999,1,1\n"}, {"out of order", header + "2,1,1\n1,1,1\n"},
		{"time cap", header + "86400.001,1,1\n"}, {"missing field", header + "0,1\n"},
		{"bad CSV", header + "0,\"bad,1\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			trace, err := ReadBurstGPT(strings.NewReader(tt.source))
			if err == nil {
				t.Fatalf("accepted %+v", trace)
			}
		})
	}
}

func TestBurstGPTLimits(t *testing.T) {
	for _, tt := range []struct{ name, source string }{
		{"bytes", header + "0,1,1," + strings.Repeat("x", maxBytes)},
		{"rows", header + strings.Repeat("0,1,1\n", maxRows+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ReadBurstGPT(strings.NewReader(tt.source)); err == nil {
				t.Fatal("accepted over limit")
			}
		})
	}
	trace, err := ReadBurstGPT(strings.NewReader(header + "86400,1000000,1000000\n"))
	if err != nil || trace.Rows[0].Offset != 86400000 {
		t.Fatalf("valid boundary: %+v %v", trace, err)
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestBurstGPTReadFailure(t *testing.T) {
	_, err := ReadBurstGPT(failedReader{})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
}
