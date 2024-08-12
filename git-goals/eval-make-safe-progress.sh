#!/bin/bash
set -euo pipefail

# Function to analyze sandbox output
analyze_sandbox_output() {
    echo "Analyzing sandbox output..."
    if [ -f sandbox_output.txt ]; then
        cat sandbox_output.txt | grep -v "^$" | tail -n 20 | cgpt -s "Analyze this sandbox output and suggest improvements or next steps:"
    else
        echo "No sandbox output found."
    fi
}

# Function to analyze execution context
analyze_execution_context() {
    echo "Analyzing execution context..."
    if [ -f execution_context.txt ]; then
        cat execution_context.txt | cgpt -s "Analyze this execution context and provide insights:"
    else
        echo "No execution context found."
    fi
}

# Function to analyze suggested changes
analyze_suggested_changes() {
    echo "Analyzing suggested changes..."
    if [ -f suggested_changes.txt ]; then
        cat suggested_changes.txt | cgpt -s "Review these suggested changes and provide a summary of the most important improvements:"
    else
        echo "No suggested changes found."
    fi
}

# Function to analyze git commits
analyze_git_commits() {
    echo "Analyzing recent git commits..."
    git log --oneline -n 5 | cgpt -s "Analyze these recent commits and suggest next steps for the project:"
}

# Main execution
echo "Evaluating make-safe-progress.sh runs..."

analyze_sandbox_output
echo

analyze_execution_context
echo

analyze_suggested_changes
echo

analyze_git_commits
echo

echo "Evaluation complete. Please review the analysis above to determine the next steps for improving the project."