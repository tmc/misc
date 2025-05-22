package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ESEvent represents the base structure of an ES event
type ESEvent struct {
	Action        ActionData  `json:"action"`
	ActionType    int         `json:"action_type"`
	Event         EventData   `json:"event"`
	EventType     int         `json:"event_type"`
	GlobalSeqNum  int         `json:"global_seq_num"`
	MachTime      int64       `json:"mach_time"`
	Process       ProcessData `json:"process"`
	SchemaVersion int         `json:"schema_version"`
	SeqNum        int         `json:"seq_num"`
	Thread        ThreadData  `json:"thread"`
	Time          string      `json:"time"`
	Version       int         `json:"version"`
}

// ProcessData contains information about the process
type ProcessData struct {
	AuditToken            AuditToken `json:"audit_token"`
	CdHash                string     `json:"cdhash"`
	CodesigningFlags      int64      `json:"codesigning_flags"`
	Executable            FileInfo   `json:"executable"`
	GroupID               int        `json:"group_id"`
	IsPlatformBinary      bool       `json:"is_platform_binary"`
	IsESClient            bool       `json:"is_es_client"`
	OriginalPPID          int        `json:"original_ppid"`
	ParentAuditToken      AuditToken `json:"parent_audit_token"`
	PPID                  int        `json:"ppid"`
	ResponsibleAuditToken AuditToken `json:"responsible_audit_token"`
	SessionID             int        `json:"session_id"`
	SigningID             string     `json:"signing_id"`
	StartTime             string     `json:"start_time"`
	TeamID                *string    `json:"team_id"`
	TTY                   *FileInfo  `json:"tty"`
}

// AuditToken contains process identification information
type AuditToken struct {
	ASID       int `json:"asid"`
	AUID       int `json:"auid"`
	EUID       int `json:"euid"`
	EGID       int `json:"egid"`
	PID        int `json:"pid"`
	PIDVersion int `json:"pidversion"`
	RGID       int `json:"rgid"`
	RUID       int `json:"ruid"`
}

// FileInfo contains information about a file
type FileInfo struct {
	Path          string   `json:"path"`
	PathTruncated bool     `json:"path_truncated"`
	Stat          StatInfo `json:"stat,omitempty"`
}

// StatInfo contains file stat information
type StatInfo struct {
	StDev           int64  `json:"st_dev"`
	StIno           int64  `json:"st_ino"`
	StMode          int    `json:"st_mode"`
	StNlink         int    `json:"st_nlink"`
	StUID           int    `json:"st_uid"`
	StGID           int    `json:"st_gid"`
	StRdev          int64  `json:"st_rdev"`
	StSize          int64  `json:"st_size"`
	StBlocks        int64  `json:"st_blocks"`
	StBlksize       int    `json:"st_blksize"`
	StFlags         int    `json:"st_flags"`
	StGen           int    `json:"st_gen"`
	StAtimespec     string `json:"st_atimespec"`
	StMtimespec     string `json:"st_mtimespec"`
	StCtimespec     string `json:"st_ctimespec"`
	StBirthtimespec string `json:"st_birthtimespec"`
}

// FileDescriptor represents a file descriptor
type FileDescriptor struct {
	FD     int `json:"fd"`
	FDType int `json:"fdtype"`
}

// EventData contains the event details
type EventData struct {
	Exec        *ExecEvent       `json:"exec,omitempty"`
	Lookup      *LookupEvent     `json:"lookup,omitempty"`
	Readlink    *ReadlinkEvent   `json:"readlink,omitempty"`
	Stat        *StatEvent       `json:"stat,omitempty"`
	Access      *AccessEvent     `json:"access,omitempty"`
	Open        *OpenEvent       `json:"open,omitempty"`
	Close       *CloseEvent      `json:"close,omitempty"`
	Exit        *ExitEvent       `json:"exit,omitempty"`
	Read        *ReadEvent       `json:"read,omitempty"`
	Write       *WriteEvent      `json:"write,omitempty"`
}

// ExecEvent contains execution information
type ExecEvent struct {
	Args            []string         `json:"args"`
	CWD             FileInfo         `json:"cwd"`
	DyldExecPath    string           `json:"dyld_exec_path"`
	Env             []string         `json:"env"`
	FDs             []FileDescriptor `json:"fds"`
	ImageCPUType    int              `json:"image_cputype"`
	ImageCPUSubType int              `json:"image_cpusubtype"`
	LastFD          int              `json:"last_fd"`
	Script          interface{}      `json:"script"` // Can be string or object
	Target          ProcessData      `json:"target"`
}

