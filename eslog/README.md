# eslog

A tool for processing Endpoint Security (ES) event logs, with support for filtering, templating, process tree visualization, and OpenTelemetry export.

## Features

- Process Endpoint Security JSON event logs from files or stdin
- Filter events by process ID, process name, event type, or TTY
- Format output using Go templates
- Visualize process trees with OPPID, PPID, PID, and TTY information
- Display program arguments in a readable format with length limits
- Trace the full execution tree/chain of processes
- Automatic redaction of sensitive environment variables
- Interactive TUI mode for exploring process trees
- Export to OpenTelemetry traces and metrics with best practices for observability

## Usage

```
eslog [options]
```

### Basic Options

- `-file string`: Input file (if not specified, stdin is used or positional arg)
- `-output string`: Output file (if not specified, stdout is used)
- `-F`: Follow file in real-time, processing events as they are added
- `-tui`: Start interactive TUI (Terminal User Interface) mode

### Filter Options

- `-pid int`: Filter by process ID
- `-name string`: Filter by process name (supports partial matches)
- `-event int`: Filter by event type
- `-tty string`: Filter by TTY path (e.g., 'ttys009')
- `-seq-start int`: Filter events with sequence number >= this value
- `-seq-end int`: Filter events with sequence number <= this value
- `-ppid-filter int`: Only show events within a specific PPID group

### Output Options

- `-format string`: Output format: default, json, or template (default "default")
- `-template string`: Go template for output (use with -format template)
- `-json string`: JSON mode: 'clean' for essential fields, 'raw' for full events
- `-root int`: Root PID for tree view (0 for all root processes)
- `-max-args int`: Maximum length for command arguments display (default 120)
- `-show-sensitive`: Show sensitive environment variables (disabled by default)

### Configuration

- `-config string`: Path to configuration file (defaults to ~/.eslogrc.json)
- `-dump-config`: Dump default configuration template and exit

## Examples

### Processing a file and displaying as a process tree

```bash
eslog -file events.json
```

### Filtering for a specific process

```bash
eslog -file events.json -name bash
```

### Using custom output templates

```bash
eslog -file events.json -format template -template "{{.Time}} [{{.Process.AuditToken.PID}}] {{.Event.Exec.Args}}"
```

### Trace a specific process tree

```bash
eslog -file events.json -root 1234
```

### Process logs from stdin

```bash
cat events.json | eslog -name node
```

### Filter processes by TTY

```bash
eslog -file events.json -tty ttys009
```

### Display full command line arguments

```bash
eslog -file events.json -max-args 500
```

### Filter by sequence number range

```bash
eslog -file events.json -seq-start 100 -seq-end 200
```

### Filter by parent process ID

```bash
eslog -file events.json -ppid-filter 7322
```

### Output clean JSON format

```bash
eslog -file events.json -json clean > events-clean.json
```

### Output raw JSON format

```bash
eslog -file events.json -json raw > events-full.json
```

### Generate a default configuration template

```bash
eslog -dump-config > ~/.eslogrc.json
```

### Follow log file in real-time

```bash
eslog -file events.json -F
```

### Start interactive TUI mode

```bash
eslog -file events.json -tui
```

## Configuration File

eslog supports a unified JSON configuration file with settings for all components (CLI, TUI, and OpenTelemetry export). The default location is `~/.eslogrc.json`, or you can specify a file with the `-config` flag.

### Configuration Structure

```json
{
  "default_root_pid": 0,
  "max_args_length": 120,
  "default_format": "default",
  "command_extractors": [
    {
      "pattern": "source\\s+.*\\s+&&\\s+eval\\s+'([^']+)'",
      "group": 1,
      "display_name": "EVAL:"
    }
  ],
  "tui": {
    "color_scheme": "default",
    "default_expand_level": 2,
    "show_tooltips": true
  },
  "opentelemetry": {
    "default_service_name": "eslog",
    "default_exporter": "stdout",
    "default_endpoint": "localhost:4317"
  }
}
```

See `example_config.json` for a complete configuration example.

### Command Extractors

Command extractors are used to parse and extract commands from shell scripts or command lines. Each extractor has:

- `pattern`: A regular expression pattern to match
- `group`: The capture group number (starting from 1) to extract
- `display_name`: The prefix to display before the extracted command

## OpenTelemetry Export

eslog can export event data as OpenTelemetry traces through the `eslog-to-otel` tool:

```bash
eslog -json raw -file events.json | eslog-to-otel
```

This allows visualizing process execution in tools like Jaeger, Zipkin, or any OpenTelemetry-compatible backend.

See the `tools/eslog-to-otel/README.md` for detailed OpenTelemetry export options.

## Template Fields

Some commonly used fields for templates:

- `{{.Process.Executable.Path}}`: Executable path
- `{{.Process.AuditToken.PID}}`: Process ID
- `{{.Process.PPID}}`: Parent Process ID
- `{{.Process.OriginalPPID}}`: Original Parent Process ID
- `{{.Time}}`: Event timestamp
- `{{.EventType}}`: Event type
- `{{.Event.Exec.Args}}`: Command arguments (use `{{index .Event.Exec.Args 0}}` for the program name)
- `{{.Event.Exec.CWD.Path}}`: Current working directory

## OpenTelemetry Export

ESLog includes a powerful OpenTelemetry exporter that follows best practices for observability:

### Features

- **Hierarchical trace structure**: Process relationships are preserved with parent-child spans
- **OpenTelemetry metrics**: File operations are tracked as metrics instead of span attributes
- **W3C trace context**: Automatic linking with existing traces via TRACEPARENT environment variable
- **Semantic conventions**: Uses standard OpenTelemetry attributes for processes
- **Flexible exporters**: Support for stdout, OTLP/gRPC, and OTLP/HTTP

### Usage

```bash
# Export to stdout (for debugging)
eslog -json raw -file events.json | tools/eslog-to-otel/eslog-to-otel -exporter stdout

# Export to Jaeger
eslog -json raw -file events.json | tools/eslog-to-otel/eslog-to-otel \
  -exporter otlp \
  -endpoint localhost:4317

# Export with metrics
eslog -json raw -file events.json | tools/eslog-to-otel/eslog-to-otel \
  -use-metrics \
  -temporality delta \
  -metrics-export 5s
```

### Metrics

The following metrics are exported:

- `eslog.file.lookups`: Number of file lookup operations
- `eslog.file.stats`: Number of stat operations
- `eslog.file.opens`: Number of open operations
- `eslog.file.reads`: Number of read operations
- `eslog.file.writes`: Number of write operations
- `eslog.file.bytes_read`: Total bytes read
- `eslog.file.bytes_written`: Total bytes written

Each metric includes `pid` and `executable` attributes for process identification.

### Configuration

Key flags for the OpenTelemetry exporter:

- `-exporter`: Choose exporter type (`stdout`, `otlp`, `otlphttp`)
- `-endpoint`: OpenTelemetry collector endpoint
- `-service`: Service name for traces
- `-use-metrics`: Enable metrics for file operations
- `-temporality`: Metric temporality (`delta` or `cumulative`)
- `-skip-stats`: Skip creating spans for stat events
- `-skip-lookups`: Skip creating spans for lookup events

See `tools/eslog-to-otel/README.md` for detailed documentation.

## Installation

```bash
go install github.com/tmc/misc/eslog@latest
```

Or build from source:

```bash
git clone https://github.com/tmc/misc/eslog
cd eslog
go build

# Also build the OpenTelemetry exporter
cd tools/eslog-to-otel
go build
```