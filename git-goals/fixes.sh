#!/bin/bash"

# Analyze the current state of the project
echo "Analyzing project state..."
git status
git log -n 5 --oneline

# Update the test script
echo "Updating test-git-goals.sh..."
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

export PATH="/workspace/git-goals:/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

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
goal_id=$(git goals list | grep "Implement new feature" | awk '{print $2}' | tr -d '()')

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

# Update the git-goals-create script
echo "Updating git-goals-create..."
cat << 'EOF' > git-goals-create
#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Error: Please provide a description for the goal."
    echo "Usage: git goals create <description>"
    exit 1
fi

description="$*"
id=$(date +%Y%m%d%H%M%S)

git notes --ref=goals add -m "id: $id
type: goal
description: $description
status: active
created_at: $(date -I)" HEAD

echo "Created new goal with ID: $id"
echo "Description: $description"
EOF

chmod +x git-goals-create

# Update the git-goals-list script
echo "Updating git-goals-list..."
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

# Run the test script in the sandbox
echo "Running test script in sandbox..."
sandbox-exec ./test-git-goals.sh

# Commit changes
echo "Committing changes..."
git add test-git-goals.sh git-goals-create git-goals-list
git commit -m "git-notes: Updating test script and git-goals commands"

echo "Script execution completed."