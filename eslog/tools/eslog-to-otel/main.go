package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	
	// OTLP metric exporters
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
)

// ESEvent matches the structure in the main eslog package
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

// EventData contains the event details
type EventData struct {
	Exec     *ExecEvent     `json:"exec,omitempty"`
	Lookup   *LookupEvent   `json:"lookup,omitempty"`
	Readlink *ReadlinkEvent `json:"readlink,omitempty"`
	Stat     *StatEvent     `json:"stat,omitempty"`
	Access   *AccessEvent   `json:"access,omitempty"`
	Open     *OpenEvent     `json:"open,omitempty"`
	Close    *CloseEvent    `json:"close,omitempty"`
	Exit     *ExitEvent     `json:"exit,omitempty"`
	Read     *ReadEvent     `json:"read,omitempty"`
	Write    *WriteEvent    `json:"write,omitempty"`
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

// FileDescriptor represents a file descriptor
type FileDescriptor struct {
	FD     int `json:"fd"`
	FDType int `json:"fdtype"`
}

// File operation event types
type LookupEvent struct {
	SourceDir    FileInfo `json:"source_dir"`
	RelativePath string   `json:"relative_path"`
}

type ReadlinkEvent struct {
	Source FileInfo `json:"source"`
}

type StatEvent struct {
	Source FileInfo `json:"source"`
}

type AccessEvent struct {
	Source FileInfo `json:"source"`
	Mode   int      `json:"mode"`
}

type OpenEvent struct {
	File FileInfo `json:"file"`
	Mode int      `json:"mode"`
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

// FileOperationMetrics holds counters for file operations by process
type FileOperationMetrics struct {
	Lookups      int64
	Stats        int64
	Readlinks    int64
	Accesses     int64
	Opens        int64
	Closes       int64
	Reads        int64
	Writes       int64
	BytesRead    int64
	BytesWritten int64
	mu           sync.Mutex // For thread safety when incrementing
}

// Increment the lookup counter
func (m *FileOperationMetrics) IncrementLookups() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Lookups++
}

// Increment the stat counter
func (m *FileOperationMetrics) IncrementStats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Stats++
}

// Increment the readlink counter
func (m *FileOperationMetrics) IncrementReadlinks() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Readlinks++
}

// Increment the access counter
func (m *FileOperationMetrics) IncrementAccesses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Accesses++
}

// Increment the open counter
func (m *FileOperationMetrics) IncrementOpens() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Opens++
}

// Increment the close counter
func (m *FileOperationMetrics) IncrementCloses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Closes++
}

// Increment the read counter and add bytes
func (m *FileOperationMetrics) IncrementReads(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Reads++
	m.BytesRead += bytes
}

// Increment the write counter and add bytes
func (m *FileOperationMetrics) IncrementWrites(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Writes++
	m.BytesWritten += bytes
}

var (
	// OpenTelemetry export options
	otelExporter    = flag.String("exporter", "stdout", "OpenTelemetry exporter to use (stdout, otlp, otlphttp)")
	otelEndpoint    = flag.String("endpoint", "localhost:4317", "OpenTelemetry collector endpoint when using otlp exporter")
	serviceName     = flag.String("service", "eslog", "Service name to use in OpenTelemetry traces")

	// Process filtering
	filterPID       = flag.Int("pid", 0, "Filter for specific process ID")
	filterName      = flag.String("name", "", "Filter by process name (substring match)")

	// Event handling options
	useMetrics      = flag.Bool("use-metrics", true, "Use metrics for file operation counts instead of span attributes")
	aggregateIO     = flag.Bool("aggregate-io", true, "Aggregate file I/O as attributes instead of spans")
	skipStats       = flag.Bool("skip-stats", true, "Skip creating spans for stat events (reduces trace size)")
	skipLookups     = flag.Bool("skip-lookups", true, "Skip creating spans for lookup events (reduces trace size)")

	// Metrics options
	temporality     = flag.String("temporality", "delta", "Aggregation temporality for metrics (delta or cumulative)")
	exportInterval  = flag.Duration("metrics-export", 5*time.Second, "Metrics export interval")

	// Root span options
	createRootSpan  = flag.Bool("create-root-span", true, "Create a root span for the session")
	rootSpanName    = flag.String("root-span-name", "eslog-session", "Name for the root span")
	respectTrace    = flag.Bool("respect-traceparent", true, "Respect existing TRACEPARENT env var if present")

	// Misc options
	batchSize       = flag.Int("batch", 100, "Number of events to process before printing a status update")
	verbose         = flag.Bool("verbose", false, "Enable verbose output")
)

// processMetrics maps PIDs to their file operation metrics
var processMetrics = make(map[int]*FileOperationMetrics)
var processMetricsMu sync.Mutex // For thread safety when accessing the map

