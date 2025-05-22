#!/bin/bash

# ESLog Analyzer - A tool for analyzing macOS endpoint security logs
#
# This script provides various queries to analyze the NDJSON logs from
# macOS endpoint security framework

DB_FILE="cc_session.duckdb"
NDJSON_FILE="cc-session.ndjson"

# Function to print usage
usage() {
  echo "ESLog Analyzer - A tool for analyzing macOS endpoint security logs"
  echo ""
  echo "Usage: $0 [command]"
  echo ""
  echo "Commands:"
  echo "  load             - Load the NDJSON file into DuckDB"
  echo "  summary          - Show a summary of the database"
  echo "  processes        - List all processes captured in the log"
  echo "  file-access      - Show file access patterns"
  echo "  network          - Show network connections"
  echo "  process-tree     - Show process hierarchy"
  echo "  longest-running  - Show processes with longest runtime"
  echo "  events [type]    - Show events of specified type (number)"
  echo "  claude           - Show Claude-related activities"
  echo "  query [sql]      - Run a custom SQL query"
  echo ""
  echo "Examples:"
  echo "  $0 load"
  echo "  $0 summary"
  echo "  $0 events 9"
  echo "  $0 claude"
  echo "  $0 longest-running"
  echo "  $0 query \"SELECT * FROM cc_session LIMIT 5\""
  exit 1
}

