package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/tmc/misc/eslog"
)

const (
	headerText = `eslog - Endpoint Security Event Log Processor
Version: 1.0.0

`
	usageText = `Usage: eslog [options] [file]

eslog processes Endpoint Security JSON event logs from files or stdin,
with filtering, formatting, and visualization capabilities.

Basic Options:
  -file string       Input file (if not specified, stdin is used or positional arg)
  -output string     Output file (if not specified, stdout is used)
  -F                 Follow file in real-time, processing events as they are added
  -tui               Start interactive TUI (Terminal User Interface) mode

Filter Options:
  -pid int           Filter by process ID
  -name string       Filter by process name (supports partial matches)
  -event int         Filter by event type
  -tty string        Filter by TTY path (e.g., 'ttys009')
  -seq-start int     Filter events with sequence number >= this value
  -seq-end int       Filter events with sequence number <= this value
  -ppid-filter int   Only show events within a specific PPID group

Output Options:
  -format string     Output format: default, json, or template (default "default")
  -template string   Go template for output (use with -format template)
  -json string       JSON mode: 'clean' for essential fields, 'raw' for full events
  -root int          Root PID for tree view (0 for all root processes)
  -max-args int      Maximum length for command arguments display (default 120)
  -show-sensitive    Show sensitive environment variables (disabled by default)

Configuration:
  -config string     Path to configuration file (defaults to ~/.eslogrc.json)
  -dump-config       Dump default configuration template and exit
  
Examples:
  eslog events.json                          Process a file and display as tree
  eslog -name bash events.json               Filter for bash processes
  eslog -tui events.json                     Open interactive TUI mode
  eslog -json clean events.json              Output clean JSON format
  cat events.json | eslog -name node         Process logs from stdin
  eslog -format template -template "{{.Process.Executable.Path}}" events.json

For more examples and full documentation, visit: 
https://github.com/tmc/misc/eslog
`
)

func usage() {
	fmt.Fprintf(os.Stderr, headerText)
	fmt.Fprintf(os.Stderr, usageText)
}

