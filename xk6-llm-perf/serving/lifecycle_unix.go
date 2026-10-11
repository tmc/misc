//go:build darwin || linux

package serving

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func stopGroup(cmd *exec.Cmd, done <-chan error, timeout time.Duration) error {
	pid := cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
	// Always kill remaining descendants, including those surviving leader exit.
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("owned process did not reap after kill")
	}
}
