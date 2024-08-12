#!/usr/bin/bash

# This script will perform the following tasks:
# 1. Clean up the codebase by removing unnecessary files
# 2. Update the README.md and USAGE.md files
# 3. Remove typos and improve consistency across scripts
# 4. Update the CHANGELOG.md file
# 5. Remove the "meta-improvement" tools

# 1. Clean up the codebase
echo "Cleaning up the codebase..."
rm -f make-fix-suggestion.sh
rm -f THOUGHTS
rm -f eval-make-safe-progress.sh
rm -f make-safe-progress.sh
rm -rf .tools

# 2. Update README.md and USAGE.md
echo "Updating README.md and USAGE.md..."
sed -i 's/v0.1.4/v0.1.5/' README.md
cat << EOF > USAGE.md
# git-goals Usage Guide

## Installation

1. Clone the repository:
   \`\`\`
   git clone https://github.com/yourusername/git-goals.git
   \`\`\`
2. Add the git-goals directory to your PATH:
   \`\`\`
   export PATH=\$PATH:/path/to/git-goals
   \`\`\`
3. Ensure all scripts are executable:
   \`\`\`
   chmod +x /path/to/git-goals/git-goals*
   \`\`\`

## Basic Usage

### Create a new goal
\`\`\`
git goals create "Implement new feature"
\`\`\`

### List all goals
\`\`\`
git goals list
\`\`\`

### Show goal details
\`\`\`
git goals show <goal_id>
\`\`\`

### Update a goal
\`\`\`
git goals update <goal_id> "Updated goal description"
\`\`\`

### Complete a goal
\`\`\`
git goals complete <goal_id> [attempt_id] [rationale]
\`\`\`

### Delete a goal
\`\`\`
git goals delete <goal_id>
\`\`\`

### Generate a report
\`\`\`
git goals report
\`\`\`

### Recover goals
\`\`\`
git goals recover
\`\`\`

For more detailed information on each command, use the --help option:
\`\`\`
git goals <command> --help
\`\`\`
EOF

# 3. Remove typos and improve consistency
echo "Removing typos and improving consistency..."
sed -i 's/nload_config/load_config/' git-goals-*
sed -i 's/fxing/fixing/' .claude-system-prompt

# 4. Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << EOF >> CHANGELOG.md

## [0.1.5] - $(date +%Y-%m-%d)
### Changed
- Removed unnecessary "meta-improvement" tools
- Updated README.md and USAGE.md for clarity and completeness
- Improved consistency across all scripts
- Cleaned up codebase by removing unused files
EOF

# 5. Update version number in git-goals script
sed -i 's/VERSION="0.1.4"/VERSION="0.1.5"/' git-goals

# 6. Remove IMPORTANT file as its tasks have been completed
rm -f IMPORTANT

echo "Cleanup and updates completed. Please review the changes and commit them if satisfactory."