// OTel metrics
var (
	lookupCounter    metric.Int64Counter
	statCounter      metric.Int64Counter
	readlinkCounter  metric.Int64Counter
	accessCounter    metric.Int64Counter
	openCounter      metric.Int64Counter
	closeCounter     metric.Int64Counter
	readCounter      metric.Int64Counter
	writeCounter     metric.Int64Counter
	bytesReadCounter metric.Int64Counter
	bytesWriteCounter metric.Int64Counter
)

// initMetrics initializes all the metrics with the OpenTelemetry API
func initMetrics(meter metric.Meter) error {
	var err error

	// File operation count metrics
	lookupCounter, err = meter.Int64Counter(
		"eslog.file.lookups",
		metric.WithDescription("Number of file lookup operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create lookup counter: %w", err)
	}

	statCounter, err = meter.Int64Counter(
		"eslog.file.stats",
		metric.WithDescription("Number of file stat operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create stat counter: %w", err)
	}

	readlinkCounter, err = meter.Int64Counter(
		"eslog.file.readlinks",
		metric.WithDescription("Number of readlink operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create readlink counter: %w", err)
	}

	accessCounter, err = meter.Int64Counter(
		"eslog.file.accesses",
		metric.WithDescription("Number of file access operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create access counter: %w", err)
	}

	openCounter, err = meter.Int64Counter(
		"eslog.file.opens",
		metric.WithDescription("Number of file open operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create open counter: %w", err)
	}

	closeCounter, err = meter.Int64Counter(
		"eslog.file.closes",
		metric.WithDescription("Number of file close operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create close counter: %w", err)
	}

	readCounter, err = meter.Int64Counter(
		"eslog.file.reads",
		metric.WithDescription("Number of file read operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create read counter: %w", err)
	}

	writeCounter, err = meter.Int64Counter(
		"eslog.file.writes",
		metric.WithDescription("Number of file write operations"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create write counter: %w", err)
	}

	// Byte counters for read/write operations
	bytesReadCounter, err = meter.Int64Counter(
		"eslog.file.bytes_read",
		metric.WithDescription("Number of bytes read from files"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create bytes read counter: %w", err)
	}

	bytesWriteCounter, err = meter.Int64Counter(
		"eslog.file.bytes_written",
		metric.WithDescription("Number of bytes written to files"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create bytes written counter: %w", err)
	}

	return nil
}

// getOrCreateMetrics gets or creates metrics for a process
func getOrCreateMetrics(pid int) *FileOperationMetrics {
	processMetricsMu.Lock()
	defer processMetricsMu.Unlock()

	if metrics, ok := processMetrics[pid]; ok {
		return metrics
	}

	metrics := &FileOperationMetrics{}
	processMetrics[pid] = metrics
	return metrics
}

// recordMetricsForProcess records all metrics for a process using OTel metrics
func recordMetricsForProcess(ctx context.Context, pid int, executable string) {
	processMetricsMu.Lock()
	metrics, exists := processMetrics[pid]
	processMetricsMu.Unlock()

	if !exists || metrics == nil {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	// Create common attributes for this process
	attrs := []attribute.KeyValue{
		attribute.Int("pid", pid),
		attribute.String("executable", executable),
	}

	// Record each metric with process attributes
	lookupCounter.Add(ctx, metrics.Lookups, metric.WithAttributes(attrs...))
	statCounter.Add(ctx, metrics.Stats, metric.WithAttributes(attrs...))
	readlinkCounter.Add(ctx, metrics.Readlinks, metric.WithAttributes(attrs...))
	accessCounter.Add(ctx, metrics.Accesses, metric.WithAttributes(attrs...))
	openCounter.Add(ctx, metrics.Opens, metric.WithAttributes(attrs...))
	closeCounter.Add(ctx, metrics.Closes, metric.WithAttributes(attrs...))
	readCounter.Add(ctx, metrics.Reads, metric.WithAttributes(attrs...))
	writeCounter.Add(ctx, metrics.Writes, metric.WithAttributes(attrs...))
	bytesReadCounter.Add(ctx, metrics.BytesRead, metric.WithAttributes(attrs...))
	bytesWriteCounter.Add(ctx, metrics.BytesWritten, metric.WithAttributes(attrs...))

	if *verbose {
		fmt.Fprintf(os.Stderr, "Recorded metrics for PID %d (%s): L:%d S:%d R:%d A:%d O:%d C:%d Rd:%d Wr:%d BytesR:%d BytesW:%d\n",
			pid, executable, metrics.Lookups, metrics.Stats, metrics.Readlinks,
			metrics.Accesses, metrics.Opens, metrics.Closes, metrics.Reads, 
			metrics.Writes, metrics.BytesRead, metrics.BytesWritten)
	}
}

// DetectAndLinkW3CContext checks for W3C trace context in environment variables
// and adds linking information to the span
func DetectAndLinkW3CContext(span trace.Span, env []string, pid int, verbose bool) {
	if env == nil || len(env) == 0 {
		return
	}

	for _, envVar := range env {
		// Look for traceparent in common environment variable formats
		if strings.HasPrefix(envVar, "TRACEPARENT=") ||
			strings.HasPrefix(envVar, "HTTP_TRACEPARENT=") ||
			strings.HasPrefix(envVar, "OTEL_TRACEPARENT=") {

			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) != 2 {
				continue
			}

			traceParentValue := parts[1]
			if verbose {
				fmt.Fprintf(os.Stderr, "Found traceparent: %s in PID %d\n", traceParentValue, pid)
			}

			// Add trace parent as an attribute for linking in trace viewers
			span.SetAttributes(attribute.String("traceparent", traceParentValue))

			// Parse traceparent value (format: 00-traceID-spanID-flags)
			tpParts := strings.Split(traceParentValue, "-")
			if len(tpParts) != 4 {
				continue
			}

			// Extract trace ID and parent span ID
			traceIDHex := tpParts[1]
			parentSpanIDHex := tpParts[2]

			// Add as attributes for better visibility in trace viewers
			span.SetAttributes(
				attribute.String("linked_trace_id", traceIDHex),
				attribute.String("linked_span_id", parentSpanIDHex),
			)

			// Try to parse and create actual links if possible
			if len(traceIDHex) == 32 && len(parentSpanIDHex) == 16 {
				var traceID trace.TraceID
				var spanID trace.SpanID

				// Decode hex strings to bytes
				traceIDBytes, err1 := hex.DecodeString(traceIDHex)
				spanIDBytes, err2 := hex.DecodeString(parentSpanIDHex)

				if err1 == nil && len(traceIDBytes) == 16 && err2 == nil && len(spanIDBytes) == 8 {
					copy(traceID[:], traceIDBytes)
					copy(spanID[:], spanIDBytes)

					if verbose {
						fmt.Fprintf(os.Stderr, "Created link to external trace ID: %s, span ID: %s\n",
							traceID.String(), spanID.String())
					}

					// Add special attributes for trace visualization tools
					span.SetAttributes(
						attribute.String("otel.trace_id.linked", traceID.String()),
						attribute.String("otel.span_id.linked", spanID.String()),
					)
				}
			}

			// Only process the first valid traceparent we find
			break
		}
	}
}

func main() {
	// Show usage if no arguments are provided
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [file...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "If no files are specified, input is read from stdin.\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	flag.Parse()

	// Set up OpenTelemetry resource with proper identification
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(*serviceName),
		semconv.ServiceVersion("1.0.0"),
		attribute.String("environment", "production"),
		attribute.String("application", "eslog"),
		attribute.String("application.component", "process_monitor"),
	)

	// Set up OpenTelemetry tracing
	tp, err := initTracer(*serviceName, *otelExporter, *otelEndpoint, res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing OpenTelemetry tracing: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down tracer provider: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Successfully shut down tracer provider\n")
		}
	}()

	// Declare maps that will be used throughout the program
	processSpans := make(map[int]trace.Span)
	var processNames map[int]string
	var processNamesMu sync.Mutex

	// Set up OpenTelemetry metrics if enabled
	var mp *sdkmetric.MeterProvider
	if *useMetrics {
		// Initialize process names map
		processNames = make(map[int]string)
		
		mp, err = initMeterProvider(*serviceName, *otelExporter, *otelEndpoint, res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing OpenTelemetry metrics: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := mp.Shutdown(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error shutting down meter provider: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Successfully shut down meter provider\n")
			}
		}()

		// Initialize metric instruments
		meter := mp.Meter("github.com/tmc/misc/eslog/tools/eslog-to-otel")
		if err := initMetrics(meter); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing metrics: %v\n", err)
			os.Exit(1)
		}

		// Set up periodic metrics export if using metrics
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start a goroutine to periodically record metrics
		go func() {
			ticker := time.NewTicker(*exportInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Iterate through all processes and record their metrics
					processMetricsMu.Lock()
					processNamesMu.Lock()
					for pid := range processMetrics {
						// Get the process name
						execName := processNames[pid]
						if execName == "" {
							execName = fmt.Sprintf("pid-%d", pid) // Default name with PID
						}
						
						// Record metrics for this process
						recordMetricsForProcess(ctx, pid, execName)
					}
					processNamesMu.Unlock()
					processMetricsMu.Unlock()
				}
			}
		}()
	}

	// Create a root span for the session if configured
	rootCtx := context.Background()
	var rootSpan trace.Span
	var rootTraceID trace.TraceID

	// Check for existing TRACEPARENT in environment if respectTrace is enabled
	existingTraceParent := ""
	var existingTraceID trace.TraceID
	var existingSpanID trace.SpanID
	if *respectTrace {
		// Check common environment variables for traceparent
		envVars := []string{"TRACEPARENT", "HTTP_TRACEPARENT", "OTEL_TRACEPARENT"}
		for _, ev := range envVars {
			if val, exists := os.LookupEnv(ev); exists && val != "" {
				existingTraceParent = val
				// Parse traceparent value (format: 00-traceID-spanID-flags)
				tpParts := strings.Split(val, "-")
				if len(tpParts) == 4 {
					// Extract trace ID and parent span ID
					traceIDHex := tpParts[1]
					spanIDHex := tpParts[2]

					if len(traceIDHex) == 32 && len(spanIDHex) == 16 {
						traceIDBytes, err1 := hex.DecodeString(traceIDHex)
						spanIDBytes, err2 := hex.DecodeString(spanIDHex)

						if err1 == nil && len(traceIDBytes) == 16 && err2 == nil && len(spanIDBytes) == 8 {
							copy(existingTraceID[:], traceIDBytes)
							copy(existingSpanID[:], spanIDBytes)
							if *verbose {
								fmt.Fprintf(os.Stderr, "Found existing traceparent: %s\n", existingTraceParent)
								fmt.Fprintf(os.Stderr, "Using existing trace ID: %s\n", existingTraceID.String())
							}
							break
						}
					}
				}
			}
		}
	}

	if *createRootSpan {
		// Create the root span for this processing session
		startTime := time.Now()

		// If we have an existing trace context and respectTrace is true, link to it
		var rootSpanOpts []trace.SpanStartOption
		rootSpanOpts = append(rootSpanOpts,
			trace.WithAttributes(
				semconv.ServiceName(*serviceName),
				semconv.ProcessRuntimeNameKey.String("eslog-to-otel"),
				semconv.ProcessRuntimeVersionKey.String("1.0.0"),
				attribute.String("session.start_time", startTime.Format(time.RFC3339)),
			),
		)

		if existingTraceParent != "" && *respectTrace {
			rootSpanOpts = append(rootSpanOpts,
				trace.WithAttributes(
					attribute.String("traceparent", existingTraceParent),
					attribute.String("linked_trace_id", existingTraceID.String()),
					attribute.String("linked_span_id", existingSpanID.String()),
				),
			)
		}

		rootCtx, rootSpan = otel.Tracer(*serviceName).Start(
			rootCtx,
			*rootSpanName,
			rootSpanOpts...,
		)

		rootTraceID = rootSpan.SpanContext().TraceID()
		if *verbose {
			fmt.Fprintf(os.Stderr, "Created root span with trace ID: %s\n", rootTraceID.String())
		}

		// Make sure the root span is ended when we're done
		defer rootSpan.End()
	}

	// Set up additional maps
	traceIDMap := make(map[int]trace.TraceID) // Map process IDs to trace IDs
	pendingEvents := make(map[int][]ESEvent)  // Store events that arrive before their parent process's span is created

	// Setup input sources - either files from arguments or stdin
	var inputSources []io.Reader

	// Get non-flag arguments as file paths
	inputFiles := flag.Args()
	if len(inputFiles) > 0 {
		for _, filePath := range inputFiles {
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filePath, err)
				continue
			}
			defer file.Close()
			inputSources = append(inputSources, file)
			if *verbose {
				fmt.Fprintf(os.Stderr, "Added file for processing: %s\n", filePath)
			}
		}
	} else {
		// Check if stdin has data
		stdinStat, _ := os.Stdin.Stat()
		if (stdinStat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintf(os.Stderr, "No input files specified and no data on stdin.\n")
			fmt.Fprintf(os.Stderr, "Usage: %s [options] [file...]\n", os.Args[0])
			os.Exit(1)
		}
		inputSources = append(inputSources, os.Stdin)
		if *verbose {
			fmt.Fprintf(os.Stderr, "Reading from stdin\n")
		}
	}

	// Process events from all input sources
	count := 0
	fmt.Fprintf(os.Stderr, "Starting to process events...\n")

	// Create a multi-reader if we have multiple input sources
	var input io.Reader
	if len(inputSources) > 1 {
		input = io.MultiReader(inputSources...)
	} else if len(inputSources) == 1 {
		input = inputSources[0]
	} else {
		fmt.Fprintf(os.Stderr, "No input sources available\n")
		os.Exit(1)
	}

	// Setup scanner for reading JSON events
	scanner := bufio.NewScanner(input)
	largeBuffer := make([]byte, 10*1024*1024) // 10MB buffer for large events
	scanner.Buffer(largeBuffer, 10*1024*1024)

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

		// Apply process filters if specified
		pid := event.Process.AuditToken.PID
		if *filterPID > 0 && pid != *filterPID {
			continue
		}

		if *filterName != "" {
			execPath := event.Process.Executable.Path
			if !strings.Contains(strings.ToLower(execPath), strings.ToLower(*filterName)) {
				continue
			}
		}

		count++
		if count%*batchSize == 0 && *verbose {
			fmt.Fprintf(os.Stderr, "Processed %d events...\n", count)
		}

		// Extract process info
		ppid := event.Process.PPID
		execPath := event.Process.Executable.Path

		// Get or create span for this process
		ctx := context.Background()
		span, exists := processSpans[pid]

		if !exists {
			// If parent process has a span, use its context
			if parentSpan, parentExists := processSpans[ppid]; parentExists {
				// Create as child span
				parentCtx := trace.ContextWithSpan(ctx, parentSpan)
				parentTraceID := parentSpan.SpanContext().TraceID()

				// Extract just the executable name for a more readable span name
				execName := execPath
				if parts := strings.Split(execPath, "/"); len(parts) > 0 {
					execName = parts[len(parts)-1]
				}

				// Apply semantic conventions for process spans
				ctx, span = otel.Tracer(*serviceName).Start(
					parentCtx,
					fmt.Sprintf("process: %s", execName),
					trace.WithAttributes(
						// OpenTelemetry semantic conventions for process
						semconv.ProcessPIDKey.Int(pid),
						semconv.ProcessParentPIDKey.Int(ppid),
						semconv.ProcessExecutableName(execName),
						semconv.ProcessExecutablePath(execPath),
						semconv.ProcessCommandKey.String(execName),
						// Additional process-specific attributes
						attribute.Int("process.original_ppid", event.Process.OriginalPPID),
						attribute.String("process.start_time", event.Process.StartTime),
					),
				)

				// Use the same trace ID as parent
				traceIDMap[pid] = parentTraceID
				
				// Store process name for metrics
				if *useMetrics {
					processNamesMu.Lock()
					processNames[pid] = execName
					processNamesMu.Unlock()
				}
			} else {
				// No parent span yet - we'll either create a root span or temporarily store the event
				if ppid == 0 || ppid == 1 {
					// This is a root process or direct child of init
					// Extract just the executable name for a more readable span name
					execName := execPath
					if parts := strings.Split(execPath, "/"); len(parts) > 0 {
						execName = parts[len(parts)-1]
					}

					// Apply semantic conventions for process spans
					parentCtx := rootCtx
					if *createRootSpan {
						parentCtx = trace.ContextWithSpan(parentCtx, rootSpan)
					}
					
					ctx, span = otel.Tracer(*serviceName).Start(
						parentCtx,
						fmt.Sprintf("process: %s", execName),
						trace.WithAttributes(
							// OpenTelemetry semantic conventions for process
							semconv.ProcessPIDKey.Int(pid),
							semconv.ProcessParentPIDKey.Int(ppid),
							semconv.ProcessExecutableName(execName),
							semconv.ProcessExecutablePath(execPath),
							semconv.ProcessCommandKey.String(execName),
							// Additional process-specific attributes
							attribute.Int("process.original_ppid", event.Process.OriginalPPID),
							attribute.String("process.start_time", event.Process.StartTime),
						),
					)

					// Store the trace ID
					traceIDMap[pid] = span.SpanContext().TraceID()
					
					// Store process name for metrics
					if *useMetrics {
						processNamesMu.Lock()
						processNames[pid] = execName
						processNamesMu.Unlock()
					}
				} else {
					// Store event for processing once parent span is available
					pendingEvents[pid] = append(pendingEvents[pid], event)
					continue
				}
			}

			// Store the span
			processSpans[pid] = span

			// Create metrics for this process if we're using metrics
			if *useMetrics {
				getOrCreateMetrics(pid)
			}

			// Process any pending events for this PID's children
			processPendingChildEvents(pid, processSpans, traceIDMap, pendingEvents, processNames, &processNamesMu)
		}

		// Determine if this is a file operation that we should track with metrics
		isFileOperation := false
		var fileOpMetrics *FileOperationMetrics

		if *useMetrics {
			// Get the metrics for this process
			fileOpMetrics = getOrCreateMetrics(pid)
		}

		// Determine event type and update metrics accordingly
		if event.Event.Lookup != nil {
			// Lookup event - increment lookup counter
			if *useMetrics {
				fileOpMetrics.IncrementLookups()
				isFileOperation = true
			}
			
			// Skip creating spans for lookups if configured
			if *skipLookups {
				continue
			}
		} else if event.Event.Stat != nil {
			// Stat event - increment stat counter
			if *useMetrics {
				fileOpMetrics.IncrementStats()
				isFileOperation = true
			}
			
			// Skip creating spans for stats if configured
			if *skipStats {
				continue
			}
		} else if event.Event.Readlink != nil {
			// Readlink event - increment readlink counter
			if *useMetrics {
				fileOpMetrics.IncrementReadlinks()
				isFileOperation = true
			}
		} else if event.Event.Access != nil {
			// Access event - increment access counter
			if *useMetrics {
				fileOpMetrics.IncrementAccesses()
				isFileOperation = true
			}
		} else if event.Event.Open != nil {
			// Open event - increment open counter
			if *useMetrics {
				fileOpMetrics.IncrementOpens()
				isFileOperation = true
			}
		} else if event.Event.Close != nil {
			// Close event - increment close counter
			if *useMetrics {
				fileOpMetrics.IncrementCloses()
				isFileOperation = true
			}
		} else if event.Event.Read != nil {
			// Read event - increment read counter and bytes
			if *useMetrics {
				size := int64(0)
				if event.Event.Read.Size > 0 {
					size = event.Event.Read.Size
				}
				fileOpMetrics.IncrementReads(size)
				isFileOperation = true
			}
		} else if event.Event.Write != nil {
			// Write event - increment write counter and bytes
			if *useMetrics {
				size := int64(0)
				if event.Event.Write.Size > 0 {
					size = event.Event.Write.Size
				}
				fileOpMetrics.IncrementWrites(size)
				isFileOperation = true
			}
		}

		// If this is a file operation and we're using metrics, skip creating a span
		if isFileOperation && *useMetrics && *aggregateIO {
			continue
		}

		// For non-file operations or when we want spans for file operations
		// Create span with readable name while preserving original info in attributes
		eventName := "unknown"
		eventDescription := "unknown event"

		if event.Event.Exec != nil {
			eventName = "exec"
			command := "unknown"
			if len(event.Event.Exec.Args) > 0 {
				command = event.Event.Exec.Args[0]
				// Extract just the base command without path
				if parts := strings.Split(command, "/"); len(parts) > 0 {
					command = parts[len(parts)-1]
				}
			}
			eventDescription = fmt.Sprintf("exec %s", command)
		} else if event.Event.Exit != nil {
			eventName = "exit"
			eventDescription = fmt.Sprintf("exit (code: %d)", event.Event.Exit.ExitCode)
		} else if event.Event.Open != nil {
			eventName = "open"
			path := "unknown"
			if event.Event.Open.File.Path != "" {
				path = event.Event.Open.File.Path
				// Extract just the file name without full path
				if parts := strings.Split(path, "/"); len(parts) > 0 {
					path = parts[len(parts)-1]
				}
			}
			eventDescription = fmt.Sprintf("open %s", path)
		} else if event.Event.Close != nil {
			eventName = "close"
			eventDescription = fmt.Sprintf("close fd %d", event.Event.Close.Target.FD)
		} else if event.Event.Read != nil {
			eventName = "read"
			size := int64(0)
			if event.Event.Read.Size > 0 {
				size = event.Event.Read.Size
			}
			eventDescription = fmt.Sprintf("read fd %d (%d bytes)", event.Event.Read.FD.FD, size)
		} else if event.Event.Write != nil {
			eventName = "write"
			size := int64(0)
			if event.Event.Write.Size > 0 {
				size = event.Event.Write.Size
			}
			eventDescription = fmt.Sprintf("write fd %d (%d bytes)", event.Event.Write.FD.FD, size)
		} else if event.Event.Lookup != nil {
			eventName = "lookup"
			path := event.Event.Lookup.RelativePath
			eventDescription = fmt.Sprintf("lookup %s", path)
		} else if event.Event.Stat != nil {
			eventName = "stat"
			path := "unknown"
			if event.Event.Stat.Source.Path != "" {
				path = event.Event.Stat.Source.Path
				// Extract just the file name without full path
				if parts := strings.Split(path, "/"); len(parts) > 0 {
					path = parts[len(parts)-1]
				}
			}
			eventDescription = fmt.Sprintf("stat %s", path)
		} else if event.Event.Readlink != nil {
			eventName = "readlink"
			path := "unknown"
			if event.Event.Readlink.Source.Path != "" {
				path = event.Event.Readlink.Source.Path
				// Extract just the file name without full path
				if parts := strings.Split(path, "/"); len(parts) > 0 {
					path = parts[len(parts)-1]
				}
			}
			eventDescription = fmt.Sprintf("readlink %s", path)
		} else if event.Event.Access != nil {
			eventName = "access"
			path := "unknown"
			if event.Event.Access.Source.Path != "" {
				path = event.Event.Access.Source.Path
				// Extract just the file name without full path
				if parts := strings.Split(path, "/"); len(parts) > 0 {
					path = parts[len(parts)-1]
				}
			}
			eventDescription = fmt.Sprintf("access %s", path)
		}

		// Create span with proper attributes
		eventCtx := trace.ContextWithSpan(ctx, span)
		_, eventSpan := otel.Tracer(*serviceName).Start(
			eventCtx,
			eventDescription,
			trace.WithAttributes(
				// Event metadata with semantic conventions where applicable
				attribute.String("event.type", eventName),
				attribute.Int("es.event.seq_num", event.SeqNum),
				attribute.String("es.event.time", event.Time),
				attribute.Int64("es.event.mach_time", event.MachTime),
				semconv.ProcessPIDKey.Int(pid),
			),
		)

		// Add specific event attributes
		if event.Event.Exec != nil {
			// Check for W3C traceparent in environment variables
			if event.Event.Exec.Env != nil && len(event.Event.Exec.Env) > 0 {
				DetectAndLinkW3CContext(eventSpan, event.Event.Exec.Env, pid, *verbose)
			}
			
			if len(event.Event.Exec.Args) > 0 {
				command := ""
				if len(event.Event.Exec.Args) > 0 {
					command = event.Event.Exec.Args[0]
				}

				args := []string{}
				if len(event.Event.Exec.Args) > 1 {
					args = event.Event.Exec.Args[1:]
				}

				eventSpan.SetAttributes(
					semconv.ProcessCommandKey.String(command),
					semconv.ProcessCommandArgsKey.StringSlice(args),
				)

				if event.Event.Exec.CWD.Path != "" {
					eventSpan.SetAttributes(
						attribute.String("process.working_directory", event.Event.Exec.CWD.Path),
					)
				}
			}
		}

		if event.Event.Exit != nil {
			eventSpan.SetAttributes(
				attribute.Int("process.exit_code", event.Event.Exit.ExitCode),
				attribute.String("process.exit_reason", event.Event.Exit.Reason),
			)

			// End the process span when we get an exit event
			span.End()
		}

		// Events are short-lived
		eventSpan.End()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// End any remaining process spans
	for pid, span := range processSpans {
		// Record final metrics for this process if using metrics
		if *useMetrics {
			// Get the process name from our processNames map
			execName := ""
			if *useMetrics {
				processNamesMu.Lock()
				execName = processNames[pid]
				processNamesMu.Unlock()
			}

			// Record metrics for this process one last time
			if execName != "" {
				recordMetricsForProcess(context.Background(), pid, execName)
			}
		}

		// End the span
		span.End()
		fmt.Fprintf(os.Stderr, "Completed trace for process %d: %s\n", pid, traceIDMap[pid].String())
	}

	fmt.Fprintf(os.Stderr, "Processing complete. Total events: %d\n", count)
}

