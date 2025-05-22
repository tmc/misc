# ESLog Analyzer

This project provides tools for analyzing macOS Endpoint Security (ES) logs in NDJSON format using DuckDB.

## Overview

The macOS Endpoint Security framework produces detailed logs of system events such as process executions, file operations, and network activity. These logs are valuable for security analysis, debugging, and understanding application behavior. The tools in this project make it easy to load these logs into DuckDB and analyze them with SQL queries.

## Files

- `cc-session.ndjson` - An example ES log file containing system events
- `eslog_analyzer.sh` - Main script for analyzing ES logs
- `load_to_duckdb.sh` - Script for loading NDJSON files into DuckDB
- `analyze_data.sh` - Script with sample analysis queries
- `explore_events.sh` - Script for exploring specific event types
- `claude_related.sql` - SQL queries focused on Claude-related activities
- `query_processes.sql` - SQL queries for analyzing process executions

## Prerequisites

- DuckDB (installed via `brew install duckdb`)
- Bash

## Usage

The main script `eslog_analyzer.sh` provides several commands for analyzing ES logs:

```bash
# Load data into DuckDB
./eslog_analyzer.sh load

# Show a summary of the data
./eslog_analyzer.sh summary

# List all processes executed
./eslog_analyzer.sh processes

# Show file access patterns
./eslog_analyzer.sh file-access

# Show network connections
./eslog_analyzer.sh network

# Show process hierarchy
./eslog_analyzer.sh process-tree

# Show events of a specific type (e.g., exec events with type 9)
./eslog_analyzer.sh events 9

# Show Claude-related activities
./eslog_analyzer.sh claude

# Run a custom SQL query
./eslog_analyzer.sh query "SELECT * FROM cc_session LIMIT 5"
```

## Event Types

Common event types in ES logs include:

- 9: exec (process execution)
- 10: fork
- 11: exit
- 12: close
- 21: mprotect
- 33: write
- 39: open
- 43: mmap
- 54: ioctl
- 55: access
- 62: stat
- 68: getpath

## Examples

Here are some example queries you can run:

### Find all process executions with their arguments

```sql
SELECT
  time,
  process.audit_token.pid as pid,
  process.executable.path as process_path,
  json_extract(event, '$.exec.target.executable.path') as target_path,
  json_extract(event, '$.exec.args') as args
FROM cc_session
WHERE event_type = 9  -- exec events
ORDER BY time;
```

### Find file accesses by a specific process

```sql
SELECT 
  time,
  process.audit_token.pid as pid,
  json_extract(event, '$.open.target.path') as file_path
FROM cc_session
WHERE event_type = 39  -- open events
  AND process.executable.path = '/path/to/executable'
ORDER BY time;
```

## Custom Analysis

You can write your own SQL queries for specific analysis needs. The schema of the `cc_session` table is based on the structure of the NDJSON file, with top-level fields like `time`, `event_type`, `process`, and `event`.

For complex event data stored in the `event` field, you can use the `json_extract` function to access nested JSON values.

## License

This project is open source and available under the MIT License.