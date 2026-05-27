package webkithost

import (
	"testing"
	"time"

	"github.com/tmc/apple/usernotifications"
)

func TestNotifyStatusText(t *testing.T) {
	tests := []struct {
		status usernotifications.UNAuthorizationStatus
		text   string
		ok     bool
	}{
		{usernotifications.UNAuthorizationStatusAuthorized, "authorized", true},
		{usernotifications.UNAuthorizationStatusDenied, "denied", false},
		{usernotifications.UNAuthorizationStatusEphemeral, "ephemeral", true},
		{usernotifications.UNAuthorizationStatusNotDetermined, "not-determined", false},
		{usernotifications.UNAuthorizationStatusProvisional, "provisional", true},
		{usernotifications.UNAuthorizationStatus(99), "unknown-99", false},
	}
	for _, tt := range tests {
		if got := notifyStatusText(tt.status); got != tt.text {
			t.Fatalf("notifyStatusText(%v) = %q, want %q", tt.status, got, tt.text)
		}
		if got := notifyAuthorized(tt.status); got != tt.ok {
			t.Fatalf("notifyAuthorized(%v) = %v, want %v", tt.status, got, tt.ok)
		}
	}
}

func TestNotifyDelay(t *testing.T) {
	tests := []struct {
		text string
		want time.Duration
	}{
		{"", time.Second},
		{"0s", time.Second},
		{"-1s", time.Second},
		{"250ms", 250 * time.Millisecond},
		{"2s", 2 * time.Second},
	}
	for _, tt := range tests {
		got, err := notifyDelay(tt.text)
		if err != nil {
			t.Fatalf("notifyDelay(%q): %v", tt.text, err)
		}
		if got != tt.want {
			t.Fatalf("notifyDelay(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
	if _, err := notifyDelay("bad"); err == nil {
		t.Fatal("bad delay succeeded")
	}
}

func TestNotifyRequestID(t *testing.T) {
	if got := notifyRequestID("12"); got != "wanix-12" {
		t.Fatalf("notifyRequestID = %q", got)
	}
}

func TestNotifySound(t *testing.T) {
	if got := notifySound("none"); got.GetID() != 0 {
		t.Fatalf("none sound id = %v", got.GetID())
	}
	if got := notifySound("silent"); got.GetID() != 0 {
		t.Fatalf("silent sound id = %v", got.GetID())
	}
}
