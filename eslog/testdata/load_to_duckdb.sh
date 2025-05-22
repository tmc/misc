#!/bin/bash

# Define paths
INPUT_FILE="cc-session.ndjson"
OUTPUT_DB="cc_session.duckdb"

# Create SQL for DuckDB
SQL_COMMANDS=$(cat << EOF
-- Create a table from the NDJSON file
CREATE TABLE cc_session AS 
SELECT * FROM read_ndjson_auto('$INPUT_FILE');

-- Get record count
SELECT COUNT(*) FROM cc_session;

-- Show table schema
DESCRIBE cc_session;

-- Sample queries
SELECT * FROM cc_session LIMIT 1;

-- Event types distribution
SELECT event_type, COUNT(*) as count 
FROM cc_session 
GROUP BY event_type 
ORDER BY count DESC 
LIMIT 10;
EOF
)

# Execute the SQL commands with DuckDB
echo "Loading $INPUT_FILE into DuckDB..."
echo "$SQL_COMMANDS" | duckdb "$OUTPUT_DB"

echo "Done! Database saved to $OUTPUT_DB"