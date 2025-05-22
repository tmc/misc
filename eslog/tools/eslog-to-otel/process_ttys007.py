#!/usr/bin/env python3
import sys
import json
import subprocess
import os
import tempfile

# Check if log file path is provided
if len(sys.argv) < 2:
    print("Usage: python3 process_ttys007.py <log_file_path>")
    print("Example: python3 process_ttys007.py /tmp/eslogevents/cc-session-8.ndjson")
    sys.exit(1)

log_file = sys.argv[1]

# Check if the file exists
if not os.path.isfile(log_file):
    print(f"Error: File not found: {log_file}")
    sys.exit(1)

print(f"Processing ttys007 events from {log_file} and sending to Jaeger...")
print("Jaeger UI will be available at http://localhost:16686")

# Create a temporary file to store filtered events
temp_file = tempfile.NamedTemporaryFile(mode='w+', delete=False)
print(f"Creating temporary filtered file: {temp_file.name}")

# Change directory to the eslog directory
os.chdir('/Volumes/tmc/go/src/github.com/tmc/misc/eslog')

# Run eslog to filter by ttys007 and process each line
command = f"./eslog -file {log_file} -tty ttys007 -json raw"
process = subprocess.Popen(command, shell=True, stdout=subprocess.PIPE, universal_newlines=True)

# Counter for events with ttys007
event_count = 0

# Read process output line by line
for line in process.stdout:
    # Write non-empty lines to temp file
    line = line.strip()
    if line:
        temp_file.write(line + '\n')
        if '"time":' in line:
            event_count += 1

# Close the temp file
temp_file.close()

print(f"Found {event_count} events with ttys007")

# Process the filtered events with eslog-to-otel
print("Processing filtered events with eslog-to-otel...")
command = f"cat {temp_file.name} | ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 -service ttys007-processes -verbose"
subprocess.run(command, shell=True)

# Clean up temp file
os.unlink(temp_file.name)

print("Processing complete. Check Jaeger UI for the traces.")