#!/bin/bash

echo "Starting Jaeger connection validation..."

# Check if Jaeger is running
echo "Checking if Jaeger container is running..."
if ! docker ps | grep -q jaeger; then
  echo "Jaeger container not found. Starting one..."
  docker run -d --name jaeger \
    -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
    -e COLLECTOR_OTLP_ENABLED=true \
    -p 16686:16686 \
    -p 4317:4317 \
    -p 4318:4318 \
    -p 9411:9411 \
    jaegertracing/all-in-one:latest
  
  echo "Waiting for Jaeger to start..."
  sleep 5
fi

# Verify ports are accessible
echo "Verifying Jaeger ports are accessible..."
for port in 16686 4317 4318 9411; do
  if nc -z localhost $port; then
    echo "Port $port is open"
  else
    echo "Port $port is closed"
  fi
done

# Generate simple test span
echo "Generating test span..."
cat <<EOF > test_span.json
{
  "resource": {
    "attributes": {
      "service.name": "test-service"
    }
  },
  "scopeSpans": [
    {
      "scope": {
        "name": "test-scope"
      },
      "spans": [
        {
          "traceId": "5b8aa5a2d2c872e8321cf37308d69df2",
          "spanId": "5fb397be34d25d33",
          "name": "test-span",
          "kind": 1,
          "startTimeUnixNano": "1619095213000000000",
          "endTimeUnixNano": "1619095213100000000",
          "attributes": [
            {
              "key": "test.key",
              "value": {
                "stringValue": "test-value"
              }
            }
          ]
        }
      ]
    }
  ]
}
EOF

# Try sending to Jaeger using OTLP HTTP
echo "Sending test span to Jaeger via OTLP HTTP..."
curl -v -X POST -H "Content-Type: application/json" \
  --data @test_span.json \
  http://localhost:4318/v1/traces

# Try sending data with a simple tool
echo -e "\nTesting with eslog-to-otel:"
cd /Volumes/tmc/go/src/github.com/tmc/misc/eslog
cat test.ndjson | ./tools/eslog-to-otel/eslog-to-otel -exporter otlphttp -endpoint "localhost:4318/v1/traces" -service test-validation-service -verbose

# Check services again
echo -e "\nChecking for services in Jaeger..."
curl -s "http://localhost:16686/api/services" | jq

# Check for traces
echo -e "\nChecking for traces in Jaeger..."
curl -s "http://localhost:16686/api/traces?service=test-validation-service&limit=1" | jq

echo "Validation complete!"