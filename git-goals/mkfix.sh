./make-fix-suggestion.sh "#!/bin/bash
# <antthinking>
# 1. We need to address the sandbox-exec command not being found issue.
# 2. The codebase needs more comprehensive error handling and input validation.
# 3. We should implement the missing git-goals-list script.
# 4. The README and USAGE files need to be updated to reflect all available commands.
# 5. We should consider adding more detailed logging or verbose output options.
# 6. The test script reveals some gaps in functionality that need to be addressed.
# 7. We should ensure consistent formatting and error handling across all scripts.
# 8. The git-goals script needs to be implemented as the main entry point.
# 9. We should consider adding a git-goals-update script for updating existing goals.
# 10. The overall structure of the project could be improved for better organization.
# </antthinking>

# Function to create a new script with proper permissions
create_script() {
    local script_name=\"$1\"
    local content=\"$2\"
    echo \"$content\" > \"$script_name\"
    chmod +x \"$script_name\"
}

# Create git-goals-list script
create_script \"git-goals-list\" '#!/bin/bash
# git-goals-list - Lists all goals
set -euo pipefail

echo \"Current Goals:\"

# Check if there are any notes
if ! git notes --ref=goals list > /dev/null 2>&1; then
    echo \"No goals found.\"
    exit 0
fi

