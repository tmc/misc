# Using eslog-to-otel with Jaeger

This guide shows how to export ESLog traces and metrics to Jaeger for visualization.

## Quick Start

1. **Start Jaeger** using Docker:

```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

2. **Export traces** from ESLog:

```bash
# Export a file
eslog -json raw -file your_events.json | eslog-to-otel -exporter otlp -endpoint localhost:4317

# Export from live stream
eslog -json raw -follow | eslog-to-otel -exporter otlp -endpoint localhost:4317
```

3. **View traces** in Jaeger UI:
   - Open http://localhost:16686
   - Select service "eslog" from the dropdown
   - Click "Find Traces"

## Features Demonstrated

### Process Hierarchy
- Parent-child relationships are preserved in the trace view
- Each process appears as a span with its children nested underneath

### File Operations Metrics
With `-use-metrics` flag, file operations are tracked as metrics:
- `eslog.file.lookups`: Number of lookup operations
- `eslog.file.stats`: Number of stat operations
- `eslog.file.opens`: Number of open operations
- `eslog.file.reads`: Number of read operations
- `eslog.file.bytes_read`: Total bytes read
- `eslog.file.writes`: Number of write operations
- `eslog.file.bytes_written`: Total bytes written

### W3C Trace Context
When processes have `TRACEPARENT` environment variable, they're linked to external traces:
```bash
TRACEPARENT=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01 your_command
```

## Configuration Options

```bash
# Full example with all options
eslog -json raw -file events.json | eslog-to-otel \
  -exporter otlp \
  -endpoint localhost:4317 \
  -service "my-app" \
  -use-metrics \
  -temporality delta \
  -metrics-export 5s \
  -verbose
```

### Key Options:
- `-exporter`: Choose between `stdout`, `otlp`, or `otlphttp`
- `-endpoint`: Jaeger collector endpoint (default: localhost:4317)
- `-service`: Service name shown in Jaeger (default: eslog)
- `-use-metrics`: Enable metrics for file operations
- `-temporality`: Choose `delta` or `cumulative` for metrics
- `-skip-stats`: Skip creating spans for stat events
- `-skip-lookups`: Skip creating spans for lookup events

## Docker Compose Setup

For production use, create a `docker-compose.yaml`:

```yaml
version: '3.8'

services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

Then run:
```bash
docker-compose up -d
```

## Troubleshooting

1. **Connection errors**: Ensure Jaeger is running and the endpoint is correct
2. **Missing traces**: Check the service name filter in Jaeger UI
3. **No metrics**: Jaeger all-in-one doesn't support metrics; use a full observability stack
4. **High volume**: Use filtering options (`-name`, `-pid`) to reduce data

## Example Workflow

```bash
# 1. Start Jaeger
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest

# 2. Capture some events
eslog -json raw -duration 10s > events.json

# 3. Export to Jaeger
cat events.json | eslog-to-otel -exporter otlp -use-metrics -verbose

# 4. View in Jaeger
open http://localhost:16686
```

## Advanced: Metrics with Prometheus

For full metrics support, use the OpenTelemetry Collector with Prometheus:

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
  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```