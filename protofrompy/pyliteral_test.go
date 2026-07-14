package main

import (
	"bytes"
	"testing"
)

func TestDecodePyBytes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"plain", "abc", []byte("abc")},
		{"hex", `\x00\xff\x7f`, []byte{0x00, 0xff, 0x7f}},
		{"named", `\n\t\r\\`, []byte{'\n', '\t', '\r', '\\'}},
		{"quotes", `\'\"`, []byte{'\'', '"'}},
		{"bell_bs_ff_vt", `\a\b\f\v`, []byte{0x07, 0x08, 0x0c, 0x0b}},
		{"octal_full", `\101\060`, []byte{'A', '0'}},
		{"octal_short", `\0`, []byte{0}},
		{"octal_then_digit", `\1014`, []byte{'A', '4'}}, // \101 = 'A', then literal '4'
		{"unknown_escape_keeps_backslash", `\q`, []byte{'\\', 'q'}},
		{"mixed", `\n\x13tinker`, append([]byte{'\n', 0x13}, []byte("tinker")...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePyBytes(tt.in)
			if err != nil {
				t.Fatalf("decodePyBytes(%q) error: %v", tt.in, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("decodePyBytes(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestBytesPrefixLen(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{`b'x'`, 1},
		{`B'x'`, 1},
		{`rb'x'`, 2},
		{`br'x'`, 2},
		{`'x'`, -1}, // str, not bytes
		{`r'x'`, -1},
		{``, -1},
	}
	for _, tt := range tests {
		if got := bytesPrefixLen(tt.in); got != tt.want {
			t.Errorf("bytesPrefixLen(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestExtractSerializedFile(t *testing.T) {
	// Single literal.
	src := "DESCRIPTOR = pool.AddSerializedFile(b'\\n\\x05hello')\n"
	got, err := extractSerializedFile(src)
	if err != nil {
		t.Fatalf("extractSerializedFile: %v", err)
	}
	if want := append([]byte{'\n', 0x05}, []byte("hello")...); !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Implicitly concatenated adjacent literals across lines (older protoc style).
	src2 := "AddSerializedFile(b'\\n\\x05hel'\n    b'lo')\n"
	got2, err := extractSerializedFile(src2)
	if err != nil {
		t.Fatalf("extractSerializedFile concat: %v", err)
	}
	if want := append([]byte{'\n', 0x05}, []byte("hello")...); !bytes.Equal(got2, want) {
		t.Errorf("concat got %v, want %v", got2, want)
	}

	// Double-quoted variant.
	src3 := `AddSerializedFile(b"\n\x05hello")`
	got3, err := extractSerializedFile(src3)
	if err != nil {
		t.Fatalf("extractSerializedFile dq: %v", err)
	}
	if want := append([]byte{'\n', 0x05}, []byte("hello")...); !bytes.Equal(got3, want) {
		t.Errorf("dq got %v, want %v", got3, want)
	}

	// No call present.
	if _, err := extractSerializedFile("x = 1\n"); err == nil {
		t.Error("expected error when AddSerializedFile is absent")
	}
}