git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show \"$commit_hash\" 2>/dev/null)

    # Check if it'\"'\"'s the old format (contains '\"'\"'tree'\"'\"' and '\"'\"'parent'\"'\"')
    if echo \"$goal_data\" | grep -q \"tree\"; then
        description=$(echo \"$goal_data\" | grep \"Goal:\" | sed '\"'\"'s/Goal: //'\"'\"')
        if [ -z \"$description\" ]; then
            echo \"Warning: No goal description found for commit $commit_hash\"
            continue
        fi
        echo \"- $commit_hash (old format): $description\"
    else
        goal_id=$(echo \"$goal_data\" | grep \"^id:\" | cut -d'\"'\"' '\"'\"' -f2-)
        description=$(echo \"$goal_data\" | grep \"^description:\" | cut -d'\"'\"' '\"'\"' -f2-)
        status=$(echo \"$goal_data\" | grep \"^status:\" | cut -d'\"'\"' '\"'\"' -f2-)

        if [ -z \"$goal_id\" ] || [ -z \"$description\" ] || [ -z \"$status\" ]; then
            echo \"Warning: Incomplete goal data for commit $commit_hash\"
            continue
        fi

        echo \"- $commit_hash $goal_id ($status): $description\"
    fi
done'

# Create git-goals main script
create_script \"git-goals\" '#!/bin/bash
# git-goals - Main entry point for git-goals commands

set -euo pipefail

SCRIPT_DIR=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")\" && pwd)\"

usage() {
    echo \"Usage: git goals <command> [<args>]\"
    echo
    echo \"Available commands:\"
    echo \"  create    Create a new goal\"
    echo \"  update    Update an existing goal or create a new one\"
    echo \"  list      List all goals\"
    echo \"  show      Show details of a specific goal\"
    echo \"  complete  Mark a goal as complete\"
    echo \"  delete    Delete a goal\"
    echo \"  report    Generate a report of all goals\"
    echo
    echo \"Run '\"'\"'git goals <command> --help'\"'\"' for more information on a specific command.\"
}

if [ $# -eq 0 ]; then
    usage
    exit 1
fi

command=\"$1\"
shift

case \"$command\" in
    create|update|list|show|complete|delete|report)
        exec \"$SCRIPT_DIR/git-goals-$command\" \"$@\"
        ;;
    --help|-h|help)
        usage
        exit 0
        ;;
    *)
        echo \"Error: Unknown command '\"'\"'$command'\"'\"'\" >&2
        usage
        exit 1
        ;;
esac'

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

This creates a new goal branch with the given description. If you're already on a goal branch, it creates a subgoal.

### Update a goal

\`\`\`
git goals update <new_goal_description>
\`\`\`

Updates the current goal or creates a new one if it doesn't exist.

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

git-goals uses Git notes to store goal metadata. Each goal is associated with a specific commit, and the goal information is stored as a note on that commit. The tools provided allow you to manage these notes and the associated goal branches easily.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
EOL

# Update USAGE.md
cat > USAGE.md << EOL
## Example Usage

Here's an example of how to use git-goals:

\`\`\`bash
# Create a new goal
$ git goals create \"Implement new feature\"
Created new goal with ID: 20230811123456
Description: Implement new feature

# List all goals
$ git goals list
Current Goals:
- 20230811123456 (active): Implement new feature

# Show goal details
$ git goals show 20230811123456
{
  \"id\": \"20230811123456\",
  \"type\": \"goal\",
  \"description\": \"Implement new feature\",
  \"status\": \"active\",
  \"created_at\": \"2023-08-11\"
}

# Update a goal
$ git goals update 20230811123456 \"Implement new feature with improved performance\"
Updated goal 20230811123456: Implement new feature with improved performance

# Complete a goal
$ git goals complete 20230811123456 \"\" \"Feature implemented and tested\"
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

# Update test-git-goals.sh
cat > test-git-goals.sh << EOL
#!/bin/bash
set -euo pipefail

# Function to run a command and print its output
run_command() {
    echo \"$ \$@\"
    output=$(\"\$@\")
    echo \"\$output\"
    echo
}

export PATH=\"\$(pwd):\$PATH\"

# Set up a temporary test directory
test_dir=\$(mktemp -d)
cd \"\$test_dir\"

echo \"Setting up test repository...\"
git init
git config user.email \"test@example.com\"
git config user.name \"Test User\"
git commit --allow-empty -m \"Initial commit\"


echo \"Testing git-goals...\"

# Test goal creation
run_command git goals create \"Implement new feature\"

# Test goal listing
run_command git goals list

# Get the goal ID from the list output
goal_id=\$(git goals list | grep \"Implement new feature\" | sed -E 's/- ([^ ]+) .*/\\1/')

if [ -z \"\$goal_id\" ]; then
    echo \"Error: Failed to extract goal ID for 'Implement new feature'\"
    exit 1
fi

# Test goal show
run_command git goals show \"\$goal_id\"

# Test goal update
run_command git goals update \"\$goal_id\" \"Implement new feature with improved performance\"

# Test goal show after update
run_command git goals show \"\$goal_id\"

# Test goal completion
run_command git goals complete \"\$goal_id\" \"\" \"Feature implemented and tested\"

# Test goal report
run_command git goals report

# Test goal deletion
run_command git goals delete \"\$goal_id\"

# Verify goal is deleted
if git goals list | grep -q \"\$goal_id\"; then
    echo \"Error: Goal \$goal_id still exists after deletion\"
    exit 1
else
    echo \"Goal \$goal_id successfully deleted\"
fi

echo \"All tests completed successfully!\"

# Clean up
cd ..
rm -rf \"\$test_dir\"
EOL

# Create a script to fix the sandbox-exec issue
create_script \"fix-sandbox-exec.sh\" '#!/bin/bash
set -euo pipefail

# Check if sandbox-exec is in the PATH
if ! command -v sandbox-exec &> /dev/null; then
    echo \"sandbox-exec not found in PATH. Attempting to fix...\"
    
    # Try to find sandbox-exec
    sandbox_exec_path=$(find /usr/local/bin /usr/bin /bin -name sandbox-exec 2>/dev/null | head -n 1)
    
    if [ -n \"$sandbox_exec_path\" ]; then
        echo \"Found sandbox-exec at $sandbox_exec_path\"
        echo \"Adding its directory to PATH...\"
        export PATH=\"$(dirname \"$sandbox_exec_path\"):$PATH\"
        echo \"PATH updated. Please run '\"'\"'source ~/.bashrc'\"'\"' or restart your terminal.\"
    else
        echo \"Could not find sandbox-exec. Please install it or add its location to your PATH manually.\"
        exit 1
    fi
else
    echo \"sandbox-exec is already in PATH.\"
fi

# Update make-safe-progress.sh to use the correct sandbox-exec
sed -i '\"'\"'s/sandbox-exec/command sandbox-exec/g'\"'\"' make-safe-progress.sh

echo \"Fixed sandbox-exec references in make-safe-progress.sh\"'

# Make the fix script executable
chmod +x fix-sandbox-exec.sh

echo \"Created fix-sandbox-exec.sh to address the sandbox-exec issue.\"
echo \"Please run './fix-sandbox-exec.sh' to attempt to fix the sandbox-exec problem.\"
echo \"After running the fix script, you may need to restart your terminal or run 'source ~/.bashrc'.\"

echo \"All scripts have been updated or created. Please review the changes and test the functionality.\""