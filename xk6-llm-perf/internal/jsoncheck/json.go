package jsoncheck

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
)

// Admission errors distinguish checks from encoding/json syntax errors.
var (
	ErrDepth     = errors.New("JSON nesting exceeds budget")
	ErrDuplicate = errors.New("duplicate JSON member")
	ErrNull      = errors.New("explicit null")
	ErrTrailing  = errors.New("trailing JSON data")
)

// Options controls member identity and null admission. Nesting is bounded at 64.
type Options struct {
	FoldNames  bool // Match encoding/json's case-insensitive typed field names.
	RejectNull bool
}

// Check checks one complete JSON document. The caller controls number decoding
// with Decoder.UseNumber and bounds the input size.
func Check(d *json.Decoder, opts Options) error {
	if err := Value(d, opts); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrTrailing
	}
	return nil
}

// Value checks one value, leaving any following value for the caller.
func Value(d *json.Decoder, opts Options) error { return value(d, opts, 0) }

func value(d *json.Decoder, opts Options, depth int) error {
	if depth > 64 {
		return ErrDepth
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token == nil && opts.RejectNull {
		return ErrNull
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("invalid JSON delimiter")
	}
	var seen map[string]bool
	if delim == '{' {
		seen = make(map[string]bool)
	}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if opts.FoldNames {
				name = fold(name)
			}
			if !ok || seen[name] {
				return ErrDuplicate
			}
			seen[name] = true
		}
		if err := value(d, opts, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func fold(name string) string {
	return strings.Map(func(r rune) rune {
		for {
			next := unicode.SimpleFold(r)
			if next <= r {
				return next
			}
			r = next
		}
	}, name)
}