// ActionData contains action information
type ActionData struct {
	Result ActionResult `json:"result"`
}

// ActionResult contains the result of an action
type ActionResult struct {
	ResultType int         `json:"result_type"`
	Result     interface{} `json:"result"`
}

// ThreadData contains thread information
type ThreadData struct {
	ThreadID int `json:"thread_id"`
}

// CommandExtractor defines a pattern to extract actual command from shell scripts
type CommandExtractor struct {
	Pattern     string `json:"pattern"`      // Regex pattern to match
	Group       int    `json:"group"`        // Capture group to extract
	DisplayName string `json:"display_name"` // Display prefix for the command
}

// Config holds application configuration
type Config struct {
	CommandExtractors []CommandExtractor `json:"command_extractors"`
}

// DefaultConfig provides the default configuration
func DefaultConfig() Config {
	return Config{
		CommandExtractors: []CommandExtractor{
			{
				Pattern:     `source\s+.*\s+&&\s+eval\s+'([^']+)'`,
				Group:       1,
				DisplayName: "EVAL:",
			},
			{
				Pattern:     `which\s+(\S+)`,
				Group:       1,
				DisplayName: "WHICH:",
			},
			{
				Pattern:     `go\s+test\s+(.+)`,
				Group:       1,
				DisplayName: "GO TEST:",
			},
			{
				Pattern:     `go\s+run\s+(.+)`,
				Group:       1,
				DisplayName: "GO RUN:",
			},
			{
				Pattern:     `npm\s+(.+)`,
				Group:       1,
				DisplayName: "NPM:",
			},
		},
	}
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()

	if path == "" {
		return config, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

// CleanEvent is a simplified version of ESEvent with only essential fields
type CleanEvent struct {
	SeqNum         int       `json:"seq_num"`
	Time           string    `json:"time"`
	RelativeTime   float64   `json:"relative_time"`
	PID            int       `json:"pid"`
	PPID           int       `json:"ppid"`
	OriginalPPID   int       `json:"original_ppid"`
	TTY            string    `json:"tty,omitempty"`
	Command        string    `json:"command"`
	Args           []string  `json:"args,omitempty"`
	CWD            string    `json:"cwd,omitempty"`
	ExecutablePath string    `json:"executable_path"`
}

// File operation event types
type LookupEvent struct {
	SourceDir FileInfo `json:"source_dir"`
	RelativePath string `json:"relative_path"`
}

type ReadlinkEvent struct {
	Source FileInfo `json:"source"`
}

type StatEvent struct {
	Source FileInfo `json:"source"`
}

type AccessEvent struct {
	Source FileInfo `json:"source"`
	Mode int `json:"mode"`
}

type OpenEvent struct {
	File FileInfo `json:"file"`
	Mode int `json:"mode"`
}

type CloseEvent struct {
	Target FileDescriptor `json:"target"`
}

// ReadEvent contains read operation information
type ReadEvent struct {
	FD     FileDescriptor `json:"fd"`
	Size   int64          `json:"size,omitempty"`
	Offset int64          `json:"offset,omitempty"`
}

// WriteEvent contains write operation information
type WriteEvent struct {
	FD     FileDescriptor `json:"fd"`
	Size   int64          `json:"size,omitempty"`
	Offset int64          `json:"offset,omitempty"`
}

// ExitEvent contains process exit information
type ExitEvent struct {
	ExitCode int    `json:"exit_code"`
	Reason   string `json:"reason"`
}

// FileOpsCounter tracks file operation counts and amounts for a process
type FileOpsCounter struct {
	Lookups      int
	Readlinks    int
	Stats        int
	Accesses     int
	Opens        int
	Closes       int
	Reads        int
	Writes       int
	BytesRead    int64
	BytesWritten int64
}

// ProcessNode represents a node in the process tree
type ProcessNode struct {
	Process    *ProcessData
	Children   []*ProcessNode
	Execs      []*ESEvent
	Parent     *ProcessNode
	FileOps    FileOpsCounter // Counter for file operations
	ExitEvent  *ESEvent       // Event containing process exit details
	ExitTime   time.Time      // Time when the process exited
	ExitCode   int            // Exit code of the process
	HasExited  bool           // Flag indicating if the process has exited
}

// ProcessTree manages process hierarchy
type ProcessTree struct {
	Nodes map[int]*ProcessNode // PID -> Node
	FirstStartTime time.Time   // First process start time for relative timing
	Config Config              // Command extraction configuration
}

// NewProcessTree creates a new process tree
func NewProcessTree() *ProcessTree {
	return &ProcessTree{
		Nodes: make(map[int]*ProcessNode),
		FirstStartTime: time.Time{}, // Will be set when we encounter the first process
		Config: DefaultConfig(),     // Initialize with default config
	}
}

// filterSensitiveEnvVars filters sensitive environment variables
func filterSensitiveEnvVars(env []string) []string {
	if env == nil {
		return nil
	}

	sensitivePattern := regexp.MustCompile(`(?i)(TOKEN|SECRET|KEY|PASSWORD|CREDENTIAL|AUTH)=.+`)

	filtered := make([]string, len(env))
	for i, e := range env {
		if sensitivePattern.MatchString(e) {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				filtered[i] = parts[0] + "=[REDACTED]"
			} else {
				filtered[i] = e
			}
		} else {
			filtered[i] = e
		}
	}
	return filtered
}

// AddEvent adds an event to the process tree
func (pt *ProcessTree) AddEvent(event *ESEvent) {
	pid := event.Process.AuditToken.PID
	ppid := event.Process.PPID

	// Create or get the process node
	node, exists := pt.Nodes[pid]
	if !exists {
		node = &ProcessNode{
			Process:    &event.Process,
			Children:   make([]*ProcessNode, 0),
			Execs:      make([]*ESEvent, 0),
			FileOps:    FileOpsCounter{}, // Initialize with zero values
			HasExited:  false,
			ExitCode:   -1, // -1 indicates not yet exited
		}
		pt.Nodes[pid] = node
	}

	// Process based on event type
	if event.Event.Exec != nil {
		// Filter sensitive env vars
		if event.Event.Exec.Env != nil {
			event.Event.Exec.Env = filterSensitiveEnvVars(event.Event.Exec.Env)
		}

		// Add exec event to the node
		node.Execs = append(node.Execs, event)
	} else if event.Event.Exit != nil {
		// Process exit event
		node.HasExited = true
		node.ExitEvent = event
		node.ExitCode = event.Event.Exit.ExitCode

		// Parse the exit time
		if exitTime, err := time.Parse(time.RFC3339Nano, event.Time); err == nil {
			node.ExitTime = exitTime
		}
	} else {
		// Count file operations
		if event.Event.Lookup != nil {
			node.FileOps.Lookups++
		} else if event.Event.Readlink != nil {
			node.FileOps.Readlinks++
		} else if event.Event.Stat != nil {
			node.FileOps.Stats++
		} else if event.Event.Access != nil {
			node.FileOps.Accesses++
		} else if event.Event.Open != nil {
			node.FileOps.Opens++
		} else if event.Event.Close != nil {
			node.FileOps.Closes++
		} else if event.Event.Read != nil {
			node.FileOps.Reads++
			if event.Event.Read.Size > 0 {
				node.FileOps.BytesRead += event.Event.Read.Size
			}
		} else if event.Event.Write != nil {
			node.FileOps.Writes++
			if event.Event.Write.Size > 0 {
				node.FileOps.BytesWritten += event.Event.Write.Size
			}
		} else {
			// Not a tracked operation, skip further processing
			return
		}
	}

	// Set up parent relationship
	parentNode, parentExists := pt.Nodes[ppid]
	if !parentExists {
		// Create a placeholder parent node
		parentNode = &ProcessNode{
			Process:  nil, // Will be filled when we see the parent process event
			Children: make([]*ProcessNode, 0),
			Execs:    make([]*ESEvent, 0),
		}
		pt.Nodes[ppid] = parentNode
	}

	// Set up parent-child relationship if not already established
	if node.Parent == nil {
		node.Parent = parentNode
		parentNode.Children = append(parentNode.Children, node)
	}
}

// PrintProcessHistory prints the full history of a process and all its children
func (pt *ProcessTree) PrintProcessHistory(w io.Writer, pid int, tmpl *template.Template, maxArgsLen *int) {
	// Print the process itself and its ancestors
	pt.printProcessAndAncestors(w, pid, tmpl, maxArgsLen)

	// Print all descendants with full history
	pt.printAllDescendantsHistory(w, pid, 1, tmpl, maxArgsLen)
}

// printProcessAndAncestors prints the process and all its ancestors
func (pt *ProcessTree) printProcessAndAncestors(w io.Writer, pid int, tmpl *template.Template, maxArgsLen *int) {
	node, exists := pt.Nodes[pid]
	if !exists {
		return
	}

	// First print ancestors
	if node.Parent != nil && node.Parent.Process != nil {
		pt.printProcessAndAncestors(w, node.Parent.Process.AuditToken.PID, tmpl, maxArgsLen)
	}

	// Then print this process - check if Process is not nil to avoid panic
	if node.Process != nil {
		fmt.Fprintf(w, "Process %d (%s):\n", pid, node.Process.Executable.Path)
		pt.printNodeHistory(w, node, 0, tmpl, maxArgsLen)
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "Process %d (unknown - parent only):\n", pid)
		fmt.Fprintln(w)
	}
}

