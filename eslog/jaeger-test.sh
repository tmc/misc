#!/bin/bash

# Script to test eslog-to-otel with Jaeger
set -e

# Display welcome message
echo "=== Testing eslog-to-otel with Jaeger ==="
echo "This script will verify the connection to Jaeger and test both HTTP and gRPC protocols."

# Check if Jaeger is running
echo -e "\n> Checking if Jaeger is running..."
if ! docker ps | grep -q jaeger; then
  echo "Error: Jaeger container not found. Please start Jaeger with:"
  echo "docker run -d --name jaeger -e COLLECTOR_OTLP_ENABLED=true -p 16686:16686 -p 4317:4317 -p 4318:4318 jaegertracing/all-in-one:latest"
  exit 1
fi

# Verify that ports are accessible
echo -e "\n> Verifying Jaeger ports..."
for port in 16686 4317 4318; do
  if nc -z localhost $port; then
    echo "✓ Port $port is open"
  else
    echo "✗ Port $port is closed"
    exit 1
  fi
done

# Test with both protocols
PROTOCOLS=("grpc" "http")
TIMESTAMP=$(date +%s)

for protocol in "${PROTOCOLS[@]}"; do
  echo -e "\n> Testing with $protocol protocol..."
  
  # Define service name and endpoint based on protocol
  if [ "$protocol" == "grpc" ]; then
    SERVICE_NAME="eslog-test-grpc-$TIMESTAMP"
    EXPORTER="otlp"
    ENDPOINT="localhost:4317"
  else
    SERVICE_NAME="eslog-test-http-$TIMESTAMP"
    EXPORTER="otlphttp"
    ENDPOINT="localhost:4318"
  fi
  
  echo "  Service name: $SERVICE_NAME"
  echo "  Endpoint: $ENDPOINT"
  
  # Run the test
  echo "  Sending test data..."
  cat test.ndjson | ./tools/eslog-to-otel/eslog-to-otel -exporter $EXPORTER -endpoint $ENDPOINT -service $SERVICE_NAME -verbose
  
  # Wait for data to be processed
  echo "  Waiting for data to be processed..."
  sleep 2
  
  # Check if service appears in Jaeger
  echo "  Checking if service appears in Jaeger..."
  SERVICES=$(curl -s "http://localhost:16686/api/services" | jq -r '.data[]')
  
  if echo "$SERVICES" | grep -q "$SERVICE_NAME"; then
    echo "  ✓ Service $SERVICE_NAME found in Jaeger"
    
    # Check if traces appear
    echo "  Checking for traces..."
    TRACES=$(curl -s "http://localhost:16686/api/traces?service=$SERVICE_NAME&limit=1")
    if [ "$(echo "$TRACES" | jq '.data | length')" -gt 0 ]; then
      echo "  ✓ Traces found for $SERVICE_NAME"
      echo "  Protocol $protocol works successfully!"
    else
      echo "  ✗ No traces found for $SERVICE_NAME"
      echo "  Protocol $protocol is sending to Jaeger but no traces are being stored"
    fi
  else
    echo "  ✗ Service $SERVICE_NAME not found in Jaeger"
    echo "  Protocol $protocol failed to send data to Jaeger"
  fi
done

echo -e "\n=== Test Summary ==="
echo "Services in Jaeger:"
curl -s "http://localhost:16686/api/services" | jq

echo -e "\nTest completed. Check the Jaeger UI at http://localhost:16686 for detailed results."