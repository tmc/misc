package webkithost

import (
	"strings"
	"testing"
)

func TestFormatScreenJSON(t *testing.T) {
	got, err := formatScreenJSON([]screenDisplay{{
		ID:     7,
		X:      1,
		Y:      2,
		Width:  1440,
		Height: 900,
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{`"id": 7`, `"width": 1440`, "\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted JSON = %q, missing %q", text, want)
		}
	}
	if got[len(got)-1] != '\n' {
		t.Fatalf("formatted JSON does not end in newline: %q", got)
	}
}

func TestScreenContentFilterRejectsBadTargets(t *testing.T) {
	for _, target := range []string{"display x", "display", "window -1", "bogus 1"} {
		if _, err := screenContentFilter(nil, target); err == nil {
			t.Fatalf("screenContentFilter(%q) succeeded", target)
		}
	}
}

func TestScreenFrameRecord(t *testing.T) {
	got := screenFrameRecord(3, []byte("png"))
	want := "frame 3 bytes 3\npng\n"
	if string(got) != want {
		t.Fatalf("record = %q, want %q", got, want)
	}
}
