#!/bin/bash

# Define the database file
DB_FILE="cc_session.duckdb"

# SQL queries to run
SQL_COMMANDS=$(cat << EOF
-- Top executables by count
SELECT 
  process.executable.path as executable_path, 
  COUNT(*) as event_count 
FROM cc_session 
WHERE process.executable.path IS NOT NULL
GROUP BY executable_path 
ORDER BY event_count DESC 
LIMIT 15;

-- Process start time distribution by hour
SELECT 
  SUBSTR(process.start_time, 12, 2) as hour, 
  COUNT(*) as count 
FROM cc_session 
WHERE process.start_time IS NOT NULL
GROUP BY hour 
ORDER BY hour;

-- Top event types and their descriptions
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
  LIMIT 15
)
SELECT * FROM event_types;

-- Count events by session ID
SELECT 
  process.session_id, 
  COUNT(*) as event_count 
FROM cc_session 
WHERE process.session_id IS NOT NULL
GROUP BY process.session_id 
ORDER BY event_count DESC 
LIMIT 10;

-- Top paths accessed
SELECT 
  json_extract(event, '$.open.target.path') as path,
  COUNT(*) as count
FROM cc_session
WHERE event_type = 39 -- open
GROUP BY path
ORDER BY count DESC
LIMIT 15;
EOF
)

# Execute the SQL queries with DuckDB
echo "Running analysis queries on $DB_FILE..."
echo "$SQL_COMMANDS" | duckdb "$DB_FILE"

echo "Analysis complete!"