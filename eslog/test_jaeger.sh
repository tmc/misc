#!/bin/bash
# Test script for eslog-to-otel with Jaeger

echo "Testing eslog-to-otel with sample data..."

# First, test with stdout to see if it works
echo "=== Testing stdout export ==="
./tools/eslog-to-otel/eslog-to-otel -exporter stdout -verbose -use-metrics < test.ndjson | head -20

echo ""
echo "=== Testing OTLP export to non-existent Jaeger (should fail gracefully) ==="
./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 -verbose -use-metrics < test.ndjson

echo ""
echo "=== Testing metrics export with delta temporality ==="
./tools/eslog-to-otel/eslog-to-otel -exporter stdout -verbose -use-metrics -temporality delta -metrics-export 2s < test.ndjson | head -40

echo ""
echo "If Jaeger is running on localhost:4317, traces and metrics should be visible at http://localhost:16686"