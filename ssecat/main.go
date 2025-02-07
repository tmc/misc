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

// message represents an SSE JSON content block.
type message struct {
	Type  string
	Index int
	Delta struct {
		Type string
		Text string
	}
}

var (
	delay  time.Duration
	inFile string
)

func init() {
	flag.DurationVar(&delay, "delay", 0, "delay between chunks (e.g. 100ms)")
	flag.StringVar(&inFile, "f", "-", "input file (- for stdin)")
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

		var m message
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m); err != nil {
			return fmt.Errorf("json unmarshal: %w", err)
		}

		if m.Type == "content_block_delta" && m.Delta.Type == "text_delta" {
			hasOutput = true
			if _, err := fmt.Fprint(w, m.Delta.Text); err != nil {
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
