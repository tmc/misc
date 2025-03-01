package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

const (
	// DirectionIncoming represents messages from the client to the server
	DirectionIncoming = ">"
	// DirectionOutgoing represents messages from the server to the client
	DirectionOutgoing = "<"
)

// Spy is a tool for recording MCP traffic.
type Spy struct {
	input     io.Reader
	output    io.Writer
	recorder  io.Writer
	verbose   bool
	scanner   *bufio.Scanner
	writer    *bufio.Writer
	recWriter *bufio.Writer
}

// NewSpy creates a new Spy.
func NewSpy(input io.Reader, output io.Writer, recorder io.Writer, verbose bool) (*Spy, error) {
	return &Spy{
		input:     input,
		output:    output,
		recorder:  recorder,
		verbose:   verbose,
		scanner:   bufio.NewScanner(input),
		writer:    bufio.NewWriter(output),
		recWriter: bufio.NewWriter(recorder),
	}, nil
}

// Start begins the spying process, reading from input and writing to output.
func (s *Spy) Start() error {
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if err := s.processLine(line, DirectionIncoming); err \!= nil {
			return err
		}
	}
	if err := s.scanner.Err(); err \!= nil {
		return fmt.Errorf("scanner error: %v", err)
	}
	return nil
}

// processLine handles a single line of MCP traffic.
func (s *Spy) processLine(line string, direction string) error {
	// Record the line
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	recordLine := fmt.Sprintf("%s %s %s\n", timestamp, direction, line)
	if _, err := s.recWriter.WriteString(recordLine); err \!= nil {
		return fmt.Errorf("failed to write to recorder: %v", err)
	}
	if err := s.recWriter.Flush(); err \!= nil {
		return fmt.Errorf("failed to flush recorder: %v", err)
	}

	// Log if verbose
	if s.verbose {
		var msg json.RawMessage
		if err := json.Unmarshal([]byte(line), &msg); err \!= nil {
			log.Printf("Warning: Invalid JSON: %v", err)
		} else {
			pretty, _ := json.MarshalIndent(msg, "", "  ")
			log.Printf("%s %s", direction, string(pretty))
		}
	}

	// Forward to output
	if _, err := s.writer.WriteString(line + "\n"); err \!= nil {
		return fmt.Errorf("failed to write to output: %v", err)
	}
	if err := s.writer.Flush(); err \!= nil {
		return fmt.Errorf("failed to flush output: %v", err)
	}

	return nil
}

// Close cleans up resources.
func (s *Spy) Close() error {
	return s.recWriter.Flush()
}
