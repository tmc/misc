#!/usr/bin/env python3
import duckdb
import json
import os

# Define paths
input_file = 'cc-session.ndjson'
output_db = 'cc_session.duckdb'

# Connect to DuckDB
con = duckdb.connect(output_db)

# Create a table from the NDJSON file
print(f"Loading {input_file} into DuckDB...")
con.sql(f"""
CREATE TABLE cc_session AS 
SELECT * FROM read_ndjson_auto('{input_file}')
""")

# Get record count
count = con.sql("SELECT COUNT(*) FROM cc_session").fetchone()[0]
print(f"Loaded {count} records into {output_db}")

# Show table schema
print("\nTable schema:")
schema = con.sql("DESCRIBE cc_session").fetchall()
for row in schema:
    print(f"{row[0]}: {row[1]}")

# Sample queries
print("\nSample record:")
con.sql("SELECT * FROM cc_session LIMIT 1").show()

print("\nEvent types distribution:")
con.sql("""
SELECT event_type, COUNT(*) as count 
FROM cc_session 
GROUP BY event_type 
ORDER BY count DESC 
LIMIT 10
""").show()

print("\nDone!")