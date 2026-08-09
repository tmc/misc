package main

import (
	"image"
	"image/color"
	"regexp"
	"testing"

	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/x/vzkit/ocr"
)

func mustPatterns(t *testing.T, custom []string, builtin bool) []*regexp.Regexp {
	t.Helper()
	ps, err := compilePatterns(custom, builtin)
	if err != nil {
		t.Fatalf("compilePatterns: %v", err)
	}
	return ps
}

func TestIsSensitiveBuiltin(t *testing.T) {
	patterns := mustPatterns(t, nil, true)
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "publishable", text: "pk_test_ABCDEFGHIJKLMNOP1234", want: true},
		{name: "secret", text: "sk_test_ABCDEFGHIJKLMNOP1234", want: true},
		{name: "ocr spaces", text: "pk test ABCDEFGHIJKLMNOP1234", want: true},
		{name: "ocr mixed", text: "sk test_ABCDEFGHIJKLMNOP1234", want: true},
		{name: "embedded in sentence", text: "key: sk_test_ABCDEFGHIJKLMNOP1234 here", want: true},
		{name: "short body rejected", text: "pk_test_12345", want: false},
		{name: "desktop prose", text: "desktop testing guide", want: false},
		{name: "backpack prose", text: "Backpack test notes", want: false},
		{name: "plain text", text: "hello world", want: false},
		{name: "wrong prefix", text: "xk_test_ABCDEFGHIJKLMNOP1234", want: false},
	}
	for _, tt := range tests {
		if got := isSensitive(tt.text, patterns); got != tt.want {
			t.Errorf("%s: isSensitive(%q) = %v, want %v", tt.name, tt.text, got, tt.want)
		}
	}
}

func TestCompilePatternsCustom(t *testing.T) {
	patterns := mustPatterns(t, []string{`SECRET-\d{4}`}, false)
	if !isSensitive("SECRET-1234", patterns) {
		t.Error("custom pattern did not match SECRET-1234")
	}
	if isSensitive("pk_test_ABCDEFGHIJKLMNOP1234", patterns) {
		t.Error("builtin should be disabled when custom patterns set and -builtin=false")
	}
}

func TestCompilePatternsCustomWithBuiltin(t *testing.T) {
	patterns := mustPatterns(t, []string{`SECRET-\d{4}`}, true)
	if !isSensitive("SECRET-1234", patterns) {
		t.Error("custom pattern did not match")
	}
	if !isSensitive("pk_test_ABCDEFGHIJKLMNOP1234", patterns) {
		t.Error("builtin pattern should still match when -builtin=true")
	}
}

func TestCompilePatternsInvalid(t *testing.T) {
	if _, err := compilePatterns([]string{"(unclosed"}, false); err == nil {
		t.Fatal("compilePatterns([unclosed], false) succeeded, want error")
	}
	if _, err := compilePatterns(nil, false); err == nil {
		t.Fatal("compilePatterns(nil, false) succeeded, want error (no patterns)")
	}
}

func TestMatchedRects(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 100)
	obs := []ocr.TextObservation{
		{
			Text: "pk_test_ABCDEFGHIJKLMNOP1234",
			BoundingBox: corefoundation.CGRect{
				Origin: corefoundation.CGPoint{X: 0.25, Y: 0.25},
				Size:   corefoundation.CGSize{Width: 0.5, Height: 0.25},
			},
		},
	}
	got := matchedRects(obs, bounds, 0, mustPatterns(t, nil, true))
	want := image.Rect(50, 50, 150, 75)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("matchedRects = %v, want [%v]", got, want)
	}
}

func TestMergeRects(t *testing.T) {
	rects := []image.Rectangle{
		image.Rect(0, 0, 10, 10),
		image.Rect(9, 2, 20, 12),
		image.Rect(50, 50, 60, 60),
	}
	got := mergeRects(rects)
	if len(got) != 2 {
		t.Fatalf("mergeRects len = %d, want 2", len(got))
	}
	if got[0] != image.Rect(0, 0, 20, 12) && got[1] != image.Rect(0, 0, 20, 12) {
		t.Fatalf("mergeRects missing union: %v", got)
	}
}

func TestApplyBlurRedactionChangesRegion(t *testing.T) {
	oldStyle, oldBlur := *styleFlag, *blurFlag
	*styleFlag, *blurFlag = "blur", 3
	defer func() {
		*styleFlag, *blurFlag = oldStyle, oldBlur
	}()

	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: uint8((x + y) * 5), A: 255})
		}
	}
	before := img.RGBAAt(10, 10)
	applyRedaction(img, image.Rect(4, 4, 16, 16), color.Black)
	after := img.RGBAAt(10, 10)
	if before == after {
		t.Fatalf("blur redaction did not change pixel: before=%v after=%v", before, after)
	}
}

func TestValidateStyle(t *testing.T) {
	if err := validateStyle("blur"); err != nil {
		t.Fatalf("validateStyle(blur): %v", err)
	}
	if err := validateStyle("solid"); err != nil {
		t.Fatalf("validateStyle(solid): %v", err)
	}
	if err := validateStyle("nope"); err == nil {
		t.Fatal("validateStyle(nope) succeeded, want error")
	}
}
