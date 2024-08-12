#!/bin/bash

# This script will attempt to fix issues in the git-goals codebase

set -euo pipefail

# Function to update a script
update_script() {
  local script_name=$1
  local content=$2

  echo "Updating $script_name..."
  echo "$content" > "$script_name"
  chmod +x "$script_name"
}

# Update git-goals-show
update_script "git-goals-show" '#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals show <goal_id>"
    exit 1
fi

goal_id="$1"

commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

git notes --ref=goals show "$commit_hash"
'

# Update git-goals-list
update_script "git-goals-list" '#!/bin/bash
set -euo pipefail

echo "Current Goals:"
git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash")
    id=$(echo "$goal_data" | grep "^id:" | cut -d" " -f2-)
    status=$(echo "$goal_data" | grep "^status:" | cut -d" " -f2-)
    description=$(echo "$goal_data" | grep "^description:" | cut -d" " -f2-)
    echo "- $id ($status): $description"
done
'

# Update test-git-goals.sh
update_script "test-git-goals.sh" '#!/bin/bash
set -euo pipefail

# Function to run a command and print its output
run_command() {
    echo "$ $*"
    output=$("$@")
    echo "$output"
    echo
}

export PATH="/workspace/git-goals:$PATH"

# Set up a temporary test directory
test_dir=$(mktemp -d)
cd "$test_dir"

echo "Setting up test repository..."
git init
git config user.email "test@example.com"
git config user.name "Test User"
git commit --allow-empty -m "Initial commit"

echo "Testing git-goals..."

# Test goal creation
run_command git goals create "Implement new feature"

# Test goal listing
run_command git goals list

# Get the goal ID from the list output
goal_id=$(git goals list | grep "Implement new feature" | cut -d" " -f2 | tr -d "()")

if [ -z "$goal_id" ]; then
    echo "Error: Failed to extract goal ID for '\''Implement new feature'\''"
    exit 1
fi

# Test goal show
run_command git goals show "$goal_id"

# Test goal update
run_command git goals update "$goal_id" "Implement new feature with improved performance"

# Test goal show after update
run_command git goals show "$goal_id"

# Test goal completion
run_command git goals complete "$goal_id" "" "Feature implemented and tested"

# Test goal report
run_command git goals report

# Test goal deletion
run_command git goals delete "$goal_id"

# Verify goal is deleted
if git goals list | grep -q "$goal_id"; then
    echo "Error: Goal $goal_id still exists after deletion"
    exit 1
else
    echo "Goal $goal_id successfully deleted"
fi

echo "All tests completed successfully!"

# Clean up
cd ..
rm -rf "$test_dir"
'

echo "Scripts have been updated. Please run the test script to verify the changes."

# Run the test script
echo "Running test script..."
./test-git-goals.sh

echo "Fixes have been applied. Please review the changes and test results."

# Sleep for a longer period if we're getting close to being done
sleep 30