// printAllDescendantsHistory prints the complete history of all descendants
func (pt *ProcessTree) printAllDescendantsHistory(w io.Writer, pid int, depth int, tmpl *template.Template, maxArgsLen *int) {
	node, exists := pt.Nodes[pid]
	if !exists {
		return
	}

	// Print all children
	for _, child := range node.Children {
		if child.Process != nil {
			childPid := child.Process.AuditToken.PID
			indent := strings.Repeat("  ", depth)
			fmt.Fprintf(w, "%sChild Process %d (%s):\n", indent, childPid, child.Process.Executable.Path)
			pt.printNodeHistory(w, child, depth, tmpl, maxArgsLen)
			fmt.Fprintln(w)

			// Recursively print its descendants
			pt.printAllDescendantsHistory(w, childPid, depth+1, tmpl, maxArgsLen)
		}
	}
}

// printNodeHistory prints the complete execution history of a single node
func (pt *ProcessTree) printNodeHistory(w io.Writer, node *ProcessNode, depth int, tmpl *template.Template, maxArgsLen *int) {
	if node == nil || node.Process == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	for _, exec := range node.Execs {
		if tmpl != nil {
			err := tmpl.Execute(w, exec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Template error: %v\n", err)
			}
			fmt.Fprintln(w)
		} else {
			// Calculate relative time in seconds from start
			relativeTime := ""
			if !pt.FirstStartTime.IsZero() {
				processTime, err := time.Parse(time.RFC3339Nano, exec.Time)
				if err == nil {
					seconds := processTime.Sub(pt.FirstStartTime).Seconds()
					relativeTime = fmt.Sprintf(",T:%.1fs", seconds)
				}
			}

			programName := ""
			argsStr := ""

			if exec.Event.Exec != nil && len(exec.Event.Exec.Args) > 0 {
				programName = exec.Event.Exec.Args[0]

				// Add arguments with user-defined length limit
				if len(exec.Event.Exec.Args) > 1 {
					args := exec.Event.Exec.Args[1:]
					argsStr = " " + strings.Join(args, " ")

					// Check if this is a bash -c or similar wrapper
					if programName == "/bin/bash" || programName == "/bin/sh" {
						if len(args) >= 2 && (args[0] == "-c" || (len(args) >= 3 && args[1] == "-c")) {
							// Find the actual command being executed
							var cmdIndex int
							if args[0] == "-c" {
								cmdIndex = 1
							} else {
								// Skip options like -l before -c
								for i := 1; i < len(args); i++ {
									if args[i] == "-c" && i+1 < len(args) {
										cmdIndex = i + 1
										break
									}
								}
							}

							if cmdIndex > 0 && cmdIndex < len(args) {
								// Extract the actual command from the bash -c argument
								cmd := args[cmdIndex]

								// Process command using configured extractors
								extracted := false
								for _, extractor := range pt.Config.CommandExtractors {
									pattern := regexp.MustCompile(extractor.Pattern)
									if pattern.MatchString(cmd) {
										matches := pattern.FindStringSubmatch(cmd)
										if len(matches) > extractor.Group {
											// Replace the displayed command
											programName = extractor.DisplayName
											argsStr = " " + matches[extractor.Group]
											extracted = true
											break
										}
									}
								}

								// If no pattern matched, fall back to showing the whole command
								if !extracted {
									programName = "SHELL:"
									argsStr = " " + cmd
								}
							}
						}
					}

					// Apply length limit after extracting the real command
					if *maxArgsLen > 0 && len(argsStr) > *maxArgsLen {
						argsStr = argsStr[:*maxArgsLen-3] + "..."
					}
				}

				// Get TTY path if available
				ttyInfo := ""
				if node.Process.TTY != nil && node.Process.TTY.Path != "" {
					ttyName := node.Process.TTY.Path
					// Extract just the terminal name from the path
					if parts := strings.Split(ttyName, "/"); len(parts) > 0 {
						ttyName = parts[len(parts)-1]
					}
					ttyInfo = fmt.Sprintf(",TTY:%s", ttyName)
				}

				fmt.Fprintf(w, "%s[SEQ:%d%s,OPPID:%d,PPID:%d,PID:%d%s] %s%s\n",
					indent,
					exec.SeqNum,
					relativeTime,
					node.Process.OriginalPPID,
					node.Process.PPID,
					node.Process.AuditToken.PID,
					ttyInfo,
					programName,
					argsStr)
			}
		}
	}
}

