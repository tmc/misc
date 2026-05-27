package webkithost

import "testing"

func TestIndicatorID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"appkit/indicator/1/ctl", "1"},
		{"appkit/indicator/menu/text", "menu"},
		{"appkit/window/title", ""},
		{"indicator/1/ctl", ""},
	}
	for _, tt := range tests {
		if got := indicatorID(tt.name); got != tt.want {
			t.Fatalf("indicatorID(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
