#!/bin/bash
# usage_example.sh - Example script demonstrating the eslog-to-otel workflow

set -e  # Exit on any error

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Print colored section headers
section() {
  echo -e "${BOLD}${BLUE}=== $1 ===${NC}"
}

# Print command before executing
execute() {
  echo -e "${YELLOW}$ $1${NC}"
  eval "$1"
  echo ""
}

# Check if eslog is available
if ! command -v eslog &> /dev/null; then
  echo "Error: eslog is not installed or not in PATH"
  echo "Please build and install eslog first"
  exit 1
fi

# Ensure we have the eslog-to-otel binary
if [ ! -f ./eslog-to-otel ]; then
  section "Building eslog-to-otel"
  execute "go build"
fi

# Create a sample data file if it doesn't exist
SAMPLE_FILE="/tmp/sample_eslog_data.json"
if [ ! -f "$SAMPLE_FILE" ] || [ $(wc -l < "$SAMPLE_FILE") -lt 10 ]; then
  section "Generating sample data"
  echo "Running a sample command to capture endpoint security events..."
  
  # Capture some ES events by running ls and other commands
  (
    # Start es_event capture in background
    sudo log stream --predicate 'subsystem == "com.apple.EndpointSecurity"' --style json > "$SAMPLE_FILE" &
    LOGGER_PID=$!
    
    # Give it a moment to start capturing
    sleep 1
    
    # Run some commands to generate events
    ls -la
    find . -type f -name "*.go" | head -n 3
    ps aux | grep eslog
    
    # Let it capture for a moment
    sleep 2
    
    # Stop the logger
    kill $LOGGER_PID
  )
  
  echo "Sample data saved to $SAMPLE_FILE"
fi

section "Basic eslog-to-otel Usage"
echo "The following examples demonstrate how to use eslog with OpenTelemetry:"
echo

# Basic example with stdout
echo -e "${BOLD}1. Basic Usage - Export to stdout${NC}"
echo "This pipes eslog output to eslog-to-otel, which formats traces as JSON to stdout:"
echo -e "${YELLOW}$ eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel | head -n 20${NC}"
echo "# (not executing to avoid overwhelming output)"
echo

# Example with Jaeger
echo -e "${BOLD}2. Export to Jaeger${NC}"
echo "If you have Jaeger running, you can export traces directly:"
echo -e "${YELLOW}$ eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel -exporter otlp -endpoint localhost:4317${NC}"
echo "# (not executing as Jaeger may not be running)"
echo

# Example with filtering
echo -e "${BOLD}3. Filtering Before Export${NC}"
echo "Filter events before exporting to reduce trace size:"
execute "eslog -name bash -json raw -file $SAMPLE_FILE | ./eslog-to-otel -verbose | head -n 10"

# Example with custom service name
echo -e "${BOLD}4. Custom Service Name${NC}"
echo "Set a custom service name for better organization in trace visualizers:"
execute "eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel -service 'my-application' -verbose | head -n 5"

section "Advanced Usage"

# Example with reducing span count
echo -e "${BOLD}5. Reducing Trace Size${NC}"
echo "Skip common high-volume events to focus on important processes:"
execute "eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel -skip-stats=true -skip-lookups=true -verbose | head -n 5"

# Example with metrics
echo -e "${BOLD}6. Using Metrics Instead of Span Attributes${NC}"
echo "Export file operations as OpenTelemetry metrics with delta temporality:"
execute "eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel -use-metrics=true -temporality=delta -metrics-export=3s -verbose | head -n 5"

# Example with cumulative metrics
echo -e "${BOLD}7. Using Cumulative Metrics${NC}"
echo "Export file operations as cumulative metrics for aggregation:"
execute "eslog -json raw -file $SAMPLE_FILE | ./eslog-to-otel -use-metrics=true -temporality=cumulative -verbose | head -n 5"

# Example with W3C trace context
echo -e "${BOLD}8. W3C Trace Context${NC}"
echo "Create a process with traceparent to demonstrate linking:"

# Generate a random trace ID (16 bytes/32 hex chars)
TRACE_ID=$(openssl rand -hex 16)
# Generate a random span ID (8 bytes/16 hex chars)
SPAN_ID=$(openssl rand -hex 8)
# Create W3C traceparent
TRACEPARENT="00-${TRACE_ID}-${SPAN_ID}-01"

echo -e "Generated traceparent: ${GREEN}$TRACEPARENT${NC}"

# Create a temporary script with the traceparent in its environment
cat > /tmp/traced_process.sh << EOF
#!/bin/bash
# This script has W3C trace context in its environment
echo "Running with traceparent: \$TRACEPARENT"
# Do something
ls -la
echo "Done!"
EOF
chmod +x /tmp/traced_process.sh

echo -e "${YELLOW}$ TRACEPARENT=$TRACEPARENT /tmp/traced_process.sh${NC}"
echo "# (command not executed - run manually to test trace linking)"
echo

# Output formats
section "Supported Output Formats"
echo -e "${BOLD}1. JSON to stdout${NC} (default)"
echo -e "${BOLD}2. OTLP/gRPC${NC} (-exporter otlp)"
echo -e "${BOLD}3. OTLP/HTTP${NC} (-exporter otlphttp)"
echo

section "Next Steps"
echo "To visualize traces:"
echo "1. Start Jaeger: docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest"
echo "2. Run: eslog -json raw -file YOUR_LOG_FILE | ./eslog-to-otel -exporter otlp"
echo "3. Open Jaeger UI at http://localhost:16686"
echo
echo -e "${GREEN}See README.md for more detailed usage examples and options${NC}"