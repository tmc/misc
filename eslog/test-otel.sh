#!/bin/bash

# Set up variables
SERVICE_NAME="test-eslog-$(date +%s)"
JAEGER_URL="http://localhost:16686"
OTLP_ENDPOINT="localhost:4317"
TEST_FILE="test.ndjson"

echo "Starting test with service name: $SERVICE_NAME"

# Generate some test data if needed
if [ ! -f "$TEST_FILE" ]; then
  echo "Test file not found, generating sample data..."
  ./eslog -json clean | head -n 10 > "$TEST_FILE"
fi

# Send data to Jaeger
echo "Sending data to Jaeger via OTLP..."
cat "$TEST_FILE" | ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint "$OTLP_ENDPOINT" -service "$SERVICE_NAME" -verbose

# Wait for data to be processed
echo "Waiting for Jaeger to process data..."
sleep 5

# Check if data appears in Jaeger
echo "Checking Jaeger for service: $SERVICE_NAME"
curl -s "$JAEGER_URL/api/services" | jq .

echo "Done!"