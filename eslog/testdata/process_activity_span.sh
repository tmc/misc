#!/bin/bash

# A script to analyze process activity spans in ESLog data

DB_FILE="cc_session.duckdb"

echo "Process Activity Spans"
echo "====================="

SQL_COMMANDS=$(cat << EOF
-- Analyze process activity spans
-- This looks at the first and last appearance of each process
-- in the logs, regardless of event type

WITH process_spans AS (
  SELECT
    process.audit_token.pid as pid,
    process.executable.path as executable_path,
    MIN(time) as first_seen,
    MAX(time) as last_seen,
    -- Calculate duration in seconds
    (EXTRACT(EPOCH FROM CAST(MAX(time) AS TIMESTAMP)) - 
     EXTRACT(EPOCH FROM CAST(MIN(time) AS TIMESTAMP))) as activity_span_seconds,
    COUNT(*) as event_count
  FROM cc_session
  WHERE process.executable.path IS NOT NULL
    AND process.audit_token.pid IS NOT NULL
  GROUP BY pid, executable_path
  HAVING COUNT(*) > 5  -- Only consider processes with reasonable activity
)
SELECT
  pid,
  executable_path,
  first_seen,
  last_seen,
  activity_span_seconds,
  event_count,
  -- Format duration in a human-readable way
  CASE
    WHEN activity_span_seconds >= 3600 THEN 
      FLOOR(activity_span_seconds / 3600) || 'h ' || 
      FLOOR((activity_span_seconds % 3600) / 60) || 'm ' || 
      FLOOR(activity_span_seconds % 60) || 's'
    WHEN activity_span_seconds >= 60 THEN 
      FLOOR(activity_span_seconds / 60) || 'm ' || 
      FLOOR(activity_span_seconds % 60) || 's'
    ELSE 
      FLOOR(activity_span_seconds) || 's'
  END as duration_formatted
FROM process_spans
ORDER BY activity_span_seconds DESC
LIMIT 20;
EOF
)

# Execute the SQL commands
echo "$SQL_COMMANDS" | duckdb "$DB_FILE"