// processPendingChildEvents processes any events for children of the given PID
func processPendingChildEvents(
	pid int,
	processSpans map[int]trace.Span,
	traceIDMap map[int]trace.TraceID,
	pendingEvents map[int][]ESEvent,
	processNames map[int]string,
	processNamesMu *sync.Mutex,
) {
	// For each pending event
	for childPID, events := range pendingEvents {
		// Check if this is a child of the current PID
		if len(events) > 0 && events[0].Process.PPID == pid {
			parentSpan := processSpans[pid]
			parentCtx := trace.ContextWithSpan(context.Background(), parentSpan)
			parentTraceID := traceIDMap[pid]

			// Create child span with a more readable name
			execPath := events[0].Process.Executable.Path
			execName := execPath
			// Extract just the executable name without full path
			if parts := strings.Split(execPath, "/"); len(parts) > 0 {
				execName = parts[len(parts)-1]
			}

			// Apply semantic conventions
			_, span := otel.Tracer(*serviceName).Start(
				parentCtx,
				fmt.Sprintf("process: %s", execName),
				trace.WithAttributes(
					// OpenTelemetry semantic conventions for process
					semconv.ProcessPIDKey.Int(childPID),
					semconv.ProcessParentPIDKey.Int(pid),
					semconv.ProcessExecutableName(execName),
					semconv.ProcessExecutablePath(execPath),
					semconv.ProcessCommandKey.String(execName),
					// Additional process-specific attributes
					attribute.Int("process.original_ppid", events[0].Process.OriginalPPID),
					attribute.String("process.start_time", events[0].Process.StartTime),
				),
			)

			// Store span and trace ID
			processSpans[childPID] = span
			traceIDMap[childPID] = parentTraceID

			// Store process name for metrics
			if *useMetrics {
				processNamesMu.Lock()
				processNames[childPID] = execName
				processNamesMu.Unlock()
			}

			// Create metrics for this process if we're using metrics
			if *useMetrics {
				getOrCreateMetrics(childPID)
			}

			// Remove from pending events
			delete(pendingEvents, childPID)

			// Process this child's pending children recursively
			processPendingChildEvents(childPID, processSpans, traceIDMap, pendingEvents, processNames, processNamesMu)
		}
	}
}

