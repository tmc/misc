#!/bin/bash

# ESLog Analyzer Setup Script
# This script helps set up the environment for analyzing ES logs

set -e  # Exit on any error

echo "ESLog Analyzer Setup"
echo "===================="
echo ""

# Check if DuckDB is installed
if ! command -v duckdb &> /dev/null; then
    echo "DuckDB is not installed. Would you like to install it? (y/n)"
    read -r install_duckdb
    
    if [[ "$install_duckdb" =~ ^[Yy]$ ]]; then
        echo "Installing DuckDB via Homebrew..."
        if ! command -v brew &> /dev/null; then
            echo "Error: Homebrew is not installed. Please install Homebrew first."
            echo "Visit https://brew.sh for installation instructions."
            exit 1
        fi
        brew install duckdb
    else
        echo "Please install DuckDB to use this tool."
        echo "You can install it manually with: brew install duckdb"
        exit 1
    fi
fi

echo "DuckDB is installed: $(duckdb --version)"
echo ""

# Check for NDJSON files
echo "Checking for NDJSON files in the current directory..."
NDJSON_FILES=$(find . -name "*.ndjson" -type f)

if [ -z "$NDJSON_FILES" ]; then
    echo "No NDJSON files found in the current directory."
    echo "Please place your ES log files (*.ndjson) in this directory."
else
    echo "Found the following NDJSON files:"
    echo "$NDJSON_FILES"
    
    # Make our scripts executable
    echo ""
    echo "Making scripts executable..."
    chmod +x *.sh

    # If there's only one file and it's cc-session.ndjson, offer to load it
    if [ "$NDJSON_FILES" == "./cc-session.ndjson" ]; then
        echo ""
        echo "Would you like to load cc-session.ndjson into DuckDB now? (y/n)"
        read -r load_now
        
        if [[ "$load_now" =~ ^[Yy]$ ]]; then
            echo "Loading data into DuckDB..."
            ./eslog_analyzer.sh load
            
            echo ""
            echo "Generating a summary..."
            ./eslog_analyzer.sh summary
        fi
    fi
fi

echo ""
echo "Setup complete! You can now use the ESLog Analyzer:"
echo ""
echo "  ./eslog_analyzer.sh [command]"
echo ""
echo "Available commands:"
echo "  load             - Load the NDJSON file into DuckDB"
echo "  summary          - Show a summary of the database"
echo "  processes        - List all processes captured in the log"
echo "  file-access      - Show file access patterns"
echo "  network          - Show network connections"
echo "  process-tree     - Show process hierarchy"
echo "  events [type]    - Show events of specified type (number)"
echo "  claude           - Show Claude-related activities"
echo "  query [sql]      - Run a custom SQL query"
echo ""
echo "For more information, refer to the README.md file."