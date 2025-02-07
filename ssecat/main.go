// ssecat reads SSE JSON content blocks and writes text content.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var (
	delay    time.Duration
	inFile   string
	jsonPath string
)

func init() {
	flag.DurationVar(&delay, "delay", 0, "delay between chunks (e.g. 100ms)")
	flag.StringVar(&inFile, "f", "-", "input file (- for stdin)")
	flag.StringVar(&jsonPath, "path", "type=content_block_delta,delta.type=text_delta,delta.text", "path to text field (e.g. type=foo,text or a.b.type=foo,a.b.text)")
}

func main() {
	flag.Parse()

	var in *os.File
	var err error

	if inFile == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(inFile)
		if err != nil {
			log.Fatal(err)
		}
		defer in.Close()
	}

	if err := run(in, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(r io.Reader, w io.Writer) error {
	s := bufio.NewScanner(r)
	var hasOutput bool
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var m map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m); err != nil {
			return fmt.Errorf("json unmarshal: %w", err)
		}

		text, ok := getText(m, jsonPath)
		if ok {
			hasOutput = true
			if _, err := fmt.Fprint(w, text); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}

		if delay > 0 {
			time.Sleep(delay)
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if hasOutput {
		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}

// get follows a dot-separated path in a nested map and returns the value.
func get(m map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	curr := m
	for i, p := range parts {
		if i == len(parts)-1 {
			v, ok := curr[p]
			return v, ok
		}
		next, ok := curr[p]
		if !ok {
			return nil, false
		}
		curr, ok = next.(map[string]interface{})
		if !ok {
			return nil, false
		}
	}
	return nil, false
}

// getText extracts text from a JSON object using a path pattern.
// The path is a comma-separated list of paths, where all but the last must be key=value pairs,
// and the last path contains the text to extract.
func getText(m map[string]interface{}, pattern string) (string, bool) {
	parts := strings.Split(pattern, ",")
	if len(parts) < 2 {
		return "", false
	}

	// Check all key=value pairs except the last one
	for i := 0; i < len(parts)-1; i++ {
		kv := strings.Split(parts[i], "=")
		if len(kv) != 2 {
			return "", false
		}

		// Check if the key matches the value
		got, ok := get(m, kv[0])
		if !ok {
			return "", false
		}
		if s, ok := got.(string); !ok || s != kv[1] {
			return "", false
		}
	}

	// Get the text value
	text, ok := get(m, parts[len(parts)-1])
	if !ok {
		return "", false
	}
	s, ok := text.(string)
	return s, ok
}
