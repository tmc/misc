// Package debug provides advanced debugging capabilities for macgo.
// It includes detailed signal handling logs and pprof support.
package debug

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof" // Import for side effects to register pprof handlers
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	// Debug flags
	signalDebugEnabled = false
	advancedDebugLevel = 0
	debugLogFile       *os.File
	debugLogger        *log.Logger
	defaultLogPath     = filepath.Join(os.TempDir(), "macgo-debug.log")
	pprofEnabled       = false
	defaultPprofPort   = 6060
	pprofPortIncrement = 1
	pprofBasePort      = defaultPprofPort
	debugMutex         sync.Mutex
	isInitialized      bool

	// TraceSignalHandling enables detailed tracing of all signal handling operations
	TraceSignalHandling = false
)

// initialize sets up the debug package
func initialize() {
	debugMutex.Lock()
	defer debugMutex.Unlock()

	if isInitialized {
		return
	}

	// Check environment variables
	signalDebugEnabled = os.Getenv("MACGO_SIGNAL_DEBUG") == "1"

	// Parse advanced debug level
	if level, err := strconv.Atoi(os.Getenv("MACGO_DEBUG_LEVEL")); err == nil {
		advancedDebugLevel = level
	}

	// Set up signal debugging if enabled
	if signalDebugEnabled {
		TraceSignalHandling = true

		// Get custom log path if provided
		logPath := os.Getenv("MACGO_DEBUG_LOG")
		if logPath == "" {
			logPath = defaultLogPath
		}

		// Set up debug log file
		var err error
		debugLogFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to open debug log file: %v\n", err)
			debugLogger = log.New(os.Stderr, "[macgo-debug] ", log.LstdFlags|log.Lmicroseconds)
		} else {
			// Use multi-writer to write to both stderr and file
			multiWriter := io.MultiWriter(os.Stderr, debugLogFile)
			debugLogger = log.New(multiWriter, "[macgo-debug] ", log.LstdFlags|log.Lmicroseconds)
		}

		logSystemInfo()
	}

	// Check if pprof is enabled
	pprofEnabled = os.Getenv("MACGO_PPROF") == "1"
	if pprofEnabled {
		// Get custom base port if provided
		if port, err := strconv.Atoi(os.Getenv("MACGO_PPROF_PORT")); err == nil {
			pprofBasePort = port
		}

		// Start pprof server on this process
		go startPprofServer(pprofBasePort)
	}

	isInitialized = true
}

// Init ensures the debug package is initialized
func Init() {
	if !isInitialized {
		initialize()
	}
}

