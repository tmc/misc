package macgo

import (
	"os"
	"os/signal"
	"syscall"
)

// forwardSignals sets up signal forwarding from the parent process to the child process.
// It handles a comprehensive set of signals and forwards them to the process group
// of the provided process ID.
func forwardSignals(pid int) {
	sigCh := make(chan os.Signal, 16)
	// Handle every signal other than a few exceptions
	signal.Notify(sigCh,
		syscall.SIGABRT,
		syscall.SIGALRM,
		syscall.SIGBUS,
		syscall.SIGCHLD,
		syscall.SIGCONT,
		syscall.SIGFPE,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINT,
		syscall.SIGIO,
		syscall.SIGPIPE,
		syscall.SIGPROF,
		syscall.SIGQUIT,
		syscall.SIGSEGV,
		syscall.SIGSYS,
		syscall.SIGTERM,
		syscall.SIGTRAP,
		syscall.SIGTSTP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGVTALRM,
		syscall.SIGWINCH,
		syscall.SIGXCPU,
		syscall.SIGXFSZ,
		// explicitly not catching SIGKILL and SIGSTOP (cannot be caught)
	)

	// Forward signals to the app process group
	go func() {
		for sig := range sigCh {
			sigNum := sig.(syscall.Signal)

			// Skip SIGCHLD as we don't need to forward it
			if sigNum == syscall.SIGCHLD {
				continue
			}

			debugf("Forwarding signal %v to app bundle process group", sigNum)

			// Forward the signal to the process group of the child
			// Using negative PID sends to the entire process group
			if err := syscall.Kill(-pid, sigNum); err != nil {
				debugf("Error forwarding signal %v: %v", sigNum, err)
			}

			// If this is a terminal stop signal, we should also stop ourselves
			if sigNum == syscall.SIGTSTP || sigNum == syscall.SIGTTIN || sigNum == syscall.SIGTTOU {
				// Use SIGSTOP for these terminal signals
				syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
			}

			// Special handling for SIGINT and SIGTERM
			if sigNum == syscall.SIGINT || sigNum == syscall.SIGTERM {
				debugf("Received termination signal %v", sigNum)
				// Let the main process handle cleanup and exit
			}
		}
	}()
}

// setupSignalHandling sets up signal handling for a command
// It forwards all signals to the command and cleans up when done
func setupSignalHandling(cmd *os.Process) chan os.Signal {
	c := make(chan os.Signal, 100)
	signal.Notify(c)
	go func() {
		for sig := range c {
			debugf("Forwarding signal %v to process", sig)
			cmd.Signal(sig)
		}
	}()
	return c
}
