-- Advanced Queries for ESLog Analysis
-- These queries demonstrate more complex analysis of Endpoint Security logs

-- 1. Process Execution Chain: Shows the sequence of process executions with parent-child relationships
WITH process_chain AS (
  SELECT
    time,
    event_type,
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as target_path,
    json_extract(event, '$.exec.args') as args,
    ROW_NUMBER() OVER (PARTITION BY process.audit_token.pid ORDER BY time) as rn
  FROM cc_session
  WHERE event_type = 9  -- exec events
  ORDER BY time
)
SELECT 
  pc1.time,
  pc1.pid,
  pc1.ppid,
  p_parent.executable.path as parent_process,
  pc1.process_path,
  pc1.target_path,
  pc1.args
FROM process_chain pc1
LEFT JOIN cc_session p_parent ON pc1.ppid = p_parent.process.audit_token.pid
WHERE pc1.rn = 1
ORDER BY pc1.time
LIMIT 20;

-- 2. File Operation Frequency: Shows which files are most frequently accessed
WITH file_ops AS (
  SELECT 
    COALESCE(
      json_extract(event, '$.open.target.path'), 
      json_extract(event, '$.close.target.path'),
      json_extract(event, '$.write.target.path')
    ) as file_path,
    CASE 
      WHEN event_type = 39 THEN 'open'
      WHEN event_type = 12 THEN 'close'
      WHEN event_type = 33 THEN 'write'
      ELSE 'other'
    END as operation,
    COUNT(*) as count
  FROM cc_session
  WHERE event_type IN (39, 12, 33)  -- open, close, write
    AND (
      json_extract(event, '$.open.target.path') IS NOT NULL OR
      json_extract(event, '$.close.target.path') IS NOT NULL OR
      json_extract(event, '$.write.target.path') IS NOT NULL
    )
  GROUP BY file_path, operation
)
SELECT 
  file_path,
  SUM(count) as total_operations,
  SUM(CASE WHEN operation = 'open' THEN count ELSE 0 END) as open_count,
  SUM(CASE WHEN operation = 'close' THEN count ELSE 0 END) as close_count,
  SUM(CASE WHEN operation = 'write' THEN count ELSE 0 END) as write_count
FROM file_ops
GROUP BY file_path
ORDER BY total_operations DESC
LIMIT 20;

-- 3. Process Resource Usage: Analyze which processes are using the most resources
WITH process_stats AS (
  SELECT
    process.executable.path as process_path,
    COUNT(*) as total_events,
    COUNT(CASE WHEN event_type = 39 THEN 1 END) as file_opens,
    COUNT(CASE WHEN event_type = 43 THEN 1 END) as mmaps,
    COUNT(CASE WHEN event_type = 33 THEN 1 END) as writes
  FROM cc_session
  WHERE process.executable.path IS NOT NULL
  GROUP BY process_path
)
SELECT 
  process_path,
  total_events,
  file_opens,
  mmaps,
  writes,
  ROUND(100.0 * total_events / SUM(total_events) OVER (), 2) as percent_of_total
FROM process_stats
ORDER BY total_events DESC
LIMIT 20;

-- 4. Command Line Reconstruction: Reconstruct full command lines for process executions
WITH exec_events AS (
  SELECT
    time,
    process.audit_token.pid as pid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.target.executable.path') as target_path,
    json_extract(event, '$.exec.args') as args
  FROM cc_session
  WHERE event_type = 9  -- exec events
)
SELECT
  time,
  pid,
  process_path,
  target_path,
  args,
  CASE 
    WHEN args IS NOT NULL THEN list_aggregate(array_unnest(args::VARCHAR[]), ' ')
    ELSE target_path::VARCHAR
  END as command_line
FROM exec_events
ORDER BY time
LIMIT 20;

-- 5. Process Lifetime: Calculate how long processes ran for
WITH process_lifetimes AS (
  SELECT
    process.audit_token.pid as pid,
    process.executable.path as process_path,
    MIN(time) as start_time,
    MAX(time) as last_seen_time
  FROM cc_session
  GROUP BY pid, process_path
)
SELECT
  pid,
  process_path,
  start_time,
  last_seen_time,
  -- Calculate approximate duration in seconds
  EXTRACT(EPOCH FROM CAST(last_seen_time AS TIMESTAMP) - CAST(start_time AS TIMESTAMP)) as duration_seconds
