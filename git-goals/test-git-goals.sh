#!/bin/bash
set -euo pipefail

# Function to run a command and print its output
run_command() {
    echo "$ $@"
    output=$("$@")
    echo "$output"
    echo
}

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
goal_id=$(git goals list | grep "Implement new feature" | cut -d"(" -f1 | tr -d '- ')

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