// displayResults displays the results based on the format and root PID
func displayResults(w io.Writer, formatStr *string, rootPID *int, processTree *ProcessTree, tmpl *template.Template, maxArgsLen *int) {
	if *formatStr == "default" {
		if *rootPID != 0 {
			if _, exists := processTree.Nodes[*rootPID]; exists {
				// Show history for the specified PID
				processTree.PrintProcessHistory(w, *rootPID, tmpl, maxArgsLen)
			} else {
				// If specified root doesn't exist, print a message and then fallback to all roots
				fmt.Fprintf(w, "Warning: PID %d not found in the event log. Showing all root processes.\n\n", *rootPID)
				// Find root processes and print trees
				for pid, node := range processTree.Nodes {
					if node.Parent == nil || node.Parent.Process == nil {
						processTree.PrintTree(w, pid, 0, tmpl, maxArgsLen)
					}
				}
			}
		} else {
			// No specific PID requested, show all root processes
			for pid, node := range processTree.Nodes {
				if node.Parent == nil || node.Parent.Process == nil {
					processTree.PrintTree(w, pid, 0, tmpl, maxArgsLen)
				}
			}
		}
	}
}

// displayJSONEvent displays an event in JSON format
func displayJSONEvent(w io.Writer, jsonMode *string, event *ESEvent, processTree *ProcessTree) {
	switch *jsonMode {
	case "raw":
		// Output full raw event as JSON
		jsonOutput, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return
		}
		fmt.Fprintln(w, string(jsonOutput))
	case "clean":
		// Create and output a clean event with only essential fields
		cleanEvent := CleanEvent{
			SeqNum:       event.SeqNum,
			Time:         event.Time,
			PID:          event.Process.AuditToken.PID,
			PPID:         event.Process.PPID,
			OriginalPPID: event.Process.OriginalPPID,
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
			return
		}
		fmt.Fprintln(w, string(jsonOutput))
	}
}

