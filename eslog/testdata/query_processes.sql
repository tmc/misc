-- Process executions with their arguments and environment
WITH process_execs AS (
  SELECT
    time,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as target_path,
    json_extract(event, '$.exec.args') as args,
    json_extract(event, '$.exec.env') as env
  FROM cc_session
  WHERE event_type = 9  -- exec events
  ORDER BY time
)
SELECT * FROM process_execs;

-- Get Claude-related commands and arguments
WITH claude_commands AS (
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
      LOWER(json_extract(event, '$.exec.target.executable.path')::VARCHAR) LIKE '%claude%' OR
      array_contains(json_extract(event, '$.exec.args')::VARCHAR[], '%claude%')
    )
  ORDER BY time
)
SELECT * FROM claude_commands;