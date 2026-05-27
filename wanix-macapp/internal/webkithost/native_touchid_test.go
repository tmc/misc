package webkithost

import (
	"testing"

	"github.com/tmc/apple/localauthentication"
)

func TestBiometryText(t *testing.T) {
	tests := []struct {
		kind localauthentication.LABiometryType
		want string
	}{
		{localauthentication.LABiometryTypeTouchID, "touchid"},
		{localauthentication.LABiometryTypeFaceID, "faceid"},
		{localauthentication.LABiometryTypeOpticID, "opticid"},
		{localauthentication.LABiometryTypeNone, "none"},
		{localauthentication.LABiometryType(99), "unknown-99"},
	}
	for _, tt := range tests {
		if got := biometryText(tt.kind); got != tt.want {
			t.Fatalf("biometryText(%v) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
