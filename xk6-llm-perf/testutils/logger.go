package testutils

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

type Logger = logrus.FieldLogger

// NewLogger creates a new logger for testing that outputs to test logs
func NewLogger(t *testing.T) Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard logs in tests
	logger.AddHook(&testLogHook{t: t})
	return logger
}

type testLogHook struct {
	t *testing.T
}

func (h *testLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *testLogHook) Fire(entry *logrus.Entry) error {
	// Log to test output
	h.t.Logf("[%s] %s", entry.Level, entry.Message)
	return nil
}