// PrintTree prints the process tree starting from a given PID
func (pt *ProcessTree) PrintTree(w io.Writer, pid int, depth int, tmpl *template.Template, maxArgsLen *int) {
	node, exists := pt.Nodes[pid]
	if !exists {
		return
	}

	// Print the current node
	if node.Process != nil && len(node.Execs) > 0 {
		indent := strings.Repeat("  ", depth)
		for _, exec := range node.Execs {
			if tmpl != nil {
				err := tmpl.Execute(w, exec)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Template error: %v\n", err)
				}
			} else {
				if exec.Event.Exec != nil && len(exec.Event.Exec.Args) > 0 {
					programName := exec.Event.Exec.Args[0]
					argsStr := ""

					// Add arguments with user-defined length limit
					if len(exec.Event.Exec.Args) > 1 {
						args := exec.Event.Exec.Args[1:]
						argsStr = " " + strings.Join(args, " ")

						// Check if this is a bash -c or similar wrapper
						if programName == "/bin/bash" || programName == "/bin/sh" {
							if len(args) >= 2 && (args[0] == "-c" || (len(args) >= 3 && args[1] == "-c")) {
								// Find the actual command being executed
								var cmdIndex int
								if args[0] == "-c" {
									cmdIndex = 1
								} else {
									// Skip options like -l before -c
									for i := 1; i < len(args); i++ {
										if args[i] == "-c" && i+1 < len(args) {
											cmdIndex = i + 1
											break
										}
									}
								}

								if cmdIndex > 0 && cmdIndex < len(args) {
									// Extract the actual command from the bash -c argument
									cmd := args[cmdIndex]

									// Process command using configured extractors
									extracted := false
									for _, extractor := range pt.Config.CommandExtractors {
										pattern := regexp.MustCompile(extractor.Pattern)
										if pattern.MatchString(cmd) {
											matches := pattern.FindStringSubmatch(cmd)
											if len(matches) > extractor.Group {
												// Replace the displayed command
												programName = extractor.DisplayName
												argsStr = " " + matches[extractor.Group]
												extracted = true
												break
											}
										}
									}

									// If no pattern matched, fall back to showing the whole command
									if !extracted {
										programName = "SHELL:"
										argsStr = " " + cmd
									}
								}
							}
						}

						// Apply length limit after extracting the real command
						if *maxArgsLen > 0 && len(argsStr) > *maxArgsLen {
							argsStr = argsStr[:*maxArgsLen-3] + "..."
						}
					}

					// Get TTY path if available
					ttyInfo := ""
					if node.Process.TTY != nil && node.Process.TTY.Path != "" {
						ttyName := node.Process.TTY.Path
						// Extract just the terminal name from the path
						if parts := strings.Split(ttyName, "/"); len(parts) > 0 {
							ttyName = parts[len(parts)-1]
						}
						ttyInfo = fmt.Sprintf(",TTY:%s", ttyName)
					}

					// Calculate relative time in seconds from start
					relativeTime := ""
					if !pt.FirstStartTime.IsZero() {
						processTime, err := time.Parse(time.RFC3339Nano, exec.Time)
						if err == nil {
							seconds := processTime.Sub(pt.FirstStartTime).Seconds()
							relativeTime = fmt.Sprintf(",T:%.1fs", seconds)
						}
					}

					fmt.Fprintf(w, "%s[SEQ:%d%s,OPPID:%d,PPID:%d,PID:%d%s] %s%s\n",
						indent,
						exec.SeqNum,
						relativeTime,
						node.Process.OriginalPPID,
						node.Process.PPID,
						node.Process.AuditToken.PID,
						ttyInfo,
						programName,
						argsStr)
				} else {
					// Get TTY path if available
					ttyInfo := ""
					if node.Process.TTY != nil && node.Process.TTY.Path != "" {
						ttyName := node.Process.TTY.Path
						// Extract just the terminal name from the path
						if parts := strings.Split(ttyName, "/"); len(parts) > 0 {
							ttyName = parts[len(parts)-1]
						}
						ttyInfo = fmt.Sprintf(",TTY:%s", ttyName)
					}

					// Calculate relative time in seconds from start
					relativeTime := ""
					if !pt.FirstStartTime.IsZero() {
						processTime, err := time.Parse(time.RFC3339Nano, exec.Time)
						if err == nil {
							seconds := processTime.Sub(pt.FirstStartTime).Seconds()
							relativeTime = fmt.Sprintf(",T:%.1fs", seconds)
						}
					}

					fmt.Fprintf(w, "%s[SEQ:%d%s,OPPID:%d,PPID:%d,PID:%d%s] (no args)\n",
						indent,
						exec.SeqNum,
						relativeTime,
						node.Process.OriginalPPID,
						node.Process.PPID,
						node.Process.AuditToken.PID,
						ttyInfo)
				}
			}
		}
	}

	// Print children recursively
	for _, child := range node.Children {
		if child.Process != nil {
			pt.PrintTree(w, child.Process.AuditToken.PID, depth+1, tmpl, maxArgsLen)
		}
	}
}

