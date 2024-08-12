#!/bin/bash

# This script will update all git-goals-* scripts to reflect recent fixes and improvements

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
# git-goals-show - Shows details of a specific goal
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals show <goal_id>"
    exit 1
fi

goal_id="$1"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

# Display the goal details
git notes --ref=goals show "$commit_hash"
'

# Update git-goals-delete
update_script "git-goals-delete" '#!/bin/bash
# git-goals-delete - Deletes a goal
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals delete <goal_id>"
    exit 1
fi

goal_id="$1"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

# Remove the git note for the goal
git notes --ref=goals remove "$commit_hash"

echo "Goal $goal_id deleted"
'

# Update git-goals-update
update_script "git-goals-update" '#!/bin/bash
# git-goals-update - Updates an existing goal
set -euo pipefail

if [ $# -lt 2 ]; then
    echo "Usage: git goals update <goal_id> <new_description>"
    exit 1
fi

goal_id="$1"
shift
new_description="$*"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

# Get the current goal data
current_data=$(git notes --ref=goals show "$commit_hash")

# Update the description
updated_data=$(echo "$current_data" | sed "s/^description:.*$/description: $new_description/")

# Update the git note
echo "$updated_data" | git notes --ref=goals add -f -F - "$commit_hash"

echo "Updated goal $goal_id: $new_description"
'

# Update git-goals-report
update_script "git-goals-report" '#!/bin/bash
# git-goals-report - Generates a report of all goals
set -euo pipefail

echo "Goal Report"
echo "==========="

git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash")

    goal_id=$(echo "$goal_data" | grep "^id:" | cut -d" " -f2-)
    description=$(echo "$goal_data" | grep "^description:" | cut -d" " -f2-)
    status=$(echo "$goal_data" | grep "^status:" | cut -d" " -f2-)
    created_at=$(echo "$goal_data" | grep "^created_at:" | cut -d" " -f2-)
    completed_at=$(echo "$goal_data" | grep "^completed_at:" | cut -d" " -f2-)

    echo "Goal ID: $goal_id"
    echo "Description: $description"
    echo "Status: $status"
    echo "Created: $created_at"
    if [ "$status" = "completed" ]; then
        echo "Completed: $completed_at"
    fi
    echo "---"
done
'

# Update git-goals-list
update_script "git-goals-list" '#!/bin/bash
# git-goals-list - Lists all goals
set -euo pipefail

echo "Current Goals:"

# Check if there are any notes
if ! git notes --ref=goals list > /dev/null 2>&1; then
    echo "No goals found."
    exit 0
fi

git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash" 2>/dev/null)

    # Check if it'"'"'s the old format (contains '"'"'tree'"'"' and '"'"'parent'"'"')
    if echo "$goal_data" | grep -q "tree"; then
        description=$(echo "$goal_data" | grep "Goal:" | sed '"'"'s/Goal: //'"'"')
        if [ -z "$description" ]; then
            echo "Warning: No goal description found for commit $commit_hash"
            continue
        fi
        echo "- $commit_hash (old format): $description"
    else
        goal_id=$(echo "$goal_data" | grep "^id:" | cut -d'"'"' '"'"' -f2-)
        description=$(echo "$goal_data" | grep "^description:" | cut -d'"'"' '"'"' -f2-)
        status=$(echo "$goal_data" | grep "^status:" | cut -d'"'"' '"'"' -f2-)

        if [ -z "$goal_id" ] || [ -z "$description" ] || [ -z "$status" ]; then
            echo "Warning: Incomplete goal data for commit $commit_hash"
            continue
        fi

        echo "- $commit_hash $goal_id ($status): $description"
    fi
done
'

# Update git-goals-create
update_script "git-goals-create" '#!/bin/bash
# git-goals-create - Creates a new goal
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals create <goal_description>"
    exit 1
fi

description="$*"
goal_id=$(date +%Y%m%d%H%M%S)

# Prepare goal metadata
goal_metadata="id: $goal_id
type: goal
description: $description
status: active
created_at: $(date -I)"

# Create a new empty commit
git commit --allow-empty -m "Goal: $description"
commit_hash=$(git rev-parse HEAD)

# Add goal metadata as a Git note
git notes --ref=goals add -f -m "$goal_metadata" $commit_hash

echo "Created new goal with ID: $goal_id"
echo "Description: $description"
'

# Update git-goals-complete
update_script "git-goals-complete" '#!/bin/bash
# git-goals-complete - Marks a goal as complete
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: git goals complete <goal_id> [attempt_id] [rationale]"
    exit 1
fi

goal_id="$1"
attempt_id="${2:-}"
rationale="${3:-}"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

# Get the current goal data
current_data=$(git notes --ref=goals show "$commit_hash")

# Update the status and add completion details
updated_data=$(echo "$current_data" | sed "s/^status:.*$/status: completed/")
updated_data+="
completed_at: $(date -I)"

if [ -n "$attempt_id" ]; then
    updated_data+="
attempt_id: $attempt_id"
fi

if [ -n "$rationale" ]; then
    updated_data+="
rationale: $rationale"
fi

# Update the git note
echo "$updated_data" | git notes --ref=goals add -f -F - "$commit_hash"

echo "Goal $goal_id marked as complete"
if [ -n "$rationale" ]; then
    echo "Rationale: $rationale"
fi
'

# Update git-goals (main script)
update_script "git-goals" '#!/bin/bash
# git-goals - Main entry point for git-goals commands

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
    echo "Usage: git goals <command> [<args>]"
    echo
    echo "Available commands:"
    echo "  create    Create a new goal"
    echo "  update    Update an existing goal"
    echo "  list      List all goals"
    echo "  show      Show details of a specific goal"
    echo "  complete  Mark a goal as complete"
    echo "  delete    Delete a goal"
    echo "  report    Generate a report of all goals"
    echo
    echo "Run '"'"'git goals <command> --help'"'"' for more information on a specific command."
}

