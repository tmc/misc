#!/usr/bin/env -S bash -euo pipefail
set -euo pipefail
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
    echo "Error: Failed to extract goal ID for 'Implement new feature'"
    exit 1
fi

# Test goal show
run_command git goals show "$goal_id"

# Test goal update
run_command git goals update "$goal_id" "Implement new feature with improved performance"

# Test goal show after update
run_command git goals show "$goal_id"

# Test goal prioritization
run_command git goals prioritize "$goal_id" "high"

# Test goal show after prioritization
run_command git goals show "$goal_id"

# Test setting goal deadline
run_command git goals deadline "$goal_id" "2024-12-31"

# Test goal show after setting deadline
run_command git goals show "$goal_id"

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

# Test notification system
echo "Testing notification system..."
test_goal_id=$(git goals create "Test goal with deadline")
git goals deadline $test_goal_id "2023-12-31"
notification_output=$(git goals notify)
if [[ $notification_output == *"WARNING: Goal $test_goal_id is due"* ]]; then
    echo "Notification test passed"
else
    echo "Notification test failed"
    exit 1
fi

# Clean up test goal
git goals delete $test_goal_id

# Test priority sorting in list command
echo "Testing priority sorting in list command..."
high_priority_goal=
git goals prioritize $high_priority_goal high
medium_priority_goal=
git goals prioritize $medium_priority_goal medium
low_priority_goal=
git goals prioritize $low_priority_goal low

sorted_list=
if [[ "$sorted_list" == *"High priority goal"*"Medium priority goal"*"Low priority goal"* ]]; then
    echo "Priority sorting test passed"
else
    echo "Priority sorting test failed"
    exit 1
fi

# Clean up test goals
git goals delete $high_priority_goal
git goals delete $medium_priority_goal
git goals delete $low_priority_goal

# Test plugin system
echo "Testing plugin system..."
source plugins/examples/hello_world.sh
output=$(git_goals_hello_world)
if [[ "$output" == "Hello from the git-goals plugin system!" ]]; then
    echo "Plugin test passed"
else
    echo "Plugin test failed"
    exit 1
fi
