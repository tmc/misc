#!/bin/bash

SERVICE_NAME="eslog-with-options-$(date +%s)"
echo "Testing with service name: $SERVICE_NAME"

echo "Running test with full trace (including stat and lookup events)..."
cat test.ndjson | ./tools/eslog-to-otel/eslog-to-otel \
  -exporter=otlp \
  -endpoint=localhost:4317 \
  -service="$SERVICE_NAME-full" \
  -skip-stats=false \
  -skip-lookups=false \
  -verbose

echo "Running test with reduced trace (skipping stat and lookup events)..."
cat test.ndjson | ./tools/eslog-to-otel/eslog-to-otel \
  -exporter=otlp \
  -endpoint=localhost:4317 \
  -service="$SERVICE_NAME-reduced" \
  -skip-stats=true \
  -skip-lookups=true \
  -verbose

# Wait a moment for traces to be processed
sleep 2

echo "Done! Check Jaeger UI for comparison of traces"