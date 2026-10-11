//go:build unix

package ownedprocess

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ErrCleanup reports that pipe cleanup exceeded its one-second budget.
var ErrCleanup = errors.New("process pipe cleanup exceeded budget")

// Run runs cmd in its own process group, kills descendants when the leader exits,
// and waits for output copies before returning. cmd must be created with
// exec.CommandContext. Its input reader and output writers must not block
// independently of the child process.
func Run(cmd *exec.Cmd) error {
	// Give exec direct files so Wait reports leader exit without waiting for
	// descendants that inherited its pipes. Drain stdout concurrently, and
	// complete pipe cleanup after killing the group at leader exit.
	input, inputWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = input.Close(); _ = inputWriter.Close() }()
	output, outputWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = output.Close(); _ = outputWriter.Close() }()
	errorOutput, errorWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = errorOutput.Close(); _ = errorWriter.Close() }()
	in, out, stderr := cmd.Stdin, cmd.Stdout, cmd.Stderr
	if out == nil {
		out = io.Discard
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, outputWriter, errorWriter
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = input.Close()
	_ = outputWriter.Close()
	_ = errorWriter.Close()
	inputDone, outputDone, errorDone := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	go func() {
		var err error
		if in != nil {
			_, err = io.Copy(inputWriter, in)
		}
		_ = inputWriter.Close()
		inputDone <- err
	}()
	go func() {
		_, err := io.Copy(out, output)
		_ = output.Close()
		outputDone <- err
	}()
	go func() {
		if stderr == nil {
			stderr = io.Discard
		}
		_, err := io.Copy(stderr, errorOutput)
		_ = errorOutput.Close()
		errorDone <- err
	}()
	waitErr := cmd.Wait()
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = inputWriter.Close()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	var copyErr error
	for inputDone != nil || outputDone != nil || errorDone != nil {
		select {
		case err := <-inputDone:
			inputDone = nil
			if err != nil && !errors.Is(copyErr, ErrCleanup) {
				copyErr = err
			}
		case err := <-outputDone:
			outputDone = nil
			if err != nil && !errors.Is(copyErr, ErrCleanup) {
				copyErr = err
			}
		case err := <-errorDone:
			errorDone = nil
			if err != nil && !errors.Is(copyErr, ErrCleanup) {
				copyErr = err
			}
		case <-timer.C:
			// Closing our readers unblocks the copies; collect both completions
			// before callers inspect the bounded buffers.
			_ = output.Close()
			_ = errorOutput.Close()
			copyErr = ErrCleanup
		}
	}
	if waitErr != nil {
		return waitErr
	}
	return copyErr
}
