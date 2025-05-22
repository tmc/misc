#!/bin/bash
# Demo script to run eslog-to-otel with W3C trace context detection

echo "Starting eslog and eslog-to-otel with trace context detection..."
echo "1. Make sure Jaeger is running: docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/jaeger:2.6.0"
echo "2. In another terminal, run ./trace_test.sh"
echo "3. Check Jaeger UI at http://localhost:16686 for the traces"
echo ""

echo "Waiting for events to process..."
cd /Volumes/tmc/go/src/github.com/tmc/misc/eslog
./eslog -json clean | ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 -service eslog-trace-demo -verbose