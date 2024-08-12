#!/bin/bash

# Function to create a new git commit
create_commit() {
    git add -u .
    git commit -m "git-notes: Fixing the codebase" --allow-empty
}

# Function to run command in sandbox and capture output
run_sandbox() {
    local command="$1"
    sandbox-exec "$command" > sandbox_output.txt 2>&1
}

# Function to analyze sandbox output and suggest improvements
analyze_sandbox() {
    echo "Analyzing sandbox execution..."
    (echo "we will fix what went wrong"; cat sandbox_output.txt; git st .; ~/code-to-gpt.sh) | cgpt -s "we are fixing issues in this codebase. output an invocation of ./make-fix-suggestion.sh; at the top of the bash script, output comments explaining <antthinking thoughts" -p "./make-fix-suggestion.sh" | tee mkfix.sh
}

# Function to apply suggested fixes
apply_fixes() {
    echo "Applying suggested fixes..."
    chmod +x mkfix.sh
    ./mkfix.sh
}

# Main execution loop
create_commit

echo "Running tests in sandbox..."
run_sandbox "./test-git-goals.sh"

analyze_sandbox

apply_fixes

create_commit

echo "Iteration complete. Fixes have been applied and committed."

# Attach sandbox content to the latest commit
attach-sandbox-content

# Add git notes for analysis
(ctx-exec git show --notes='*'; ctx-exec git st .; ~/code-to-gpt.sh) | cgpt -s $(cat .claude-system-prompt-analyze-attempt) |tee /tmp/claude-analysis; cat /tmp/claude-analysis |git notes --ref attempt-analysis add -F-

# Evaluate the progress
sh eval-make-safe-progress.sh | tee /tmp/eval-make-safe-progress; cat /tmp/eval-make-safe-progress |git notes --ref eval-safe-progress add -F-

echo "Iteration complete. Press Enter to continue or Ctrl+C to exit."
read