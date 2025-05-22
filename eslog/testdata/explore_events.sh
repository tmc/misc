#!/bin/bash

# Define the database file
DB_FILE="cc_session.duckdb"

# SQL queries to run
SQL_COMMANDS=$(cat << EOF
-- Extract sample events by type
WITH event_samples AS (
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
    event,
    ROW_NUMBER() OVER (PARTITION BY event_type ORDER BY mach_time) as rn
  FROM cc_session
  WHERE event_type IN (9, 10, 11, 12, 21, 33, 39, 43)
)
SELECT event_type, event_name, event
FROM event_samples
WHERE rn = 1
ORDER BY event_type;

-- Sample exec events with process path and arguments
SELECT 
  time,
  event_type,
  json_extract(event, '$.exec.target.executable.path') as target_path,
  json_extract(event, '$.exec.args') as args
FROM cc_session
WHERE event_type = 9  -- exec events
LIMIT 5;

-- Find process spawning (fork/exec) patterns
WITH process_events AS (
  SELECT
    time,
    global_seq_num,
    event_type,
    CASE event_type
      WHEN 9 THEN 'exec'
      WHEN 10 THEN 'fork'
      WHEN 11 THEN 'exit'
      ELSE 'other'
    END as event_name,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    json_extract(event, '$.exec.target.executable.path') as exec_target,
    json_extract(event, '$.fork.child.executable.path') as fork_child_path,
    json_extract(event, '$.fork.child.audit_token.pid') as fork_child_pid
  FROM cc_session
  WHERE event_type IN (9, 10, 11)
  ORDER BY time
)
SELECT * FROM process_events
LIMIT 10;

-- Analyze file access patterns
SELECT 
  time,
  event_type,
  CASE event_type
    WHEN 39 THEN 'open'
    WHEN 12 THEN 'close'
    WHEN 33 THEN 'write'
    WHEN 55 THEN 'access'
    ELSE 'other'
  END as event_name,
  process.executable.path as process_path,
  json_extract(event, '$.open.target.path') as open_path,
  json_extract(event, '$.close.target.path') as close_path,
  json_extract(event, '$.write.target.path') as write_path,
  json_extract(event, '$.access.target.path') as access_path
FROM cc_session
WHERE event_type IN (39, 12, 33, 55)
  AND (
    json_extract(event, '$.open.target.path') IS NOT NULL OR
    json_extract(event, '$.close.target.path') IS NOT NULL OR
    json_extract(event, '$.write.target.path') IS NOT NULL OR
    json_extract(event, '$.access.target.path') IS NOT NULL
  )
ORDER BY time
LIMIT 10;
EOF
)

# Execute the SQL queries with DuckDB
echo "Exploring event details in $DB_FILE..."
echo "$SQL_COMMANDS" | duckdb "$DB_FILE"

echo "Exploration complete!"