if [ $# -eq 0 ]; then
    usage
    exit 1
fi

command="$1"
shift

case "$command" in
    create|update|list|show|complete|delete|report)
        exec "$SCRIPT_DIR/git-goals-$command" "$@"
        ;;
    --help|-h|help)
        usage
        exit 0
        ;;
    *)
        echo "Error: Unknown command '"'"'$command'"'"'" >&2
        usage
        exit 1
        ;;
esac
'

echo "All git-goals-* scripts have been updated."

# Update README.md
cat > README.md << EOL
# git-goals

git-goals is a set of command-line tools to manage and track goals within a Git repository. It allows you to create, update, list, and complete goals, as well as generate reports on your progress.

## Installation

1. Clone this repository or download the scripts.
2. Add the directory containing these scripts to your PATH.
3. Ensure the scripts are executable (\`chmod +x git-goals-*\`).

## Usage

### Create a new goal

\`\`\`
git goals create <goal_description>
\`\`\`

This creates a new goal with the given description.

### Update a goal

\`\`\`
git goals update <goal_id> <new_goal_description>
\`\`\`

Updates an existing goal with a new description.

### List goals

\`\`\`
git goals list
\`\`\`

Displays a list of all current goals with their IDs, statuses, and descriptions.

### Show goal details

\`\`\`
git goals show <goal_id>
\`\`\`

Displays detailed information about a specific goal.

### Complete a goal

\`\`\`
git goals complete <goal_id> [attempt_id] [rationale]
\`\`\`

Marks a goal as complete, optionally with an attempt selection and rationale.

### Delete a goal

\`\`\`
git goals delete <goal_id>
\`\`\`

Deletes a goal by its ID.

### Generate a report

\`\`\`
git goals report
\`\`\`

Generates a comprehensive report of all goals, including their statuses, descriptions, creation dates, and completion dates.

## How it works

git-goals uses Git notes to store goal metadata. Each goal is associated with a specific commit, and the goal information is stored as a note on that commit. The tools provided allow you to manage these notes and the associated goals easily.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
EOL

echo "README.md has been updated."

# Update USAGE.md
cat > USAGE.md << EOL
## Example Usage

Here's an example of how to use git-goals:

\`\`\`bash
# Create a new goal
$ git goals create "Implement new feature"
Created new goal with ID: 20230811123456
Description: Implement new feature

# List all goals
$ git goals list
Current Goals:
- 20230811123456 (active): Implement new feature

# Show goal details
$ git goals show 20230811123456
{
  "id": "20230811123456",
  "type": "goal",
  "description": "Implement new feature",
  "status": "active",
  "created_at": "2023-08-11"
}

# Update a goal
$ git goals update 20230811123456 "Implement new feature with improved performance"
Updated goal 20230811123456: Implement new feature with improved performance

# Complete a goal
$ git goals complete 20230811123456 "" "Feature implemented and tested"
Goal 20230811123456 marked as complete
Rationale: Feature implemented and tested

# Generate a report
$ git goals report
Goal Report
===========
Goal ID: 20230811123456
Description: Implement new feature with improved performance
Status: completed
Created: 2023-08-11
Completed: 2023-08-11
---

# Delete a goal
$ git goals delete 20230811123456
Goal 20230811123456 deleted
\`\`\`

This example demonstrates the basic workflow of creating, updating, completing, and deleting a goal using git-goals.
EOL

echo "USAGE.md has been updated."

echo "All git-goals scripts and documentation have been updated."