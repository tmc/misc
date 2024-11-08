package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestNewFeatures(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		input   string
		wantOut string
		wantErr string
	}{
		{
			name:    "HTML formatting",
			args:    []string{"-h"},
			input:   "<html><body><p>Hello</p></body></html>",
			wantOut: "<p>\n  Hello\n</p>",
		},
		{
			name:    "Deeply nested HTML",
			args:    []string{"-h"},
			input:   "<div>" + strings.Repeat("<div>", 10) + "content" + strings.Repeat("</div>", 11),
			wantOut: "<div>\n  <div>\n    <div>\n      <div>\n        <div>\n          <div>\n            <div>\n              <div>\n                <div>\n                  <div>\n                    <div>\n                      content\n                    </div>\n                  </div>\n                </div>\n              </div>\n            </div>\n          </div>\n        </div>\n      </div>\n    </div>\n  </div>\n</div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test
			flag.CommandLine = flag.NewFlagSet(tt.name, flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			// Re-declare flags
			compactOutput = flag.Bool("c", false, "compact instead of pretty-printed output")
			rawOutput = flag.Bool("r", false, "output raw strings, not JSON texts")
			colorOutput = flag.Bool("C", false, "colorize JSON")
			nullInput = flag.Bool("n", false, "use `null` as the single input value")
			slurpInput = flag.Bool("s", false, "read (slurp) all inputs into an array")
			fromJSON = flag.Bool("f", false, "input is JSON, not XML")
			toJSON = flag.Bool("j", false, "output as JSON")
			htmlInput = flag.Bool("h", false, "treat input as HTML")
			streamInput = flag.Bool("S", false, "stream large XML files")
			versionInfo = flag.Bool("v", false, "output version information and exit")
			xpathQuery = flag.String("x", "", "XPath query to select nodes")

			// Parse flags
			err := flag.CommandLine.Parse(tt.args)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			got, err := processInputs([]io.Reader{strings.NewReader(tt.input)})

			if (err != nil) != (tt.wantErr != "") {
				t.Errorf("processInputs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("processInputs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got = strings.TrimSpace(got)
			if !strings.Contains(got, tt.wantOut) {
				t.Errorf("processInputs() got = %v, want %v", got, tt.wantOut)
			}
		})
	}
}
