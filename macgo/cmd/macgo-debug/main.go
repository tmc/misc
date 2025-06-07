// Command macgo-debug is a utility for debugging macgo applications
// It provides advanced debugging and monitoring capabilities for macgo processes
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	// Command-line flags
	flagPid       = flag.Int("pid", 0, "Target process ID")
	flagSignal    = flag.String("signal", "INT", "Signal to send (INT, TERM, etc)")
	flagMonitor   = flag.Bool("monitor", false, "Monitor the process")
	flagPprof     = flag.Bool("pprof", false, "Start pprof server for this process")
	flagPprofPort = flag.Int("pprof-port", 6060, "Port for pprof server")
	flagVerbose   = flag.Bool("verbose", false, "Enable verbose logging")
	flagTrace     = flag.Bool("trace", false, "Trace all signals")
	flagHelp      = flag.Bool("help", false, "Show help message")
)

func main() {
	// Parse command-line flags
	flag.Parse()

	// Show help message if requested
	if *flagHelp {
		showHelp()
		return
	}

	// Start pprof server if requested
	if *flagPprof {
		go startPprofServer(*flagPprofPort)
	}

	// Send signal to process if pid is provided
	if *flagPid > 0 {
		if err := sendSignal(*flagPid, *flagSignal); err != nil {
			fmt.Fprintf(os.Stderr, "Error sending signal: %v\n", err)
			os.Exit(1)
		}
	}

	// Monitor process if requested
	if *flagMonitor && *flagPid > 0 {
		monitorProcess(*flagPid)
	}
}

// showHelp displays usage information
func showHelp() {
	fmt.Println("macgo-debug: Advanced debugging utility for macgo applications")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  macgo-debug [options]")
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  Send SIGINT to process:        macgo-debug --pid=1234 --signal=INT")
	fmt.Println("  Monitor process:               macgo-debug --pid=1234 --monitor")
	fmt.Println("  Start pprof server:            macgo-debug --pprof --pprof-port=6060")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  MACGO_DEBUG=1                 Enable debug logging for macgo processes")
	fmt.Println("  MACGO_SIGNAL_DEBUG=1          Enable detailed signal tracing")
	fmt.Println("  MACGO_DEBUG_LEVEL=2           Set debug verbosity level (0-3)")
	fmt.Println("  MACGO_PPROF=1                 Enable pprof HTTP server in macgo processes")
	fmt.Println("  MACGO_PPROF_PORT=6060         Set base port for pprof HTTP server")
}

// sendSignal sends a signal to the specified process
func sendSignal(pid int, signalName string) error {
	// Convert signal name to signal number
	sig, err := parseSignal(signalName)
	if err != nil {
		return err
	}

	// Print signal info
	fmt.Printf("Sending signal %s (%d) to process %d\n", signalName, sig, pid)

	// Send signal to process
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process: %w", err)
	}

	return proc.Signal(sig)
}

// parseSignal converts a signal name to a signal number
func parseSignal(name string) (os.Signal, error) {
	// Remove SIG prefix if present
	name = strings.TrimPrefix(strings.ToUpper(name), "SIG")

	// Map signal names to signal numbers
	switch name {
	case "INT":
		return syscall.SIGINT, nil
	case "TERM":
		return syscall.SIGTERM, nil
	case "KILL":
		return syscall.SIGKILL, nil
	case "HUP":
		return syscall.SIGHUP, nil
	case "USR1":
		return syscall.SIGUSR1, nil
	case "USR2":
		return syscall.SIGUSR2, nil
	case "QUIT":
		return syscall.SIGQUIT, nil
	default:
		// Try to parse as a number
		if num, err := strconv.Atoi(name); err == nil {
			return syscall.Signal(num), nil
		}
		return nil, fmt.Errorf("unknown signal: %s", name)
	}
}

// monitorProcess monitors a process and its children
func monitorProcess(pid int) {
	fmt.Printf("Monitoring process %d\n", pid)

	// Check if process exists
	if _, err := os.FindProcess(pid); err != nil {
		fmt.Fprintf(os.Stderr, "Error finding process: %v\n", err)
		return
	}

	// Set up signal handling for graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if process is still running
			if !isProcessRunning(pid) {
				fmt.Printf("Process %d has exited\n", pid)
				return
			}

			// Print process information
			if *flagVerbose {
				printProcessInfo(pid)
			}

		case sig := <-sigCh:
			fmt.Printf("Received signal %v, stopping monitoring\n", sig)
			return
		}
	}
}

// isProcessRunning checks if a process is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// printProcessInfo prints information about a process
func printProcessInfo(pid int) {
	// Run ps command to get process info
	cmd := exec.Command("ps", "-o", "pid,ppid,pgid,%cpu,%mem,state,command", "-p", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting process info: %v\n", err)
		return
	}

	fmt.Println(strings.TrimSpace(string(output)))
}

// startPprofServer starts a pprof HTTP server
func startPprofServer(port int) {
	// Try multiple ports if the default one is in use
	for i := 0; i < 10; i++ {
		addr := fmt.Sprintf("localhost:%d", port)
		fmt.Printf("Starting pprof server on %s\n", addr)

		// Add custom handler for process info
		http.HandleFunc("/macgo/info", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "macgo-debug utility\n")
			fmt.Fprintf(w, "PID: %d\n", os.Getpid())
			fmt.Fprintf(w, "Target PID: %d\n", *flagPid)
			fmt.Fprintf(w, "Time: %s\n", time.Now().Format(time.RFC3339))
		})

		// Print usage info
		fmt.Printf("pprof endpoints available at:\n")
		fmt.Printf("  http://%s/debug/pprof/\n", addr)
		fmt.Printf("  Process info: http://%s/macgo/info\n", addr)

		err := http.ListenAndServe(addr, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start pprof server on port %d: %v\n", port, err)
			// Try next port
			port++
			continue
		}
		break
	}
}
