#!/usr/bin/bash

# Implement sorting of goals by priority in the list command
echo "Implementing priority sorting in git-goals-list..."
cat << 'EOF' > git-goals-list
#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/git-goals-common.sh"

load_config
NOTE_REF_NAME=${NOTE_REF_NAME:-goals}
DATE_FORMAT=${DATE_FORMAT:-%Y-%m-%d}
MAX_GOALS_DISPLAY=${MAX_GOALS_DISPLAY:-0}

set -euo pipefail
load_config
check_args "$@"

echo -e "\033[1mCurrent Goals:"

# Function to convert priority to numeric value for sorting
priority_to_number() {
    case "$1" in
        high) echo 3 ;;
        medium) echo 2 ;;
        low) echo 1 ;;
        *) echo 0 ;;
    esac
}

# Collect and sort goals
goals=$(git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=$(git notes --ref=goals show "$commit_hash")
    id=$(echo -e "$goal_data" | grep "^id:" | cut -d" " -f2-)
    status=$(echo -e "$goal_data" | grep "^status:" | cut -d" " -f2-)
    description=$(echo -e "$goal_data" | grep "^description:" | cut -d" " -f2-)
    priority=$(echo -e "$goal_data" | grep "^priority:" | cut -d" " -f2- || echo "Not set")
    deadline=$(echo -e "$goal_data" | grep "^deadline:" | cut -d" " -f2- || echo "Not set")
    priority_num=$(priority_to_number "$priority")
    echo "$priority_num|$id|$status|$priority|$deadline|$description"
done | sort -rn | cut -d'|' -f2-)

# Display sorted goals
echo "$goals" | while IFS='|' read -r id status priority deadline description; do
    echo -e "\033[1m- $id ($status, Priority: $priority, Deadline: $deadline): $description"
done | head -n ${MAX_GOALS_DISPLAY:-100}
EOF

chmod +x git-goals-list

# Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << 'EOF' >> CHANGELOG.md

## [0.2.9] - 2024-08-12
### Added
- Implemented sorting of goals by priority in the list command
### Changed
- Updated git-goals-list to sort goals by priority (high to low)
- Minor improvements and code cleanup
EOF

# Update version number in main git-goals script
echo "Updating version number in git-goals script..."
sed -i 's/VERSION="0.2.8"/VERSION="0.2.9"/' git-goals

# Update README.md
echo "Updating README.md..."
sed -i 's/git-goals v0.2.0/git-goals v0.2.9/' README.md

# Commit changes
echo "Committing changes..."
git add git-goals-list CHANGELOG.md git-goals README.md
git commit -m "Implement priority sorting in list command and prepare for version 0.2.9 release"

echo "All tasks completed. Sleeping for 10 minutes..."
sleep 600