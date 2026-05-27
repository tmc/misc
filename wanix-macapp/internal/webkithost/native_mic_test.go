package webkithost

import (
	"strings"
	"testing"

	"github.com/tmc/apple/avfoundation"
	"github.com/tmc/misc/wanix-macapp/internal/applefs"
)

func TestMicAuthStatusText(t *testing.T) {
	tests := []struct {
		status avfoundation.AVAuthorizationStatus
		want   string
	}{
		{avfoundation.AVAuthorizationStatusAuthorized, "authorized"},
		{avfoundation.AVAuthorizationStatusDenied, "denied"},
		{avfoundation.AVAuthorizationStatusRestricted, "restricted"},
		{avfoundation.AVAuthorizationStatusNotDetermined, "not-determined"},
		{avfoundation.AVAuthorizationStatus(99), "unknown-99"},
	}
	for _, tt := range tests {
		if got := micAuthStatusText(tt.status); got != tt.want {
			t.Fatalf("micAuthStatusText(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestFormatMicJSON(t *testing.T) {
	got, err := formatMicJSON([]micDevice{{ID: "default", Name: "Default", Connected: true, Default: true}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "" || got[len(got)-1] != '\n' {
		t.Fatalf("formatted JSON = %q", got)
	}
}

func TestMicSessionDuration(t *testing.T) {
	h := &Host{appleFS: applefs.NewRoot()}
	id := strings.TrimSpace(readAppleFSString(h, "mic/clone"))
	if err := h.applyMicSession("mic/"+id+"/ctl", "duration nope"); err == nil {
		t.Fatal("bad duration succeeded")
	}
	if err := h.applyMicSession("mic/"+id+"/ctl", "duration 250ms"); err != nil {
		t.Fatal(err)
	}
	if got := readAppleFSString(h, "mic/"+id+"/duration"); got != "250ms\n" {
		t.Fatalf("duration = %q", got)
	}
}

func TestClampStreamFPS(t *testing.T) {
	for _, tt := range []struct {
		in   int
		want int
	}{
		{0, 1},
		{2, 2},
		{99, 30},
	} {
		if got := clampStreamFPS(tt.in); got != tt.want {
			t.Fatalf("clampStreamFPS(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