FROM process_lifetimes
WHERE process_path IS NOT NULL
ORDER BY duration_seconds DESC
LIMIT 20;

-- 6. Process Environment Variables: Extract environment variables from exec events
WITH env_vars AS (
  SELECT
    time,
    process.audit_token.pid as pid,
    process.executable.path as process_path,
    json_extract(event, '$.exec.env') as env
  FROM cc_session
  WHERE event_type = 9  -- exec events
    AND json_extract(event, '$.exec.env') IS NOT NULL
)
SELECT
  time,
  pid,
  process_path,
  env
FROM env_vars
ORDER BY time
LIMIT 5;

-- 7. Security Relevant Events: Focus on potentially security-relevant activities
SELECT
  time,
  event_type,
  CASE event_type
    WHEN 9 THEN 'exec'
    WHEN 10 THEN 'fork'
    WHEN 11 THEN 'exit'
    WHEN 12 THEN 'close'
    WHEN 33 THEN 'write'
    WHEN 39 THEN 'open'
    ELSE 'other'
  END as event_name,
  process.audit_token.pid as pid,
  process.executable.path as process_path,
  CASE 
    WHEN event_type = 9 THEN json_extract(event, '$.exec.target.executable.path')
    WHEN event_type = 39 THEN json_extract(event, '$.open.target.path')
    WHEN event_type = 33 THEN json_extract(event, '$.write.target.path')
    ELSE NULL
  END as target_path
FROM cc_session
WHERE 
  -- Filter for security-relevant events
  (
    -- Executable writes
    (event_type = 33 AND json_extract(event, '$.write.target.path')::VARCHAR LIKE '%.exe' OR
     json_extract(event, '$.write.target.path')::VARCHAR LIKE '%.dylib' OR
     json_extract(event, '$.write.target.path')::VARCHAR LIKE '%.so' OR
     json_extract(event, '$.write.target.path')::VARCHAR LIKE '%.bin') OR
    -- Executable opens with write permissions
    (event_type = 39 AND json_extract(event, '$.open.target.path')::VARCHAR LIKE '%.exe' OR
     json_extract(event, '$.open.target.path')::VARCHAR LIKE '%.dylib' OR
     json_extract(event, '$.open.target.path')::VARCHAR LIKE '%.so' OR
     json_extract(event, '$.open.target.path')::VARCHAR LIKE '%.bin') OR
    -- Suspicious paths
    (json_extract(event, '$.open.target.path')::VARCHAR LIKE '/tmp/%' OR
     json_extract(event, '$.write.target.path')::VARCHAR LIKE '/tmp/%')
  )
ORDER BY time
LIMIT 20;

-- 8. Process Trees: Reconstruct process trees
WITH RECURSIVE process_tree AS (
  -- Base case: processes without parents in our dataset
  SELECT
    process.audit_token.pid as pid,
    process.ppid as ppid,
    process.executable.path as process_path,
    0 as level,
    process.executable.path as tree_path
  FROM cc_session
  WHERE process.ppid = 0 OR process.ppid IS NULL
  GROUP BY pid, ppid, process_path
  
  UNION ALL
  
  -- Recursive case: child processes
  SELECT
    c.process.audit_token.pid as pid,
    c.process.ppid as ppid,
    c.process.executable.path as process_path,
    pt.level + 1 as level,
    pt.tree_path || ' -> ' || c.process.executable.path as tree_path
  FROM cc_session c
  JOIN process_tree pt ON c.process.ppid = pt.pid
  GROUP BY c.process.audit_token.pid, c.process.ppid, c.process.executable.path, pt.level, pt.tree_path
)
SELECT
  pid,
  ppid,
  level,
  repeat('  ', level) || process_path as indented_path,
  tree_path
FROM process_tree
ORDER BY tree_path, level
LIMIT 100;