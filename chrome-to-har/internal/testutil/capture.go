package testutil

import (
	"bytes"
	"io"
	"log"
	"os"
)

// OutputCapture captures stdout and stderr during a test
type OutputCapture struct {
	stdout     *os.File
	stderr     *os.File
	logBuf     *bytes.Buffer
	outBuf     *bytes.Buffer
	oldLog     *log.Logger
	oldStdout  *os.File
	oldStderr  *os.File
	origLogger *log.Logger
}

// NewOutputCapture creates a new output capture
func NewOutputCapture() (*OutputCapture, error) {
	logBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}

	origLogger := log.Default()
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	os.Stdout = w
	os.Stderr = w
	log.SetOutput(logBuf)

	oc := &OutputCapture{
		stdout:     w,
		logBuf:     logBuf,
		outBuf:     outBuf,
		oldLog:     log.Default(),
		oldStdout:  oldStdout,
		oldStderr:  oldStderr,
		origLogger: origLogger,
	}

	go func() {
		io.Copy(outBuf, r)
	}()

	return oc, nil
}

// Stop captures and returns the output
func (oc *OutputCapture) Stop() (stdout, logs string) {
	oc.stdout.Close()
	os.Stdout = oc.oldStdout
	os.Stderr = oc.oldStderr
	log.SetOutput(oc.origLogger.Writer())
	return oc.outBuf.String(), oc.logBuf.String()
}