func main() {
	// Define command line flags with improved descriptions
	flag.Usage = usage

	// Input/output options
	inputFile := flag.String("file", "", "Input file (if not specified, stdin is used)")
	outputFile := flag.String("output", "", "Output file (if not specified, stdout is used)")
	follow := flag.Bool("F", false, "Follow file in real-time, processing events as they are added")
	tuiMode := flag.Bool("tui", false, "Start interactive TUI (Terminal User Interface) mode")

	// Filter options
	filterPID := flag.Int("pid", 0, "Filter by process ID")
	filterName := flag.String("name", "", "Filter by process name (supports partial matches)")
	filterEvent := flag.Int("event", 0, "Filter by event type")
	filterTTY := flag.String("tty", "", "Filter by TTY path (e.g., 'ttys009')")
	seqStart := flag.Int("seq-start", 0, "Filter events with sequence number greater than or equal to this value")
	seqEnd := flag.Int("seq-end", 0, "Filter events with sequence number less than or equal to this value")
	ppidFilter := flag.Int("ppid-filter", 0, "Only show events within a specific PPID group")

	// Output options
	formatStr := flag.String("format", "default", "Output format: default, json, or template")
	templateStr := flag.String("template", "{{.Process.Executable.Path}} [PID:{{.Process.AuditToken.PID}}]", "Go template for output")
	rootPID := flag.Int("root", 0, "Root PID for tree view (0 for all root processes)")
	showEnv := flag.Bool("show-sensitive", false, "Show sensitive environment variables (disabled by default)")
	maxArgsLen := flag.Int("max-args", 120, "Maximum length for command arguments display")
	jsonMode := flag.String("json", "", "JSON output mode: 'clean' for essential fields only, 'raw' for full events")
	streamMode := flag.Bool("stream", false, "Stream filtered events in full JSON format (useful for piping to other tools)")
	redactSecrets := flag.Bool("redact", true, "Redact sensitive information in environment variables even in stream mode")

	// Configuration options
	configPath := flag.String("config", "", "Path to configuration file (defaults to ~/.eslogrc.json if exists)")
	dumpConfig := flag.Bool("dump-config", false, "Dump default configuration as template and exit")

	// OpenTelemetry options (provided for backward compatibility but redirected to separate tool)
	otelExport := flag.String("otel-export", "", "Export events to OpenTelemetry format (use 'tools/eslog-to-otel' tool)")

	flag.Parse()

	// Handle positional argument as input file
	if *inputFile == "" && flag.NArg() > 0 {
		*inputFile = flag.Arg(0)
	}

	// Print friendly message about OpenTelemetry export
	if *otelExport != "" {
		fmt.Fprintf(os.Stderr, "OpenTelemetry export is available via the 'eslog-to-otel' utility.\n\n")
		fmt.Fprintf(os.Stderr, "Try the following command instead:\n")
		if *inputFile != "" {
			fmt.Fprintf(os.Stderr, "  eslog -json raw %s | eslog-to-otel\n\n", *inputFile)
		} else {
			fmt.Fprintf(os.Stderr, "  cat your_log_file.json | eslog -json raw | eslog-to-otel\n\n")
		}
		fmt.Fprintf(os.Stderr, "See 'eslog-to-otel -h' for OpenTelemetry export options.\n")
		os.Exit(1)
	}

	// If dump-config flag is set, print the default config and exit
	if *dumpConfig {
		config := eslog.DefaultConfig()
		jsonConfig, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonConfig))
		fmt.Fprintf(os.Stderr, "\nSave this to ~/.eslogrc.json or provide with -config flag\n")
		os.Exit(0)
	}

	// Set up input
	var input io.Reader
	if *inputFile != "" {
		file, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	} else {
		// Check if stdin has data
		stdinStat, _ := os.Stdin.Stat()
		if (stdinStat.Mode() & os.ModeCharDevice) != 0 {
			// No input on stdin and no file specified
			fmt.Fprintf(os.Stderr, "No input file specified and no data on stdin.\n\n")
			usage()
			os.Exit(1)
		}
		input = os.Stdin
	}

	// Set up output
	var output io.Writer
	if *outputFile != "" {
		file, err := os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Create filters with friendly messages for each
	var filters []eslog.FilterFunc
	if *filterPID != 0 {
		fmt.Fprintf(os.Stderr, "Filtering for PID: %d\n", *filterPID)
		filters = append(filters, eslog.CreateProcessPIDFilter(*filterPID))
	}
	if *filterName != "" {
		fmt.Fprintf(os.Stderr, "Filtering for process name: %s\n", *filterName)
		filters = append(filters, eslog.CreateProcessNameFilter(*filterName))
	}
	if *filterEvent != 0 {
		fmt.Fprintf(os.Stderr, "Filtering for event type: %d\n", *filterEvent)
		filters = append(filters, eslog.CreateEventTypeFilter(*filterEvent))
	}
	if *filterTTY != "" {
		fmt.Fprintf(os.Stderr, "Filtering for TTY: %s\n", *filterTTY)
		filters = append(filters, eslog.CreateTTYFilter(*filterTTY))
	}
	if *seqStart > 0 || *seqEnd > 0 {
		if *seqStart > 0 && *seqEnd > 0 {
			fmt.Fprintf(os.Stderr, "Filtering for sequence numbers: %d to %d\n", *seqStart, *seqEnd)
		} else if *seqStart > 0 {
			fmt.Fprintf(os.Stderr, "Filtering for sequence numbers: >= %d\n", *seqStart)
		} else {
			fmt.Fprintf(os.Stderr, "Filtering for sequence numbers: <= %d\n", *seqEnd)
		}
		filters = append(filters, eslog.CreateSequenceFilter(*seqStart, *seqEnd))
	}
	if *ppidFilter > 0 {
		fmt.Fprintf(os.Stderr, "Filtering for parent PID: %d\n", *ppidFilter)
		filters = append(filters, eslog.CreatePPIDFilter(*ppidFilter))
	}

	// Create output template with custom functions
	var tmpl *template.Template
	var err error
	if *formatStr == "template" {
		// Create template functions
		funcMap := template.FuncMap{
			"contains": func(s, substr string) bool {
				return strings.Contains(s, substr)
			},
			"hasPrefix": func(s, prefix string) bool {
				return strings.HasPrefix(s, prefix)
			},
			"hasSuffix": func(s, suffix string) bool {
				return strings.HasSuffix(s, suffix)
			},
			"split": func(s, sep string) []string {
				return strings.Split(s, sep)
			},
			"join": func(sep string, s []string) string {
				return strings.Join(s, sep)
			},
			"lower": func(s string) string {
				return strings.ToLower(s)
			},
			"upper": func(s string) string {
				return strings.ToUpper(s)
			},
			"title": func(s string) string {
				return strings.ToTitle(s)
			},
			"trim": func(s string) string {
				return strings.TrimSpace(s)
			},
			// Add more useful template functions
			"basename": filepath.Base,
			"dirname": filepath.Dir,
			"args": func(args []string) string {
				if len(args) == 0 {
					return ""
				}
				return strings.Join(args, " ")
			},
			"cmd": func(args []string) string {
				if len(args) == 0 {
					return ""
				}
				return args[0]
			},
			"formatTime": func(timeStr string) string {
				t, err := time.Parse(time.RFC3339Nano, timeStr)
				if err != nil {
					return timeStr
				}
				return t.Format("15:04:05.000")
			},
		}

		tmpl, err = template.New("output").Funcs(funcMap).Parse(*templateStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing template: %v\n", err)
			os.Exit(1)
		}
	}

	// Process events
	scanner := bufio.NewScanner(input)

	// Buffer for potentially large lines
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 10*1024*1024) // Allow up to 10MB per line

	// Set up file following if enabled
	var fileToFollow *os.File
	var watcher *fsnotify.Watcher
	var followOffset int64

	if *follow && *inputFile != "" {
		// We need to read the file and then follow it
		fileToFollow, err = os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file for following: %v\n", err)
			os.Exit(1)
		}
		defer fileToFollow.Close()

		// Start watching for changes
		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file watcher: %v\n", err)
			os.Exit(1)
		}
		defer watcher.Close()

		if err := watcher.Add(*inputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error watching file: %v\n", err)
			os.Exit(1)
		}

		// We'll use input for initial processing and then follow fileToFollow
		input = fileToFollow
		scanner = bufio.NewScanner(input)
		scanner.Buffer(buf, 10*1024*1024)
	}

	// Create a process tree with default config
	processTree := eslog.NewProcessTree()

	// Load user configuration if specified
	if *configPath != "" {
		userConfig, err := eslog.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration from %s: %v\n", *configPath, err)
		} else {
			processTree.Config = userConfig
			fmt.Fprintf(os.Stderr, "Loaded configuration from %s\n", *configPath)
		}
	} else {
		// If configPath is not provided, check for ~/.eslogrc.json
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultConfigPath := filepath.Join(homeDir, ".eslogrc.json")
			if _, err := os.Stat(defaultConfigPath); err == nil {
				configFromHome, err := eslog.LoadConfig(defaultConfigPath)
				if err == nil {
					processTree.Config = configFromHome
					fmt.Fprintf(os.Stderr, "Loaded configuration from %s\n", defaultConfigPath)
				}
			}
		}
	}

	// Create a customized AddEvent function based on showEnv flag
	addEventFunc := func(pt *eslog.ProcessTree, event *eslog.ESEvent) {
		// Skip if not an exec event
		if event.Event.Exec == nil {
			return
		}

		// Filter sensitive env vars if requested
		if !*showEnv && event.Event.Exec.Env != nil {
			event.Event.Exec.Env = eslog.FilterSensitiveEnvVars(event.Event.Exec.Env)
		}

		// Parse and track process start time for relative timing
		startTime, err := time.Parse(time.RFC3339Nano, event.Process.StartTime)
		if err == nil {
			// If this is the first process or has an earlier start time, update FirstStartTime
			if pt.FirstStartTime.IsZero() || startTime.Before(pt.FirstStartTime) {
				pt.FirstStartTime = startTime
			}
		}

		pid := event.Process.AuditToken.PID
		ppid := event.Process.PPID

		// Create or get the process node
		node, exists := pt.Nodes[pid]
		if !exists {
			node = &eslog.ProcessNode{
				Process:  &event.Process,
				Children: make([]*eslog.ProcessNode, 0),
				Execs:    make([]*eslog.ESEvent, 0),
			}
			pt.Nodes[pid] = node
		}

		// Add exec event to the node
		node.Execs = append(node.Execs, event)

		// Set up parent relationship
		parentNode, parentExists := pt.Nodes[ppid]
		if !parentExists {
			// Create a placeholder parent node
			parentNode = &eslog.ProcessNode{
				Process:    nil, // Will be filled when we see the parent process event
				Children:   make([]*eslog.ProcessNode, 0),
				Execs:      make([]*eslog.ESEvent, 0),
				FileOps:    eslog.FileOpsCounter{},
				HasExited:  false,
				ExitCode:   -1,
			}
			pt.Nodes[ppid] = parentNode
		}

		// Set up parent-child relationship if not already established
		if node.Parent == nil {
			node.Parent = parentNode
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	// Show progress for large files
	eventCount := 0
	lastReportTime := time.Now()
	showProgress := *inputFile != "" && !*tuiMode && !*streamMode && *jsonMode == "" 

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event eslog.ESEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON line %d: %v\n", eventCount+1, err)
			continue
		}

		eventCount++
		
		// Show periodic progress for large files
		if showProgress && eventCount%1000 == 0 && time.Since(lastReportTime) > 2*time.Second {
			fmt.Fprintf(os.Stderr, "Processed %d events...\r", eventCount)
			lastReportTime = time.Now()
		}

		// Apply filters
		passedAllFilters := true
		for _, filter := range filters {
			if !filter(&event) {
				passedAllFilters = false
				break
			}
		}

		if !passedAllFilters {
			continue
		}

		// Redact secrets if enabled for stream mode
		if *redactSecrets && *streamMode && event.Event.Exec != nil && event.Event.Exec.Env != nil {
			event.Event.Exec.Env = eslog.FilterSensitiveEnvVars(event.Event.Exec.Env)
		}

		// Handle stream mode (outputs filtered but full JSON events)
		if *streamMode {
			jsonOutput, err := json.Marshal(event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				continue
			}
			fmt.Fprintln(output, string(jsonOutput))
			continue // Skip further processing
		}

		// Add to process tree (for non-stream mode)
		addEventFunc(processTree, &event)

		// Handle JSON output modes if specified
		if *jsonMode != "" {
			switch *jsonMode {
			case "raw":
				// Output full raw event as JSON
				jsonOutput, err := json.MarshalIndent(event, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
					continue
				}
				fmt.Fprintln(output, string(jsonOutput))
			case "clean":
				// Create and output a clean event with only essential fields
				cleanEvent := eslog.CleanEvent{
					SeqNum:         event.SeqNum,
					Time:           event.Time,
					PID:            event.Process.AuditToken.PID,
					PPID:           event.Process.PPID,
					OriginalPPID:   event.Process.OriginalPPID,
					ExecutablePath: event.Process.Executable.Path,
				}

				// Calculate relative time
				startTime, timeErr := time.Parse(time.RFC3339Nano, event.Process.StartTime)
				if timeErr == nil && !processTree.FirstStartTime.IsZero() {
					cleanEvent.RelativeTime = startTime.Sub(processTree.FirstStartTime).Seconds()
				}

				// Add TTY if available
				if event.Process.TTY != nil && event.Process.TTY.Path != "" {
					cleanEvent.TTY = event.Process.TTY.Path
				}

				// Add command and args if available
				if event.Event.Exec != nil && len(event.Event.Exec.Args) > 0 {
					if len(event.Event.Exec.Args) > 0 {
						cleanEvent.Command = event.Event.Exec.Args[0]
					}
					if len(event.Event.Exec.Args) > 1 {
						cleanEvent.Args = event.Event.Exec.Args[1:]
					}
					if event.Event.Exec.CWD.Path != "" {
						cleanEvent.CWD = event.Event.Exec.CWD.Path
					}
				}

				jsonOutput, err := json.MarshalIndent(cleanEvent, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
					continue
				}
				fmt.Fprintln(output, string(jsonOutput))
			}
			continue // Skip normal output processing
		}

		// Print if not in tree mode
		if *rootPID == 0 && *formatStr != "default" {
			switch *formatStr {
			case "json":
				jsonOutput, err := json.MarshalIndent(event, "", "  ")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
					continue
				}
				fmt.Fprintln(output, string(jsonOutput))
			case "template":
				if tmpl != nil {
					if err := tmpl.Execute(output, event); err != nil {
						fmt.Fprintf(os.Stderr, "Template error: %v\n", err)
					}
					fmt.Fprintln(output)
				}
			}
		}
	}

	// Clear progress line and show final count
	if showProgress && eventCount > 1000 {
		fmt.Fprintf(os.Stderr, "                                        \r")
		fmt.Fprintf(os.Stderr, "Processed %d events\n", eventCount)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Implement file following if requested
	if *follow && fileToFollow != nil {
		// Get current file position for following
		followOffset, err = fileToFollow.Seek(0, io.SeekCurrent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting file position: %v\n", err)
			os.Exit(1)
		}

		// Display initial result first
		eslog.DisplayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)

		fmt.Fprintln(os.Stderr, "Following file... (Press Ctrl+C to exit)")

		// Enter the file following loop
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) {
					// File has been modified, read new content
					stat, err := fileToFollow.Stat()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error getting file stats: %v\n", err)
						continue
					}

					if stat.Size() < followOffset {
						// File was truncated
						followOffset = 0
						if _, err := fileToFollow.Seek(0, io.SeekStart); err != nil {
							fmt.Fprintf(os.Stderr, "Error seeking in file: %v\n", err)
							continue
						}
					} else if stat.Size() > followOffset {
						// New content added
						if _, err := fileToFollow.Seek(followOffset, io.SeekStart); err != nil {
							fmt.Fprintf(os.Stderr, "Error seeking in file: %v\n", err)
							continue
						}

						// Create a new scanner for the additional content
						newScanner := bufio.NewScanner(fileToFollow)
						newScanner.Buffer(buf, 10*1024*1024)

						// Process new lines
						linesAdded := false
						for newScanner.Scan() {
							line := newScanner.Text()
							if strings.TrimSpace(line) == "" {
								continue
							}

							var event eslog.ESEvent
							if err := json.Unmarshal([]byte(line), &event); err != nil {
								fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
								continue
							}

							// Apply filters
							passedAllFilters := true
							for _, filter := range filters {
								if !filter(&event) {
									passedAllFilters = false
									break
								}
							}

							if !passedAllFilters {
								continue
							}

							// Handle stream mode
							if *streamMode {
								// Redact secrets if requested
								if *redactSecrets && event.Event.Exec != nil && event.Event.Exec.Env != nil {
									event.Event.Exec.Env = eslog.FilterSensitiveEnvVars(event.Event.Exec.Env)
								}
								
								jsonOutput, err := json.Marshal(event)
								if err != nil {
									fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
									continue
								}
								fmt.Fprintln(output, string(jsonOutput))
								continue // Skip further processing
							}

							// Add to process tree
							addEventFunc(processTree, &event)
							linesAdded = true

							// Handle JSON output modes for immediate display
							if *jsonMode != "" {
								eslog.DisplayJSONEvent(output, jsonMode, &event, processTree)
							}
						}

						if err := newScanner.Err(); err != nil {
							fmt.Fprintf(os.Stderr, "Error reading new content: %v\n", err)
							continue
						}

						// Update the offset for next read
						newOffset, err := fileToFollow.Seek(0, io.SeekCurrent)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error getting new file position: %v\n", err)
							continue
						}
						followOffset = newOffset

						// Display updated results if needed
						if linesAdded && *jsonMode == "" && !*streamMode {
							fmt.Fprintln(output, "\n--- Updated Process Tree ---")
							eslog.DisplayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Error watching file: %v\n", err)
			}
		}
	}

	// Skip tree display in stream mode
	if *streamMode {
		return
	}

	// Check if TUI mode is enabled
	if *tuiMode {
		// Launch TUI mode with the processed data
		if err := eslog.StartTUI(processTree); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Regular display mode
		// Display results if not in following mode or if not following a file
		if !*follow || *inputFile == "" {
			eslog.DisplayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)
		}
	}
}