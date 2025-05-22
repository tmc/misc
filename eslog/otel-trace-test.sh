#\!/bin/bash

SERVICE_NAME="eslog-test-$(date +%s)"
echo "Testing with service name: $SERVICE_NAME"

echo "Running test with gRPC protocol..."
cat test.ndjson  < /dev/null |  ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 -service "$SERVICE_NAME" -verbose

# Wait a moment for traces to be processed
sleep 2

echo "Querying Jaeger for service: $SERVICE_NAME"
curl -s "http://localhost:16686/api/services" | jq
curl -s "http://localhost:16686/api/traces?service=$SERVICE_NAME" | jq '.data | length'

# Run with improved span names
echo "Done\!"
