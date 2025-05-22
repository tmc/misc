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

-- Find all network connections initiated by Claude-related processes
WITH claude_pids AS (
  SELECT DISTINCT process.audit_token.pid as pid
  FROM cc_session
  WHERE
    LOWER(process.executable.path) LIKE '%claude%' OR
    CAST(process.executable.path AS VARCHAR) LIKE '%node%'
),
network_events AS (
  SELECT 
    time,
    process.audit_token.pid as pid,
    process.executable.path as process_path,
    event_type,
    event
  FROM cc_session
  WHERE process.audit_token.pid IN (SELECT pid FROM claude_pids)
    AND (
      event_type = 97 OR  -- socket
      event_type = 98 OR  -- connect
      event_type = 99     -- bind
    )
  ORDER BY time
)
SELECT * FROM network_events
LIMIT 20;

-- Find file access patterns for Claude-related processes
WITH claude_pids AS (
  SELECT DISTINCT process.audit_token.pid as pid
  FROM cc_session
  WHERE
    LOWER(process.executable.path) LIKE '%claude%' OR
    CAST(process.executable.path AS VARCHAR) LIKE '%node%'
),
file_access AS (
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
  WHERE process.audit_token.pid IN (SELECT pid FROM claude_pids)
    AND event_type IN (39, 12, 33)
    AND (
      json_extract(event, '$.open.target.path') IS NOT NULL OR
      json_extract(event, '$.close.target.path') IS NOT NULL OR
      json_extract(event, '$.write.target.path') IS NOT NULL
    )
  ORDER BY time
)
SELECT * FROM file_access
LIMIT 20;