// FilterFunc defines a filter function for events
type FilterFunc func(*ESEvent) bool

// CreateProcessPIDFilter creates a filter for process PID
func CreateProcessPIDFilter(pid int) FilterFunc {
	return func(event *ESEvent) bool {
		return event.Process.AuditToken.PID == pid
	}
}

// CreateProcessNameFilter creates a filter for process executable name
func CreateProcessNameFilter(name string) FilterFunc {
	return func(event *ESEvent) bool {
		execPath := event.Process.Executable.Path
		return strings.Contains(execPath, name)
	}
}

// CreateEventTypeFilter creates a filter for event type
func CreateEventTypeFilter(eventType int) FilterFunc {
	return func(event *ESEvent) bool {
		return event.EventType == eventType
	}
}

// CreateSequenceFilter creates a filter for sequence number range
func CreateSequenceFilter(start, end int) FilterFunc {
	return func(event *ESEvent) bool {
		if start > 0 && event.SeqNum < start {
			return false
		}
		if end > 0 && event.SeqNum > end {
			return false
		}
		return true
	}
}

// CreatePPIDFilter creates a filter for parent process ID
func CreatePPIDFilter(ppid int) FilterFunc {
	return func(event *ESEvent) bool {
		return event.Process.PPID == ppid
	}
}

