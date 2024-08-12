#!/bin/bash
set -euo pipefail

# Function to run a command and print its output
run_command() {
    echo "$ "
    output=
    echo ""
    echo
}

export PATH="/workspace/git-goals:/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Set up a temporary test directory
test_dir=/tmp/tmp.Nlv7IUMxOa
cd ""

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
goal_id=

if [ -z "" ]; then
    echo "Error: Failed to extract goal ID for 'Implement new feature'"
    exit 1
fi

# Test goal show
run_command git goals show ""

# Test goal update
run_command git goals update "" "Implement new feature with improved performance"

# Test goal show after update
run_command git goals show ""

# Test goal completion
run_command git goals complete "" "" "Feature implemented and tested"

# Test goal report
run_command git goals report

# Test goal deletion
run_command git goals delete ""

# Verify goal is deleted
if git goals list | grep -q ""; then
    echo "Error: Goal  still exists after deletion"
    exit 1
else
    echo "Goal  successfully deleted"
fi

echo "All tests completed successfully!"

# Clean up
cd ..
rm -rf ""
