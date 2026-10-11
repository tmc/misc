//go:build unix

package llmperf

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/tmc/misc/xk6-llm-perf/internal/ownedprocess"
)

func openRuntimeTemplate(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) // #nosec G304 -- trusted operator-selected runtime/worker file; regular-file and SHA256 checks precede execution
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("close rejected runtime template worker")
		}
		return nil, fmt.Errorf("runtime template worker must be a regular file")
	}
	return f, nil
}

func runRuntimeTemplate(cmd *exec.Cmd) error {
	err := ownedprocess.Run(cmd)
	if errors.Is(err, ownedprocess.ErrCleanup) {
		return fmt.Errorf("runtime template pipe cleanup exceeded budget")
	}
	return err
}