// initTracer sets up an OpenTelemetry trace provider
func initTracer(serviceName, exporterType, endpoint string, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	// Set up the export pipeline
	var exporter sdktrace.SpanExporter
	var err error

	switch exporterType {
	case "stdout":
		// Use stdout exporter
		exporter, err = newStdoutExporter()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Using stdout exporter for traces\n")

	case "otlp":
		// Use OTLP gRPC exporter
		fmt.Fprintf(os.Stderr, "Initializing OTLP gRPC exporter for traces with endpoint: %s\n", endpoint)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithTimeout(5*time.Second),
		)

		exporter, err = otlptrace.New(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Successfully created OTLP gRPC exporter for traces\n")

	case "otlphttp":
		// Use OTLP HTTP exporter
		fmt.Fprintf(os.Stderr, "Initializing OTLP HTTP exporter for traces with endpoint: %s\n", endpoint)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
			otlptracehttp.WithTimeout(5*time.Second),
		)

		exporter, err = otlptrace.New(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Successfully created OTLP HTTP exporter for traces\n")

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}

	// Create trace provider with more frequent batch exports
	fmt.Fprintf(os.Stderr, "Creating trace provider for service: %s\n", serviceName)
	tp := sdktrace.NewTracerProvider(
		// Use a shorter batch timeout to ensure spans are exported quickly
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithBatchTimeout(2*time.Second),
			sdktrace.WithMaxExportBatchSize(10),
		),
		sdktrace.WithResource(res),
		// Use a simple sampler that samples all traces
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	fmt.Fprintf(os.Stderr, "OpenTelemetry trace provider initialized\n")

	return tp, nil
}

