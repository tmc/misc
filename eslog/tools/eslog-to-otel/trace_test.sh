#!/bin/bash
# Simple script to test W3C traceparent detection in eslog-to-otel

# Generate random trace ID and span ID
TRACE_ID=$(openssl rand -hex 16)
SPAN_ID=$(openssl rand -hex 8)

# Create a valid W3C traceparent
# Format: version-traceId-spanId-flags
TRACEPARENT="00-${TRACE_ID}-${SPAN_ID}-01"

echo "Generated traceparent: $TRACEPARENT"

# Export the traceparent environment variable
export TRACEPARENT

# Run a simple command that will be captured by eslog
echo "Running traced command..."
sleep 2
echo "Traced command completed"

# The traceparent should be detected by eslog-to-otel when this process is captured