./make-fix-suggestion.sh "Improve error handling and add more detailed usage instructions for git-goals scripts"

Here's a suggested fix script to improve error handling and add more detailed usage instructions for the git-goals scripts:

```bash
#!/bin/bash

# Function to update a script with improved error handling and usage instructions
update_script() {
  local script_name=$1
  local usage=$2
  local content=$3

  echo "Updating $script_name..."
  cat > "$script_name" << EOF
#!/bin/bash
# $script_name - $usage
set -euo pipefail

usage() {
    echo "Usage: $usage"
    echo
    echo "Options:"
    echo "  -h, --help    Show this help message"
}

# Parse command-line options
while [[ \$# -gt 0 ]]; do
    case \$1 in
        -h|--help)
            usage
            exit 0
            ;;
        *)
            break
            ;;
    esac
done

$content

EOF
  chmod +x "$script_name"
}

# Update git-goals-show
update_script "git-goals-show" "git goals show <goal_id>" '
if [ $# -eq 0 ]; then
    echo "Error: Goal ID is required." >&2
    usage
    exit 1
fi

goal_id="$1"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found." >&2
    exit 1
fi

# Display the goal details
git notes --ref=goals show "$commit_hash" || {
    echo "Error: Failed to retrieve goal details." >&2
    exit 1
}
'

# Update git-goals-delete
update_script "git-goals-delete" "git goals delete <goal_id>" '
if [ $# -eq 0 ]; then
    echo "Error: Goal ID is required." >&2
    usage
    exit 1
fi

goal_id="$1"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found." >&2
    exit 1
fi

# Remove the git note for the goal
if git notes --ref=goals remove "$commit_hash"; then
    echo "Goal $goal_id deleted"
else
    echo "Error: Failed to delete goal $goal_id" >&2
    exit 1
fi
'

# Update git-goals-update
update_script "git-goals-update" "git goals update <goal_id> <new_description>" '
if [ $# -lt 2 ]; then
    echo "Error: Goal ID and new description are required." >&2
    usage
    exit 1
fi

goal_id="$1"
shift
new_description="$*"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found." >&2
    exit 1
fi

# Get the current goal data
current_data=$(git notes --ref=goals show "$commit_hash")

# Update the description
updated_data=$(echo "$current_data" | sed "s/^description:.*$/description: $new_description/")

# Update the git note
if echo "$updated_data" | git notes --ref=goals add -f -F - "$commit_hash"; then
    echo "Updated goal $goal_id: $new_description"
else
    echo "Error: Failed to update goal $goal_id" >&2
    exit 1
fi
'

# Update git-goals-complete
update_script "git-goals-complete" "git goals complete <goal_id> [attempt_id] [rationale]" '
if [ $# -lt 1 ]; then
    echo "Error: Goal ID is required." >&2
    usage
    exit 1
fi

goal_id="$1"
attempt_id="${2:-}"
rationale="${3:-}"

# Find the commit hash for the given goal ID
commit_hash=$(git notes --ref=goals list | grep "$goal_id" | awk "{print \$1}")

if [ -z "$commit_hash" ]; then
    echo "Error: Goal with ID $goal_id not found." >&2
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
if echo "$updated_data" | git notes --ref=goals add -f -F - "$commit_hash"; then
    echo "Goal $goal_id marked as complete"
    if [ -n "$rationale" ]; then
        echo "Rationale: $rationale"
    fi
else
    echo "Error: Failed to mark goal $goal_id as complete" >&2
    exit 1
fi
'

echo "All git-goals scripts have been updated with improved error handling and usage instructions."
```

This fix script does the following:

1. Defines a function `update_script` that takes a script name, usage description, and content as arguments.
2. For each git-goals script (show, delete, update, and complete), it calls the `update_script` function with improved content.
3. The improved content for each script includes:
   - A usage function that displays detailed usage instructions
   - Command-line option parsing for -h/--help
   - Improved error handling with more informative error messages
   - Proper exit codes for error conditions
   - More robust checks for required arguments

These changes will make the git-goals scripts more user-friendly and resilient to errors. Users will now have access to detailed usage instructions for each command, and the scripts will provide more informative error messages when something goes wrong.

<antthinking>
1. The improvements focus on user experience and error handling, which are crucial for command-line tools.
2. By adding usage instructions and better error messages, we're making the tools more self-documenting and easier to use.
3. The use of exit codes will make it easier for other scripts or programs to interact with these tools.
4. We're maintaining consistency across all the scripts by using a common update function.
5. The changes respect the existing functionality while enhancing robustness and usability.
6. This approach allows for easy maintenance and future updates to the scripts.
7. We could consider adding more advanced features like verbose output options in the future.
8. It might be beneficial to add unit tests for these scripts to ensure they behave correctly under various conditions.
9. We should consider updating the main README.md file to reflect these improvements in usage instructions.
10. In the future, we might want to implement a common library for shared functions across these scripts to reduce code duplication.
</antthinking>