// initMeterProvider sets up an OpenTelemetry meter provider
func initMeterProvider(serviceName, exporterType, endpoint string, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	// Set up the export pipeline
	var reader sdkmetric.Reader

	exportInterval := *exportInterval // Local copy of flag value

	switch exporterType {
	case "stdout":
		// Use stdout exporter for metrics
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter for metrics: %w", err)
		}
		
		reader = sdkmetric.NewPeriodicReader(
			exporter,
			sdkmetric.WithInterval(exportInterval),
		)
		fmt.Fprintf(os.Stderr, "Using stdout exporter for metrics\n")

	case "otlp":
		// Use OTLP gRPC exporter for metrics
		fmt.Fprintf(os.Stderr, "Initializing OTLP gRPC exporter for metrics with endpoint: %s\n", endpoint)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		exporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP gRPC exporter for metrics: %w", err)
		}
		
		reader = sdkmetric.NewPeriodicReader(
			exporter, 
			sdkmetric.WithInterval(exportInterval),
		)
		fmt.Fprintf(os.Stderr, "Successfully created OTLP gRPC exporter for metrics\n")

	case "otlphttp":
		// Use OTLP HTTP exporter for metrics
		fmt.Fprintf(os.Stderr, "Initializing OTLP HTTP exporter for metrics with endpoint: %s\n", endpoint)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		exporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(endpoint),
			otlpmetrichttp.WithInsecure(),
			otlpmetrichttp.WithTimeout(5*time.Second),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP HTTP exporter for metrics: %w", err)
		}
		
		reader = sdkmetric.NewPeriodicReader(
			exporter,
			sdkmetric.WithInterval(exportInterval),
		)
		fmt.Fprintf(os.Stderr, "Successfully created OTLP HTTP exporter for metrics\n")

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}

	// Create meter provider 
	fmt.Fprintf(os.Stderr, "Creating meter provider for service: %s\n", serviceName)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	
	// Set up aggregation view to handle temporality (for now we skip temporality configuration)
	// The API has evolved and we may need to configure views differently
	
	otel.SetMeterProvider(mp)
	fmt.Fprintf(os.Stderr, "OpenTelemetry meter provider initialized\n")

	return mp, nil
}

