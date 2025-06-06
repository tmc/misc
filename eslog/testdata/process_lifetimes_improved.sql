-- Analyze process lifetimes using exec and exit events (improved)
-- First let's check for duplicate exit events
SELECT
  process.audit_token.pid as pid,
  process.executable.path as executable_path,
  COUNT(*) as exit_count
FROM cc_session
WHERE event_type = 11  -- exit events
GROUP BY pid, executable_path
HAVING COUNT(*) > 1
ORDER BY exit_count DESC
LIMIT 10;

-- Now a better process lifetime query
WITH process_execs AS (
  -- Get process start times from exec events
  SELECT
    process.audit_token.pid as pid,
    json_extract(event, '$.exec.target.executable.path') as executable_path,
    time as start_time
  FROM cc_session
  WHERE event_type = 9  -- exec events
    AND json_extract(event, '$.exec.target.executable.path') IS NOT NULL
),
process_exits AS (
  -- Get process end times from exit events, taking the first exit event per pid
  SELECT
    process.audit_token.pid as pid,
    process.executable.path as executable_path,
    time as exit_time,
    ROW_NUMBER() OVER (PARTITION BY process.audit_token.pid ORDER BY time) as rn
  FROM cc_session
  WHERE event_type = 11  -- exit events
    AND process.executable.path IS NOT NULL
),
process_lifetimes AS (
  -- Join start and end times, using only the first exit event per pid
  SELECT
    e.pid,
    e.executable_path,
    e.start_time,
    x.exit_time,
    -- Calculate duration in seconds
    CASE 
      WHEN x.exit_time IS NOT NULL THEN
        (EXTRACT(EPOCH FROM CAST(x.exit_time AS TIMESTAMP)) - 
         EXTRACT(EPOCH FROM CAST(e.start_time AS TIMESTAMP)))
      ELSE NULL
    END as duration_seconds
  FROM process_execs e
  LEFT JOIN process_exits x ON e.pid = x.pid AND x.rn = 1
)
SELECT
  pid,
  executable_path,
  start_time,
  exit_time,
  duration_seconds,
  -- Format duration in a human-readable way
  CASE
    WHEN duration_seconds IS NULL THEN 'No exit event'
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
ORDER BY 
  CASE WHEN duration_seconds IS NULL THEN 1 ELSE 0 END,  -- Put NULL durations at the end
  duration_seconds DESC
LIMIT 20;