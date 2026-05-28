package main

import (
	"io"
	"os"
	"sync"

	oairt "github.com/tmc/misc/oairt"
)

// AppState holds CLI-level state shared across the audio, input, and event-
// handling goroutines.
type AppState struct {
	mu sync.Mutex

	Session           *oairt.Session
	AudioOutputFile   string
	AudioFile         *os.File
	AudioOutput       io.WriteCloser
	AudioEngine       AudioEngine
	AudioMutex        sync.Mutex
	DebugLevel        int
	DefaultSampleRate int
	DefaultBitDepth   int
	DefaultChannels   int
	ActualSampleRate  int
}

// CurrentSession returns the most recently observed session (or nil).
func (s *AppState) CurrentSession() *oairt.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Session
}

// SetSession atomically updates the cached session.
func (s *AppState) SetSession(sess *oairt.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Session = sess
}

// AudioWriteCloser extends io.WriteCloser with an IsClosed probe.
type AudioWriteCloser interface {
	io.WriteCloser
	IsClosed() bool
}