# Function to load data
load_data() {
  if [ ! -f "$NDJSON_FILE" ]; then
    echo "Error: NDJSON file '$NDJSON_FILE' not found!"
    exit 1
  fi

  echo "Loading $NDJSON_FILE into DuckDB..."
  
  # Create SQL for loading data
  SQL_COMMANDS=$(cat << EOF
DROP TABLE IF EXISTS cc_session;
CREATE TABLE cc_session AS 
SELECT * FROM read_ndjson_auto('$NDJSON_FILE');
SELECT COUNT(*) FROM cc_session;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
  echo "Done! Database saved to $DB_FILE"
}

# Function to show summary
show_summary() {
  echo "ESLog Summary"
  echo "============="

  # Create SQL for summary
  SQL_COMMANDS=$(cat << EOF
-- Total number of events
SELECT COUNT(*) as total_events FROM cc_session;

-- Events by type
WITH event_types AS (
  SELECT 
    event_type,
    CASE event_type
      WHEN 9 THEN 'exec'
      WHEN 10 THEN 'fork'
      WHEN 11 THEN 'exit'
      WHEN 12 THEN 'close'
      WHEN 21 THEN 'mprotect'
      WHEN 33 THEN 'write'
      WHEN 39 THEN 'open'
      WHEN 43 THEN 'mmap'
      WHEN 54 THEN 'ioctl'
      WHEN 55 THEN 'access'
      WHEN 62 THEN 'stat'
      WHEN 68 THEN 'getpath'
      ELSE 'other'
    END as event_name,
    COUNT(*) as count
  FROM cc_session
  GROUP BY event_type
  ORDER BY count DESC
)
SELECT * FROM event_types;

-- Time range
SELECT 
  MIN(time) as start_time,
  MAX(time) as end_time
FROM cc_session;

-- Top 10 executables by count
SELECT 
  process.executable.path as executable_path, 
  COUNT(*) as event_count 
FROM cc_session 
WHERE process.executable.path IS NOT NULL
GROUP BY executable_path 
ORDER BY event_count DESC 
LIMIT 10;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to list processes
list_processes() {
  echo "Process Executions"
  echo "================="

  # Create SQL for process listing
  SQL_COMMANDS=$(cat << EOF
-- Process executions with their arguments
WITH process_execs AS (
  SELECT
    time,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as target_path,
    json_extract(event, '$.exec.args') as args
  FROM cc_session
  WHERE event_type = 9  -- exec events
  ORDER BY time
)
SELECT * FROM process_execs;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to show file access
show_file_access() {
  echo "File Access Patterns"
  echo "==================="

  # Create SQL for file access
  SQL_COMMANDS=$(cat << EOF
-- File access events (open, close, write)
SELECT 
  time,
  process.audit_token.pid as pid,
  process.executable.path as process_path,
  event_type,
  CASE event_type
    WHEN 39 THEN 'open'
    WHEN 12 THEN 'close'
    WHEN 33 THEN 'write'
    ELSE 'other'
  END as event_name,
  json_extract(event, '$.open.target.path') as open_path,
  json_extract(event, '$.close.target.path') as close_path,
  json_extract(event, '$.write.target.path') as write_path
FROM cc_session
WHERE event_type IN (39, 12, 33)
  AND (
    json_extract(event, '$.open.target.path') IS NOT NULL OR
    json_extract(event, '$.close.target.path') IS NOT NULL OR
    json_extract(event, '$.write.target.path') IS NOT NULL
  )
ORDER BY time
LIMIT 20;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to show network connections
show_network() {
  echo "Network Connections"
  echo "=================="

  # Create SQL for network connections
  SQL_COMMANDS=$(cat << EOF
-- Network-related events (socket, connect, bind)
SELECT 
  time,
  process.audit_token.pid as pid,
  process.executable.path as process_path,
  event_type,
  event  -- Show raw event data for now
FROM cc_session
WHERE (
  event_type = 97 OR  -- socket
  event_type = 98 OR  -- connect
  event_type = 99     -- bind
)
ORDER BY time
LIMIT 20;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to show process tree
show_process_tree() {
  echo "Process Hierarchy"
  echo "================="

  # Create SQL for process tree
  SQL_COMMANDS=$(cat << EOF
-- Process hierarchy (fork/exec events)
WITH process_events AS (
  SELECT
    time,
    event_type,
    CASE event_type
      WHEN 9 THEN 'exec'
      WHEN 10 THEN 'fork'
      ELSE 'other'
    END as event_name,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as exec_target,
    json_extract(event, '$.fork.child.executable.path') as fork_child_path,
    json_extract(event, '$.fork.child.audit_token.pid') as fork_child_pid
  FROM cc_session
  WHERE event_type IN (9, 10)
  ORDER BY time
)
SELECT * FROM process_events
LIMIT 20;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to show events by type
show_events_by_type() {
  if [ -z "$1" ]; then
    echo "Error: Event type is required!"
    usage
  fi

  EVENT_TYPE=$1
  echo "Events of Type $EVENT_TYPE"
  echo "======================="

  # Create SQL for events by type
  SQL_COMMANDS=$(cat << EOF
-- Events of specified type
SELECT 
  time,
  process.audit_token.pid as pid,
  process.executable.path as process_path,
  event
FROM cc_session
WHERE event_type = $EVENT_TYPE
ORDER BY time
LIMIT 20;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to run custom query
run_custom_query() {
  if [ -z "$1" ]; then
    echo "Error: SQL query is required!"
    usage
  fi

  QUERY="$1"
  echo "Running Custom Query"
  echo "==================="
  echo "$QUERY" | duckdb "$DB_FILE"
}

# Function to show Claude-related activities
show_claude_activities() {
  echo "Claude-Related Activities"
  echo "========================"

  # Create SQL for Claude-related activities
  SQL_COMMANDS=$(cat << EOF
-- Find all Claude-related processes and their arguments
WITH claude_execs AS (
  SELECT
    time,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as target_path,
    json_extract(event, '$.exec.args') as args
  FROM cc_session
  WHERE event_type = 9  -- exec events
    AND (
      LOWER(process.executable.path) LIKE '%claude%' OR
      LOWER(json_extract(event, '$.exec.target.executable.path')::VARCHAR) LIKE '%claude%' OR
      CAST(json_extract(event, '$.exec.args') AS VARCHAR) LIKE '%claude%'
    )
  ORDER BY time
)
SELECT * FROM claude_execs;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Function to show longest running processes
show_longest_running() {
  echo "Longest Running Processes"
  echo "========================="

  # Create SQL for longest running processes
  SQL_COMMANDS=$(cat << EOF
-- Find the longest running processes in the log
WITH process_lifetimes AS (
  SELECT
    process.audit_token.pid as pid,
    process.executable.path as process_path,
    MIN(time) as first_seen_time,
    MAX(time) as last_seen_time,
    -- Calculate duration in seconds
    (EXTRACT(EPOCH FROM CAST(MAX(time) AS TIMESTAMP)) -
     EXTRACT(EPOCH FROM CAST(MIN(time) AS TIMESTAMP))) as duration_seconds
  FROM cc_session
  WHERE process.executable.path IS NOT NULL
    AND process.audit_token.pid IS NOT NULL
  GROUP BY pid, process_path
  HAVING COUNT(*) > 1  -- Ensure we have at least two events to calculate duration
)
SELECT
  pid,
  process_path,
  first_seen_time,
  last_seen_time,
  duration_seconds,
  -- Format duration in a human-readable way
  CASE
    WHEN duration_seconds >= 3600 THEN
      FLOOR(duration_seconds / 3600) || 'h ' ||
      FLOOR((duration_seconds % 3600) / 60) || 'm ' ||
      FLOOR(duration_seconds % 60) || 's'
    WHEN duration_seconds >= 60 THEN
      FLOOR(duration_seconds / 60) || 'm ' ||
      FLOOR(duration_seconds % 60) || 's'
    ELSE
      FLOOR(duration_seconds) || 's'
  END as duration_formatted
FROM process_lifetimes
ORDER BY duration_seconds DESC
LIMIT 20;
EOF
)

  # Execute the SQL commands
  echo "$SQL_COMMANDS" | duckdb "$DB_FILE"
}

# Main logic
if [ $# -eq 0 ]; then
  usage
fi

case "$1" in
  load)
    load_data
    ;;
  summary)
    show_summary
    ;;
  processes)
    list_processes
    ;;
  file-access)
    show_file_access
    ;;
  network)
    show_network
    ;;
  process-tree)
    show_process_tree
    ;;
  longest-running)
    cd /Volumes/tmc/go/src/github.com/tmc/misc/eslog/testdata && ./process_activity_span.sh
    ;;
  events)
    show_events_by_type "$2"
    ;;
  claude)
    show_claude_activities
    ;;
  query)
    run_custom_query "$2"
    ;;
  *)
    usage
    ;;
esac