package main

import (
	"fmt"
	"strings"
)

// extractSerializedFile finds the bytes argument passed to AddSerializedFile in
// a generated _pb2.py file and decodes it to the raw FileDescriptorProto bytes.
//
// Modern protoc emits a single call of the form:
//
//	DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x13tinker.proto...')
//
// The literal may be a single b'...'/b"..." string or several adjacent string
// literals that Python concatenates implicitly. Both forms are handled.
func extractSerializedFile(src string) ([]byte, error) {
	i := strings.Index(src, "AddSerializedFile(")
	if i < 0 {
		return nil, fmt.Errorf("no AddSerializedFile(...) call found")
	}
	p := i + len("AddSerializedFile(")
	lit, err := readConcatenatedBytesLiterals(src, p)
	if err != nil {
		return nil, fmt.Errorf("reading AddSerializedFile argument: %w", err)
	}
	return lit, nil
}

// readConcatenatedBytesLiterals reads one or more adjacent Python bytes string
// literals starting at or after position p (skipping whitespace/newlines and
// line-continuation backslashes between them), decoding and concatenating them.
// It stops at the first non-literal token (normally the closing ')').
func readConcatenatedBytesLiterals(src string, p int) ([]byte, error) {
	var out []byte
	found := false
	for p < len(src) {
		// Skip whitespace, newlines, and line-continuation backslashes.
		for p < len(src) {
			c := src[p]
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\\' {
				p++
				continue
			}
			break
		}
		if p >= len(src) {
			break
		}
		// A bytes literal is an optional b/B prefix then a quote. We also
		// tolerate raw-bytes prefixes (rb/br) defensively.
		start := p
		prefixLen := bytesPrefixLen(src[p:])
		if prefixLen < 0 {
			break // not another literal — done
		}
		q := p + prefixLen
		if q >= len(src) || (src[q] != '\'' && src[q] != '"') {
			// 'b' that wasn't actually a literal; rewind and stop.
			p = start
			break
		}
		quote := src[q]
		raw, np, err := scanQuoted(src, q+1, quote)
		if err != nil {
			return nil, err
		}
		dec, err := decodePyBytes(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, dec...)
		found = true
		p = np
	}
	if !found {
		return nil, fmt.Errorf("expected a bytes literal")
	}
	return out, nil
}

// bytesPrefixLen returns the length of a Python bytes-literal prefix at the
// start of s (e.g. "b", "B", "rb", "br"), or -1 if s does not start a bytes
// literal. A bare quote is not a bytes literal (that's a str), so prefix is
// required.
func bytesPrefixLen(s string) int {
	if s == "" {
		return -1
	}
	// Look at up to two leading letters.
	n := 0
	hasB := false
	for n < 2 && n < len(s) {
		c := s[n] | 0x20 // lowercase
		if c == 'b' {
			hasB = true
			n++
			continue
		}
		if c == 'r' {
			n++
			continue
		}
		break
	}
	if !hasB {
		return -1
	}
	return n
}

// scanQuoted returns the raw (still-escaped) contents of a quoted string that
// starts just after an opening quote at position p, and the index just past the
// closing quote. Triple-quoted strings are supported. Backslash escapes are
// left intact for decodePyBytes.
func scanQuoted(src string, p int, quote byte) (string, int, error) {
	triple := p+1 < len(src) && src[p] == quote && src[p+1] == quote
	if triple {
		p += 2
		end := strings.Index(src[p:], string([]byte{quote, quote, quote}))
		if end < 0 {
			return "", 0, fmt.Errorf("unterminated triple-quoted literal")
		}
		return src[p : p+end], p + end + 3, nil
	}
	var b strings.Builder
	for p < len(src) {
		c := src[p]
		if c == '\\' {
			if p+1 >= len(src) {
				return "", 0, fmt.Errorf("trailing backslash in literal")
			}
			b.WriteByte(c)
			b.WriteByte(src[p+1])
			p += 2
			continue
		}
		if c == quote {
			return b.String(), p + 1, nil
		}
		b.WriteByte(c)
		p++
	}
	return "", 0, fmt.Errorf("unterminated literal")
}

// decodePyBytes decodes the contents of a Python bytes literal (the text between
// the quotes, with escapes intact) into raw bytes, applying Python's bytes
// escape rules: \xNN (hex), \NNN (octal, 1-3 digits), \n \t \r \\ \' \" \a \b
// \f \v \0, and a backslash before any other char is kept literally (Python
// keeps the backslash for unknown escapes in bytes literals).
func decodePyBytes(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '\\' {
			out = append(out, c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return nil, fmt.Errorf("trailing backslash")
		}
		e := s[i]
		switch e {
		case 'x':
			if i+2 >= len(s) {
				return nil, fmt.Errorf("truncated \\x escape")
			}
			v, err := hex2(s[i+1], s[i+2])
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			i += 3
		case 'n':
			out = append(out, '\n')
			i++
		case 't':
			out = append(out, '\t')
			i++
		case 'r':
			out = append(out, '\r')
			i++
		case '\\':
			out = append(out, '\\')
			i++
		case '\'':
			out = append(out, '\'')
			i++
		case '"':
			out = append(out, '"')
			i++
		case 'a':
			out = append(out, 0x07)
			i++
		case 'b':
			out = append(out, 0x08)
			i++
		case 'f':
			out = append(out, 0x0c)
			i++
		case 'v':
			out = append(out, 0x0b)
			i++
		case '\n':
			// line continuation inside the literal: drop it
			i++
		default:
			if e >= '0' && e <= '7' {
				// Octal escape: 1 to 3 digits.
				j := i
				val := 0
				k := 0
				for k < 3 && j < len(s) && s[j] >= '0' && s[j] <= '7' {
					val = val*8 + int(s[j]-'0')
					j++
					k++
				}
				out = append(out, byte(val))
				i = j
			} else {
				// Unknown escape: Python keeps the backslash in bytes.
				out = append(out, '\\', e)
				i++
			}
		}
	}
	return out, nil
}

func hex2(a, b byte) (byte, error) {
	hi, ok1 := unhex(a)
	lo, ok2 := unhex(b)
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("invalid \\x escape \\x%c%c", a, b)
	}
	return hi<<4 | lo, nil
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
