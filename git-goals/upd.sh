#!/bin/bash

# Script to update all git-notes-* scripts

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

echo "All git-goals-* scripts have been updated."