// logSystemInfo logs basic system information for debugging
func logSystemInfo() {
	if debugLogger == nil {
		return
	}

	debugLogger.Printf("==== macgo debug logging initialized ====")
	debugLogger.Printf("PID: %d, PPID: %d", os.Getpid(), os.Getppid())
	debugLogger.Printf("Args: %v", os.Args)
	debugLogger.Printf("Signal debugging: enabled")
	debugLogger.Printf("Debug level: %d", advancedDebugLevel)
	debugLogger.Printf("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)

	// Get current working directory
	if cwd, err := os.Getwd(); err == nil {
		debugLogger.Printf("Working directory: %s", cwd)
	}

	// Log some environment variables
	debugLogger.Printf("MACGO_DEBUG=%s", os.Getenv("MACGO_DEBUG"))
	debugLogger.Printf("MACGO_DEBUG_LEVEL=%s", os.Getenv("MACGO_DEBUG_LEVEL"))
	debugLogger.Printf("MACGO_SIGNAL_DEBUG=%s", os.Getenv("MACGO_SIGNAL_DEBUG"))
	debugLogger.Printf("MACGO_PPROF=%s", os.Getenv("MACGO_PPROF"))
	debugLogger.Printf("MACGO_PPROF_PORT=%s", os.Getenv("MACGO_PPROF_PORT"))

	// Log time
	debugLogger.Printf("Time: %s", time.Now().Format(time.RFC3339))
	debugLogger.Printf("=======================================")
}

// LogSignal logs detailed information about a signal
func LogSignal(sig syscall.Signal, format string, args ...interface{}) {
	if !signalDebugEnabled || debugLogger == nil {
		return
	}

	// Format the message
	message := fmt.Sprintf(format, args...)

	// Create stack trace if debug level is high enough
	var stack string
	if advancedDebugLevel >= 2 {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stack = string(buf[:n])
	}

	// Log the message
	debugLogger.Printf("SIGNAL %v: %s", sig, message)
	if advancedDebugLevel >= 2 {
		debugLogger.Printf("Stack trace:\n%s", stack)
	}
}

// LogDebug logs a debug message if signal debugging is enabled
func LogDebug(format string, args ...interface{}) {
	if !signalDebugEnabled || debugLogger == nil {
		return
	}

	debugLogger.Printf(format, args...)
}

// startPprofServer starts a pprof HTTP server on the specified port
func startPprofServer(port int) {
	// Increment port for each new server
	actualPort := port

	// Start the server in a separate goroutine
	go func() {
		// Try multiple ports if the default one is in use
		for i := 0; i < 10; i++ {
			addr := fmt.Sprintf("localhost:%d", actualPort)
			server := &http.Server{
				Addr:    addr,
				Handler: http.DefaultServeMux,
			}

			// Try to start the server
			if signalDebugEnabled && debugLogger != nil {
				debugLogger.Printf("Starting pprof server on %s", addr)
			} else {
				fmt.Fprintf(os.Stderr, "[macgo-pprof] Starting server on %s\n", addr)
			}

			// Set up basic pprof info handler
			http.HandleFunc("/macgo/info", func(w http.ResponseWriter, r *http.Request) {
				info := fmt.Sprintf(`
Process Information:
  PID: %d
  PPID: %d
  Command: %s
  Args: %v
  Working Directory: %s
  Timestamp: %s
  
Environment:
  MACGO_DEBUG=%s
  MACGO_DEBUG_LEVEL=%s
  MACGO_SIGNAL_DEBUG=%s
  MACGO_PPROF=%s
  MACGO_PPROF_PORT=%s
`,
					os.Getpid(),
					os.Getppid(),
					os.Args[0],
					os.Args,
					getWorkingDir(),
					time.Now().Format(time.RFC3339),
					os.Getenv("MACGO_DEBUG"),
					os.Getenv("MACGO_DEBUG_LEVEL"),
					os.Getenv("MACGO_SIGNAL_DEBUG"),
					os.Getenv("MACGO_PPROF"),
					os.Getenv("MACGO_PPROF_PORT"),
				)

				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(info))
			})

			// Print usage information to make it easier for users
			if signalDebugEnabled && debugLogger != nil {
				debugLogger.Printf("pprof endpoints available at:")
				debugLogger.Printf("  http://%s/debug/pprof/", addr)
				debugLogger.Printf("  Process info: http://%s/macgo/info", addr)
				debugLogger.Printf("  Heap profile: http://%s/debug/pprof/heap", addr)
				debugLogger.Printf("  CPU profile: http://%s/debug/pprof/profile", addr)
				debugLogger.Printf("  Goroutine trace: http://%s/debug/pprof/trace", addr)
				debugLogger.Printf("  Goroutine block profile: http://%s/debug/pprof/block", addr)
				debugLogger.Printf("  Goroutine stack traces: http://%s/debug/pprof/goroutine", addr)
			} else {
				fmt.Fprintf(os.Stderr, "[macgo-pprof] Server available at http://%s/debug/pprof/\n", addr)
				fmt.Fprintf(os.Stderr, "[macgo-pprof] Process info: http://%s/macgo/info\n", addr)
			}

			err := server.ListenAndServe()
			if err != nil {
				if signalDebugEnabled && debugLogger != nil {
					debugLogger.Printf("Failed to start pprof server on port %d: %v", actualPort, err)
				} else {
					fmt.Fprintf(os.Stderr, "[macgo-pprof] Failed to start server on port %d: %v\n", actualPort, err)
				}
				// Try next port
				actualPort += pprofPortIncrement
				continue
			}

			break
		}
	}()
}

// getWorkingDir returns the current working directory or an error message
func getWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Error getting working directory: %v", err)
	}
	return dir
}

// GetNextPprofPort returns the next available pprof port for child processes
func GetNextPprofPort() int {
	debugMutex.Lock()
	defer debugMutex.Unlock()

	pprofBasePort += pprofPortIncrement
	return pprofBasePort
}

// IsPprofEnabled returns true if pprof debugging is enabled
func IsPprofEnabled() bool {
	return pprofEnabled
}

// IsTraceEnabled returns true if trace debugging is enabled
func IsTraceEnabled() bool {
	return TraceSignalHandling
}

// Close cleans up debug resources
func Close() {
	debugMutex.Lock()
	defer debugMutex.Unlock()

	if debugLogFile != nil {
		debugLogFile.Close()
		debugLogFile = nil
	}

	debugLogger = nil
}
