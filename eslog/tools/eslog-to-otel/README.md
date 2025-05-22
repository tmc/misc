# eslog-to-otel - OpenTelemetry Exporter for eslog

This tool converts eslog event streams into OpenTelemetry traces, making it easy to visualize and analyze process execution and relationships.

## Features

- Convert eslog events to OpenTelemetry trace spans
- Visualize process trees and parent-child relationships
- Track event sequences and timing information
- Support for multiple exporters (stdout, OTLP/gRPC, OTLP/HTTP)
- Automatic linking with existing trace contexts via W3C traceparent in process environment
- Filter options to reduce trace size
- Preserve process hierarchy in the trace structure
- OpenTelemetry metrics for file operations with configurable aggregation
- Intelligent resource detection for better service identification

## Installation

```bash
# Navigate to the tool directory
cd tools/eslog-to-otel

# Build the binary
go build

# Or install globally
go install github.com/tmc/misc/eslog/tools/eslog-to-otel@latest
```

## Quick Start

Pipe eslog output to eslog-to-otel:

```bash
# Generate raw JSON events and pipe to OpenTelemetry exporter
eslog -json raw -file events.json | eslog-to-otel

# Export to an OpenTelemetry collector using gRPC
eslog -json raw -file events.json | eslog-to-otel -exporter otlp -endpoint localhost:4317

# Export to an OpenTelemetry collector using HTTP
eslog -json raw -file events.json | eslog-to-otel -exporter otlphttp -endpoint localhost:4318
```

## Usage

```
Usage of eslog-to-otel:
  -batch int
        Number of events to process before printing a status update (default 100)
  -create-root-span
        Create a root span for the session (default true)
  -endpoint string
        OpenTelemetry collector endpoint when using otlp exporter (default "localhost:4317")
  -exporter string
        OpenTelemetry exporter to use (stdout, otlp, otlphttp) (default "stdout")
  -respect-traceparent
        Respect existing TRACEPARENT env var if present (default true)
  -root-span-name string
        Name for the root span (default "eslog-session")
  -service string
        Service name to use in OpenTelemetry traces (default "eslog")
  -skip-lookups
        Skip creating spans for lookup events (reduces trace size) (default true)
  -skip-stats
        Skip creating spans for stat events (reduces trace size) (default true)
  -verbose
        Enable verbose output

  # Metrics-specific options
  -use-metrics
        Use metrics for file operation counts instead of span attributes (default true)
  -aggregate-io
        Aggregate file I/O as attributes instead of spans (default true)
  -temporality string
        Aggregation temporality for metrics (delta or cumulative) (default "delta")
  -metrics-export duration
        Metrics export interval (default 5s)
```

## Visualization Options

Once you've exported traces, you can visualize them using:

1. **Jaeger UI**: Connect to a Jaeger instance to explore trace hierarchies.
2. **Zipkin**: View traces in Zipkin for sequence analysis.
3. **Grafana Tempo**: Visualize and correlate traces with metrics and logs.
4. **OpenTelemetry Collector**: Forward to any compatible backend.

## W3C Trace Context Support

The tool automatically detects W3C Trace Context in process environment variables. When a process has `TRACEPARENT`, `HTTP_TRACEPARENT`, or `OTEL_TRACEPARENT` environment variables, the tool will:

1. Extract the trace ID and span ID from the traceparent value
2. Add them as span attributes for linking in visualization tools
3. Create logical links between the current trace and the external trace

This allows connecting process execution traces with traces from other systems that use OpenTelemetry or W3C Trace Context propagation.

## Example: Tracing Process Execution

```bash
# Track a command's execution with OpenTelemetry
eslog -json raw -file <(command_to_trace) | eslog-to-otel

# Send to Jaeger
eslog -json raw -file events.json | eslog-to-otel -exporter otlp -endpoint jaeger:4317
```

## Integration with Other Tools

### Using with Jaeger

1. Start Jaeger:
```bash
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
  -p 5775:5775/udp \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 14250:14250 \
  -p 9411:9411 \
  jaegertracing/all-in-one:1.29
```

2. Export traces to Jaeger:
```bash
eslog -json raw -file events.json | eslog-to-otel -exporter otlp -endpoint localhost:14250
```

3. View traces at http://localhost:16686

### Using with OpenTelemetry Collector

1. Configure a collector (otel-collector-config.yaml):
```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger]
```

2. Start the collector:
```bash
docker run -d --name otel-collector \
  -p 4317:4317 -p 4318:4318 \
  -v $(pwd)/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
  otel/opentelemetry-collector:0.59.0 \
  --config=/etc/otel-collector-config.yaml
```

3. Export traces to the collector:
```bash
eslog -json raw -file events.json | eslog-to-otel -exporter otlp -endpoint localhost:4317
```

## Tips for Effective Tracing

1. **Reduce Noise**: Use `-skip-stats` and `-skip-lookups` to reduce trace size.
2. **Filter Input**: Use eslog filters to focus on processes of interest before piping to eslog-to-otel.
3. **Custom Service Name**: Set `-service` to something meaningful for your application.
4. **Trace Context**: If your application sets TRACEPARENT environment variables, eslog-to-otel will link your trace with the application trace.

## Advanced Usage

### Creating a Session Root Span

By default, eslog-to-otel creates a root span for the entire trace session. This helps organize all process spans under a common parent:

```bash
# Customize the root span name
eslog -json raw -file events.json | eslog-to-otel -root-span-name "deployment-session"
```

### Using OpenTelemetry Metrics

The tool can export file operation counts as proper OpenTelemetry metrics instead of span attributes, following best practices:

```bash
# Export metrics with delta temporality (for rate calculations)
eslog -json raw -file events.json | eslog-to-otel -use-metrics -temporality delta

# Export metrics with cumulative temporality (for total counts)
eslog -json raw -file events.json | eslog-to-otel -use-metrics -temporality cumulative

# Configure the metrics export interval
eslog -json raw -file events.json | eslog-to-otel -use-metrics -metrics-export 10s
```

The following metrics are available:

- `eslog.file.lookups`: Count of file lookup operations
- `eslog.file.stats`: Count of file stat operations
- `eslog.file.readlinks`: Count of readlink operations
- `eslog.file.accesses`: Count of file access operations
- `eslog.file.opens`: Count of file open operations
- `eslog.file.closes`: Count of file close operations
- `eslog.file.reads`: Count of file read operations
- `eslog.file.writes`: Count of file write operations
- `eslog.file.bytes_read`: Total bytes read from files (in bytes)
- `eslog.file.bytes_written`: Total bytes written to files (in bytes)

Each metric includes attributes for `pid` and `executable` to identify the process.

### Tracing Specific Processes

To focus on specific processes:

```bash
# Trace only node processes
eslog -json raw -name node -file events.json | eslog-to-otel
```

### Continuous Monitoring

For real-time monitoring:

```bash
# Follow file and export to OpenTelemetry
eslog -json raw -F -file events.json | eslog-to-otel -exporter otlp
```

## Troubleshooting

- **High Volume**: For high-volume event processing, increase the batch size with `-batch 1000`
- **Memory Issues**: Use eslog's filtering options to reduce input volume before piping to eslog-to-otel
- **Connectivity Problems**: Use `-verbose` to debug connection issues with collectors
- **Missing Spans**: Ensure you're using `-json raw` with eslog to include all event details