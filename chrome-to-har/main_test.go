package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

func TestBasicRun(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		wantErr bool
		want    string
	}{
		{
			name: "missing_profile",
			opts: options{
				outputFile: "test.har",
			},
			wantErr: true,
		},
		{
			name: "basic_profile",
			opts: options{
				profileDir: "Test Profile 1",
				outputFile: "test.har",
				headless:   true,
			},
			wantErr: false,
			want:    "Starting Chrome with profile Test Profile 1",
		},
		{
			name: "with_url",
			opts: options{
				profileDir: "Test Profile 1",
				outputFile: "test.har",
				headless:   true,
				startURL:   "https://example.com",
			},
			wantErr: false,
			want:    "Navigating to https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture, err := testutil.NewOutputCapture()
			if err != nil {
				t.Fatalf("Failed to capture output: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			mockPM := testutil.NewMockProfileManager()
			runner := NewRunner(mockPM)
			err = runner.Run(ctx, tt.opts)
			stdout, logs := capture.Stop()

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.want != "" && !strings.Contains(logs, tt.want) {
				t.Errorf("Run() logs = %q, want to contain %q", logs, tt.want)
			}

			if tt.wantErr && stdout != "" {
				t.Errorf("Run() stdout = %q, want empty on error", stdout)
			}
		})
	}
}

func TestListProfiles(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		want    []string
		wantErr bool
	}{
		{
			name:    "basic_list",
			verbose: false,
			want:    []string{"Test Profile 1", "Test Profile 2"},
		},
		{
			name:    "verbose_list",
			verbose: true,
			want:    []string{"Found valid profile: Test Profile 1", "Found valid profile: Test Profile 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture, err := testutil.NewOutputCapture()
			if err != nil {
				t.Fatalf("Failed to capture output: %v", err)
			}

			mockPM := testutil.NewMockProfileManager()
			mockPM.Verbose = tt.verbose

			profiles, err := mockPM.ListProfiles()
			if err != nil {
				t.Fatalf("ListProfiles() error = %v", err)
			}

			fmt.Println("Available Chrome profiles:")
			for _, p := range profiles {
				fmt.Printf("  - %s\n", p)
			}

			stdout, logs := capture.Stop()

			if (err != nil) != tt.wantErr {
				t.Errorf("ListProfiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			output := stdout + logs
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("ListProfiles() output = %q, want to contain %q", output, want)
				}
			}
		})
	}
}

func TestStreamingOutput(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		want    []string
		wantErr bool
	}{
		{
			name: "basic_streaming",
			opts: options{
				profileDir: "Test Profile 1",
				streaming:  true,
				headless:   true,
			},
			want: []string{
				`"startedDateTime"`,
				`"request"`,
				`"response"`,
			},
		},
		{
			name: "filtered_streaming",
			opts: options{
				profileDir: "Test Profile 1",
				streaming:  true,
				headless:   true,
				filter:     "select(.response.status >= 400)",
			},
			want: []string{
				`"status":`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture, err := testutil.NewOutputCapture()
			if err != nil {
				t.Fatalf("Failed to capture output: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			mockPM := testutil.NewMockProfileManager()
			runner := NewRunner(mockPM)
			err = runner.Run(ctx, tt.opts)
			stdout, _ := capture.Stop()

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, want := range tt.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("Run() stdout = %q, want to contain %q", stdout, want)
				}
			}
		})
	}
}