// stdoutExporter outputs spans to stdout
type stdoutExporter struct{}

// newStdoutExporter creates a new stdout exporter
func newStdoutExporter() (sdktrace.SpanExporter, error) {
	return &stdoutExporter{}, nil
}

// ExportSpans implements the SpanExporter interface
func (e *stdoutExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, span := range spans {
		// Format and output span information
		startTime := span.StartTime().Format(time.RFC3339Nano)
		endTime := span.EndTime().Format(time.RFC3339Nano)
		duration := span.EndTime().Sub(span.StartTime()).Milliseconds()

		// Get attributes - iterate over attributes slice
		attrs := make(map[string]string)
		for _, attr := range span.Attributes() {
			attrs[string(attr.Key)] = attr.Value.Emit()
		}

		// Build JSON output
		data := map[string]interface{}{
			"trace_id":    span.SpanContext().TraceID().String(),
			"span_id":     span.SpanContext().SpanID().String(),
			"parent_id":   span.Parent().SpanID().String(),
			"name":        span.Name(),
			"start_time":  startTime,
			"end_time":    endTime,
			"duration_ms": duration,
			"attributes":  attrs,
		}

		// Output as JSON
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	}

	return nil
}

// Shutdown implements the SpanExporter interface
func (e *stdoutExporter) Shutdown(ctx context.Context) error {
	return nil
}

// Helper function to convert a trace ID to a human-readable string
func traceIDToString(traceID trace.TraceID) string {
	return fmt.Sprintf("%x", traceID[:])
}

// Helper function to convert a span ID to a human-readable string
func spanIDToString(spanID trace.SpanID) string {
	return fmt.Sprintf("%x", spanID[:])
}

// Helper function to turn a timestamp into a trace timestamp
func timeToTimestamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Microsecond)
}

// Helper function to convert string to number
func stringToNumber(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// Helper function to check if a string is in a slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if strings.TrimSpace(item) == s {
			return true
		}
	}
	return false
}