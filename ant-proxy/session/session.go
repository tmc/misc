// Package session provides functionality for recording and managing API sessions.
//
// This package implements the ability to record API requests and responses, 
// store them in session files, and reload them for analysis or replay.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents a recorded API session.
type Session struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Host      string      `json:"host"`
	Requests  []APIRecord `json:"requests"`
}

// APIRecord represents a single API request-response pair.
type APIRecord struct {
	RequestTime  time.Time       `json:"request_time"`
	ResponseTime time.Time       `json:"response_time"`
	Request      json.RawMessage `json:"request"`
	Response     json.RawMessage `json:"response"`
	Error        string          `json:"error,omitempty"`
}

// New creates a new session recording with the given host.
func New(host string) *Session {
	return &Session{
		ID:        fmt.Sprintf("%s_%s", host, time.Now().Format("01-02-2006-15-04-05")),
		Timestamp: time.Now(),
		Host:      host,
		Requests:  []APIRecord{},
	}
}

// AddRecord adds a new API record to the session.
func (s *Session) AddRecord(request, response interface{}, err error) {
	reqJSON, _ := json.Marshal(request)
	respJSON, _ := json.Marshal(response)

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	s.Requests = append(s.Requests, APIRecord{
		RequestTime:  time.Now(),
		ResponseTime: time.Now(),
		Request:      reqJSON,
		Response:     respJSON,
		Error:        errStr,
	})
}

// Save saves the session to a file in the specified directory.
func (s *Session) Save(dirPath string) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the session file
	filePath := filepath.Join(dirPath, s.ID+".json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the session data to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s); err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	return nil
}

// Load loads a session from a file.
func Load(filePath string) (*Session, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var session Session
	if err := json.NewDecoder(file).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}