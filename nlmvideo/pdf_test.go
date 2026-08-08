package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name string
		in   string
		cols int
		want []string
	}{
		{"empty", "", 10, nil},
		{"single", "hello", 10, []string{"hello"}},
		{"split", "aaa bbb ccc", 7, []string{"aaa bbb", "ccc"}},
		{"long word", "abcdefghijkl", 5, []string{"abcdefghijkl"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrap(tt.in, tt.cols)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("wrap(%q, %d) = %v, want %v", tt.in, tt.cols, got, tt.want)
			}
		})
	}
}

func TestEscapePDF(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"a(b)c", `a\(b\)c`},
		{`back\slash`, `back\\slash`},
		{"smart’quote", "smart?quote"},
	}
	for _, tt := range tests {
		if got := escapePDF(tt.in); got != tt.want {
			t.Errorf("escapePDF(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJPEGSize(t *testing.T) {
	// Minimal JPEG: SOI, SOF0 with 2x3 dimensions.
	jpg := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, 0x00, 0x0B, 0x08, // SOF0, length 11, precision 8
		0x00, 0x02, // height 2
		0x00, 0x03, // width 3
		0x01, 0x00, 0x00, 0x00, // 1 component
	}
	w, h, err := jpegSize(jpg)
	if err != nil {
		t.Fatal(err)
	}
	if w != 3 || h != 2 {
		t.Errorf("jpegSize = %dx%d, want 3x2", w, h)
	}
	if _, _, err := jpegSize([]byte{0, 1, 2}); err == nil {
		t.Error("jpegSize on garbage: want error")
	}
}

func TestDedupe(t *testing.T) {
	got := dedupe([]float64{5, 5.5, 10, 30, 31, 100}, 3)
	want := []float64{5, 10, 30, 100}
	if len(got) != len(want) {
		t.Fatalf("dedupe = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dedupe[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBucketWords(t *testing.T) {
	words := []word{
		{"hello", 0}, {"world", 5_000}, {"next", 21_000}, {"bucket", 39_000},
	}
	b := bucketWords(words, 20)
	if b[0] != "hello world" || b[20] != "next bucket" {
		t.Errorf("bucketWords = %v", b)
	}
}

func TestParseJSON3(t *testing.T) {
	data := []byte(`{"events":[{"tStartMs":1000,"segs":[{"utf8":"hello ","tOffsetMs":0},{"utf8":"world","tOffsetMs":500}]}]}`)
	words, err := parseJSON3(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 || words[0].Text != "hello" || words[1].Ms != 1500 {
		t.Errorf("parseJSON3 = %+v", words)
	}
}

func TestVideoID(t *testing.T) {
	for _, u := range []string{
		"https://youtu.be/vUuK4Knl7oc",
		"https://www.youtube.com/watch?v=vUuK4Knl7oc&feature=x",
		"https://youtube.com/shorts/vUuK4Knl7oc",
	} {
		id, err := videoID(u)
		if err != nil || id != "vUuK4Knl7oc" {
			t.Errorf("videoID(%q) = %q, %v", u, id, err)
		}
	}
	if _, err := videoID("https://example.com/"); err == nil {
		t.Error("videoID on non-YouTube URL: want error")
	}
}

func TestPDFStructure(t *testing.T) {
	var w pdfWriter
	font := w.add([]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"), nil)
	if font != 3 {
		t.Fatalf("first added object = %d, want 3", font)
	}
	w.set(2, []byte("<< /Type /Pages /Kids [] /Count 0 >>"))
	w.set(1, []byte("<< /Type /Catalog /Pages 2 0 R >>"))
	out := w.bytes()
	for _, want := range []string{"%PDF-1.4", "1 0 obj", "3 0 obj", "trailer", "startxref", "%%EOF"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("pdf missing %q", want)
		}
	}
}
