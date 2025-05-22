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