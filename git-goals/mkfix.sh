./make-fix-suggestion.sh "Improve git-goals-complete script"

# Improving git-goals-complete script
<antthinking>
1. The current script lacks input validation for the goal_id format
2. There's no confirmation prompt before marking a goal as complete
3. The script doesn't handle cases where the goal is already completed
4. Error messages could be more informative
5. The script could benefit from better formatting of the updated goal data
</antthinking>

#!/bin/bash

cat << 'EOF' > fixes.sh
#!/bin/bash

# Function to improve git-goals-complete script
improve_git_goals_complete() {
    cat << 'EOT' > git-goals-complete
#!/bin/bash
# git-goals-complete - Marks a goal as complete
set -euo pipefail

usage() {
    echo "Usage: git goals complete <goal_id> [attempt_id] [rationale]"
    echo "  <goal_id>    : ID of the goal to mark as complete"
    echo "  [attempt_id] : Optional ID of the attempt"
    echo "  [rationale]  : Optional rationale for completion"
    exit 1
}

if [ $# -lt 1 ]; then
    usage
fi

goal_id="$1"
attempt_id="${2:-}"
rationale="${3:-}"

# Validate goal_id format (assuming it's a timestamp format YYYYMMDDHHMMSS)
if ! [[ $goal_id =~ ^[0-9]{14}$ ]]; then
    echo "Error: Invalid goal ID format. Expected YYYYMMDDHHMMSS." >&2
    exit 1
fi

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found." >&2
    exit 1
fi

# Get the current goal data
current_data=$(git notes --ref=goals show "$commit_hash")

# Check if the goal is already completed
if echo "$current_data" | grep -q "^status: completed"; then
    echo "Error: Goal $goal_id is already marked as complete." >&2
    exit 1
fi

# Prompt for confirmation
read -p "Are you sure you want to mark goal $goal_id as complete? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 0
fi

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

# Display updated goal information
echo -e "\nUpdated goal information:"
git notes --ref=goals show "$commit_hash"
EOT

    chmod +x git-goals-complete
    echo "git-goals-complete script has been improved and updated."
}

# Run the improvement function
improve_git_goals_complete

echo "Fixes have been applied. Please review the changes in git-goals-complete."
EOF

chmod +x fixes.sh

echo "Fix suggestion script has been created. Run ./fixes.sh to apply the improvements to git-goals-complete."