// CreateTTYFilter creates a filter for TTY path
func CreateTTYFilter(ttyPath string) FilterFunc {
	return func(event *ESEvent) bool {
		// First check process TTY
		if event.Process.TTY != nil && event.Process.TTY.Path != "" {
			if strings.Contains(event.Process.TTY.Path, ttyPath) {
				return true
			}
		}

		// If event is an exec event, also check target TTY
		if event.Event.Exec != nil && event.Event.Exec.Target.TTY != nil && event.Event.Exec.Target.TTY.Path != "" {
			return strings.Contains(event.Event.Exec.Target.TTY.Path, ttyPath)
		}

		return false
	}
}

func main() {
	// Define command line flags
	inputFile := flag.String("file", "", "Input file (if not specified, stdin is used)")
	outputFile := flag.String("output", "", "Output file (if not specified, stdout is used)")
	formatStr := flag.String("format", "default", "Output format (default, json, or template)")
	templateStr := flag.String("template", "{{.Process.Executable.Path}} [PID:{{.Process.AuditToken.PID}}]", "Go template for output")
	filterPID := flag.Int("pid", 0, "Filter by process ID")
	filterName := flag.String("name", "", "Filter by process name")
	filterEvent := flag.Int("event", 0, "Filter by event type")
	filterTTY := flag.String("tty", "", "Filter by TTY path (e.g., 'ttys009')")
	rootPID := flag.Int("root", 0, "Root PID for tree view (0 for all root processes)")
	showEnv := flag.Bool("show-sensitive", false, "Show sensitive environment variables (disabled by default)")
	maxArgsLen := flag.Int("max-args", 120, "Maximum length for command arguments display")
	seqStart := flag.Int("seq-start", 0, "Filter events with sequence number greater than or equal to this value")
	seqEnd := flag.Int("seq-end", 0, "Filter events with sequence number less than or equal to this value")
	ppidFilter := flag.Int("ppid-filter", 0, "Only show events within a specific PPID group")
	jsonMode := flag.String("json", "", "JSON output mode: 'clean' for essential fields only, 'raw' for full events")
	configPath := flag.String("config", "", "Path to configuration file (defaults to ~/.eslogrc.json if exists)")
	dumpConfig := flag.Bool("dump-config", false, "Dump default configuration as template and exit")
	follow := flag.Bool("F", false, "Follow file in real-time, processing events as they are added")
	tuiMode := flag.Bool("tui", false, "Start interactive TUI (Terminal User Interface) mode")
	showFileOps := flag.Bool("file-ops", false, "Show detailed file operation counts")

	flag.Parse()

	// If dump-config flag is set, print the default config and exit
	if *dumpConfig {
		config := DefaultConfig()
		jsonConfig, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonConfig))
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

	// Create filters
	var filters []FilterFunc
	if *filterPID != 0 {
		filters = append(filters, CreateProcessPIDFilter(*filterPID))
	}
	if *filterName != "" {
		filters = append(filters, CreateProcessNameFilter(*filterName))
	}
	if *filterEvent != 0 {
		filters = append(filters, CreateEventTypeFilter(*filterEvent))
	}
	if *filterTTY != "" {
		filters = append(filters, CreateTTYFilter(*filterTTY))
	}
	if *seqStart > 0 || *seqEnd > 0 {
		filters = append(filters, CreateSequenceFilter(*seqStart, *seqEnd))
	}
	if *ppidFilter > 0 {
		filters = append(filters, CreatePPIDFilter(*ppidFilter))
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
	processTree := NewProcessTree()

	// Load user configuration if specified
	if *configPath != "" {
		userConfig, err := LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration from %s: %v\n", *configPath, err)
		} else {
			processTree.Config = userConfig
		}
	} else {
		// If configPath is not provided, check for ~/.eslogrc.json
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultConfigPath := filepath.Join(homeDir, ".eslogrc.json")
			if _, err := os.Stat(defaultConfigPath); err == nil {
				configFromHome, err := LoadConfig(defaultConfigPath)
				if err == nil {
					processTree.Config = configFromHome
				}
			}
		}
	}

	// Create a customized AddEvent function based on showEnv flag
	addEventFunc := func(pt *ProcessTree, event *ESEvent) {
		// Skip if not an exec event
		if event.Event.Exec == nil {
			return
		}

		// Filter sensitive env vars if requested
		if !*showEnv && event.Event.Exec.Env != nil {
			event.Event.Exec.Env = filterSensitiveEnvVars(event.Event.Exec.Env)
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
			node = &ProcessNode{
				Process:  &event.Process,
				Children: make([]*ProcessNode, 0),
				Execs:    make([]*ESEvent, 0),
			}
			pt.Nodes[pid] = node
		}

		// Add exec event to the node
		node.Execs = append(node.Execs, event)

		// Set up parent relationship
		parentNode, parentExists := pt.Nodes[ppid]
		if !parentExists {
			// Create a placeholder parent node
			parentNode = &ProcessNode{
				Process:    nil, // Will be filled when we see the parent process event
				Children:   make([]*ProcessNode, 0),
				Execs:      make([]*ESEvent, 0),
				FileOps:    FileOpsCounter{},
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

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event ESEvent
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

		// Add to process tree
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
				cleanEvent := CleanEvent{
					SeqNum:       event.SeqNum,
					Time:         event.Time,
					PID:          event.Process.AuditToken.PID,
					PPID:         event.Process.PPID,
					OriginalPPID: event.Process.OriginalPPID,
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
		displayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)

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

							var event ESEvent
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

							// Add to process tree
							addEventFunc(processTree, &event)
							linesAdded = true

							// Handle JSON output modes for immediate display
							if *jsonMode != "" {
								displayJSONEvent(output, jsonMode, &event, processTree)
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
						if linesAdded && *jsonMode == "" {
							fmt.Fprintln(output, "\n--- Updated Process Tree ---")
							displayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)
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

	// Check if TUI mode is enabled
	if *tuiMode {
		// Launch TUI mode with the processed data
		if err := StartTUI(processTree); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Regular display mode
		// Display results if not in following mode or if not following a file
		if !*follow || *inputFile == "" {
			displayResults(output, formatStr, rootPID, processTree, tmpl, maxArgsLen)

			// If file-ops flag is set, display detailed file operation counts
			if *showFileOps {
				fmt.Fprintln(output, "\n--- File Operation Summary ---")
				totalReads := 0
				totalWrites := 0
				totalBytesRead := int64(0)
				totalBytesWritten := int64(0)

				for pid, node := range processTree.Nodes {
					if node.FileOps.Reads > 0 || node.FileOps.Writes > 0 {
						execPath := "unknown"
						if node.Process != nil {
							execPath = node.Process.Executable.Path
						}
						fmt.Fprintf(output, "PID %d (%s):\n", pid, execPath)
						fmt.Fprintf(output, "  Reads:  %d (%.2f KB)\n", node.FileOps.Reads, float64(node.FileOps.BytesRead)/1024.0)
						fmt.Fprintf(output, "  Writes: %d (%.2f KB)\n", node.FileOps.Writes, float64(node.FileOps.BytesWritten)/1024.0)

						totalReads += node.FileOps.Reads
						totalWrites += node.FileOps.Writes
						totalBytesRead += node.FileOps.BytesRead
						totalBytesWritten += node.FileOps.BytesWritten
					}
				}

				fmt.Fprintln(output, "\nTotals:")
				fmt.Fprintf(output, "  Total Reads:  %d (%.2f KB)\n", totalReads, float64(totalBytesRead)/1024.0)
				fmt.Fprintf(output, "  Total Writes: %d (%.2f KB)\n", totalWrites, float64(totalBytesWritten)/1024.0)
			}
		}
	}
}
