#!/bin/bash

# Update git-goals-list script
cat << 'EOF' > git-goals-list
#!/bin/bash
set -euo pipefail

echo "Current Goals:"
git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash")
    id=$(echo "$goal_data" | grep "^id:" | cut -d" " -f2-)
    status=$(echo "$goal_data" | grep "^status:" | cut -d" " -f2-)
    description=$(echo "$goal_data" | grep "^description:" | cut -d" " -f2-)
    echo "- $id ($status): $description"
done
EOF
chmod +x git-goals-list

# Update git-goals-report script
cat << 'EOF' > git-goals-report
#!/bin/bash
set -euo pipefail

echo "Goal Report"
echo "==========="

git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash")

    id=$(echo "$goal_data" | grep "^id:" | cut -d" " -f2-)
    description=$(echo "$goal_data" | grep "^description:" | cut -d" " -f2-)
    status=$(echo "$goal_data" | grep "^status:" | cut -d" " -f2-)
    created_at=$(echo "$goal_data" | grep "^created_at:" | cut -d" " -f2-)
    completed_at=$(echo "$goal_data" | grep "^completed_at:" | cut -d" " -f2-)

    echo "Goal ID: $id"
    echo "Description: $description"
    echo "Status: $status"
    echo "Created: $created_at"
    if [ "$status" = "completed" ]; then
        echo "Completed: $completed_at"
    fi
    echo "---"
done
EOF
chmod +x git-goals-report

# Create git-goals main script
cat << 'EOF' > git-goals
#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: create, list, show, update, delete, complete, report"
    exit 1
fi

subcommand="$1"
shift

case "$subcommand" in
    create|list|show|update|delete|complete|report)
        "git-goals-$subcommand" "$@"
        ;;
    *)
        echo "Error: Unknown subcommand '$subcommand'"
        echo "Available subcommands: create, list, show, update, delete, complete, report"
        exit 1
        ;;
esac
EOF
chmod +x git-goals

# Update test-git-goals.sh script
cat << 'EOF' > test-git-goals.sh
#!/bin/bash
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
EOF
chmod +x test-git-goals.sh

echo "All git-goals scripts have been updated and the main git-goals script has been created."