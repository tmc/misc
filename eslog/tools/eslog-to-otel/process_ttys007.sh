#!/bin/bash
# Process only ttys007 events from a log file and send to Jaeger

# Check if log file path is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <log_file_path>"
  echo "Example: $0 /tmp/eslogevents/cc-session-8.ndjson"
  exit 1
fi

LOG_FILE="$1"

# Check if the file exists
if [ ! -f "$LOG_FILE" ]; then
  echo "Error: File not found: $LOG_FILE"
  exit 1
fi

echo "Processing ttys007 events from $LOG_FILE and sending to Jaeger..."
echo "Jaeger UI will be available at http://localhost:16686"

# Create a temporary file to store filtered events in raw JSON format
TEMP_FILE=$(mktemp)
echo "Creating temporary filtered file: $TEMP_FILE"

# Filter for ttys007 events and save raw JSON
cd /Volumes/tmc/go/src/github.com/tmc/misc/eslog
./eslog -file "$LOG_FILE" -tty ttys007 -json raw > $TEMP_FILE

# Count the number of events
EVENT_COUNT=$(grep -c '"time":' $TEMP_FILE)
echo "Found $EVENT_COUNT events with ttys007"

# Process with eslog-to-otel
echo "Processing filtered events with eslog-to-otel..."
cat $TEMP_FILE | ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 -service "ttys007-processes" -verbose

# Clean up temp file
rm $TEMP_FILE

echo "Processing complete. Check Jaeger UI for the traces."