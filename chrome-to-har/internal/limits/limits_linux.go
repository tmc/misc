//go:build linux
// +build linux

package limits

import (
	"fmt"
	"syscall"
)

// setProcLimit sets the process limit on Linux
func setProcLimit(maxProcesses uint64) error {
	if maxProcesses > 0 {
		procLimit := &syscall.Rlimit{
			Cur: maxProcesses,
			Max: maxProcesses,
		}
		
		if err := syscall.Setrlimit(syscall.RLIMIT_NPROC, procLimit); err != nil {
			return fmt.Errorf("setting process limit: %w", err)
		}
	}
	return nil
}