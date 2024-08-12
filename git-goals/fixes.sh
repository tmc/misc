#!/usr/bin/bash

# Implement goal prioritization feature
echo "Implementing goal prioritization feature..."
cat << EOF > git-goals-prioritize
#!/usr/bin/env bash

set -euo pipefail

source "\$(dirname "\$0")/git-goals-common.sh"

load_config
NOTE_REF_NAME=\${NOTE_REF_NAME:-goals}
DATE_FORMAT=\${DATE_FORMAT:-%Y-%m-%d}
MAX_GOALS_DISPLAY=\${MAX_GOALS_DISPLAY:-0}

set -euo pipefail
load_config
check_args "\$@"

if [ \$# -lt 2 ]; then
    echo -e "\033[1mUsage: git goals prioritize <goal_id> <priority>"
    echo -e "\033[1mPriority can be: high, medium, low"
    exit 1
fi

goal_id="\$1"
priority="\$2"

if [[ ! "\$priority" =~ ^(high|medium|low)$ ]]; then
    echo -e "\033[1mError: Invalid priority. Use 'high', 'medium', or 'low'."
    exit 1
fi

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo -e "\033[1mError: Goal with ID \$goal_id not found."
    exit 1
fi

current_data=\$(git notes --ref=goals show "\$commit_hash")
updated_data=\$(echo -e "\033[1m\$current_data" | sed "/^priority:/d")
updated_data+="\npriority: \$priority"

echo -e "\033[1m\$updated_data" | git notes --ref=goals add -f -F - "\$commit_hash"

echo -e "\033[1mGoal \$goal_id priority set to \$priority"
EOF

chmod +x git-goals-prioritize

# Update git-goals script to include prioritize subcommand
sed -i 's/prioritize|deadline)/prioritize|deadline)\n        subcommand="$1"\n        shift\n        "git-goals-$subcommand" "$@"\n        ;;/' git-goals

# Update git-goals-list to include priority information
sed -i '/description=$(echo -e "\\033\[1m$goal_data" | grep "^description:" | cut -d" " -f2-)/a\    priority=$(echo -e "\\033[1m$goal_data" | grep "^priority:" | cut -d" " -f2- || echo "Not set")' git-goals-list
sed -i 's/echo -e "\\033\[1m- $id ($status): $description"/echo -e "\\033[1m- $id ($status, Priority: $priority): $description"/' git-goals-list

# Update git-goals-report to include priority information
sed -i '/completed_at=$(echo -e "\\033\[1m$goal_data" | grep "^completed_at:" | cut -d" " -f2-)/a\    priority=$(echo -e "\\033[1m$goal_data" | grep "^priority:" | cut -d" " -f2- || echo "Not set")' git-goals-report
sed -i '/echo -e "\\033\[1mStatus: $status"/a\    echo -e "\\033[1mPriority: $priority"' git-goals-report

# Update README.md with prioritize command
sed -i '/### Complete a goal/i\### Prioritize a goal\n\n```\ngit goals prioritize <goal_id> <priority>\n```\n' README.md

# Update USAGE.md with prioritize command example
sed -i '/## Delete a goal/i\## Prioritize a goal\n```\n$ git goals prioritize 20240101000000 high\nGoal 20240101000000 priority set to high\n```\n' USAGE.md

# Update test-git-goals.sh to include prioritize test
sed -i '/# Test goal completion/i\# Test goal prioritization\nrun_command git goals prioritize "$goal_id" "high"\n\n# Test goal show after prioritization\nrun_command git goals show "$goal_id"\n' test-git-goals.sh

# Update CHANGELOG.md
echo "
## [0.2.1] - $(date +%Y-%m-%d)
### Added
- Goal prioritization feature
- Updated list and report commands to show priority information
- Updated documentation and test suite for prioritization feature" >> CHANGELOG.md

# Update version number
sed -i 's/VERSION="0.2.0"/VERSION="0.2.1"/' git-goals

echo "Goal prioritization feature implemented. Moving on to deadline tracking..."

# Implement deadline tracking feature
echo "Implementing deadline tracking feature..."
cat << EOF > git-goals-deadline
#!/usr/bin/env bash

set -euo pipefail

source "\$(dirname "\$0")/git-goals-common.sh"

load_config
NOTE_REF_NAME=\${NOTE_REF_NAME:-goals}
DATE_FORMAT=\${DATE_FORMAT:-%Y-%m-%d}
MAX_GOALS_DISPLAY=\${MAX_GOALS_DISPLAY:-0}

set -euo pipefail
load_config
check_args "\$@"

if [ \$# -lt 2 ]; then
    echo -e "\033[1mUsage: git goals deadline <goal_id> <deadline>"
    echo -e "\033[1mDeadline format: YYYY-MM-DD"
    exit 1
fi

goal_id="\$1"
deadline="\$2"

if ! date -d "\$deadline" &>/dev/null; then
    echo -e "\033[1mError: Invalid date format. Use YYYY-MM-DD."
    exit 1
fi

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo -e "\033[1mError: Goal with ID \$goal_id not found."
    exit 1
fi

current_data=\$(git notes --ref=goals show "\$commit_hash")
updated_data=\$(echo -e "\033[1m\$current_data" | sed "/^deadline:/d")
updated_data+="\ndeadline: \$deadline"

echo -e "\033[1m\$updated_data" | git notes --ref=goals add -f -F - "\$commit_hash"

echo -e "\033[1mGoal \$goal_id deadline set to \$deadline"
EOF

chmod +x git-goals-deadline

# Update git-goals-list to include deadline information
sed -i '/priority=$(echo -e "\\033\[1m$goal_data" | grep "^priority:" | cut -d" " -f2- || echo "Not set")/a\    deadline=$(echo -e "\\033[1m$goal_data" | grep "^deadline:" | cut -d" " -f2- || echo "Not set")' git-goals-list
sed -i 's/echo -e "\\033\[1m- $id ($status, Priority: $priority): $description"/echo -e "\\033[1m- $id ($status, Priority: $priority, Deadline: $deadline): $description"/' git-goals-list

# Update git-goals-report to include deadline information
sed -i '/priority=$(echo -e "\\033\[1m$goal_data" | grep "^priority:" | cut -d" " -f2- || echo "Not set")/a\    deadline=$(echo -e "\\033[1m$goal_data" | grep "^deadline:" | cut -d" " -f2- || echo "Not set")' git-goals-report
sed -i '/echo -e "\\033\[1mPriority: $priority"/a\    echo -e "\\033[1mDeadline: $deadline"' git-goals-report

# Update README.md with deadline command
sed -i '/### Prioritize a goal/a\### Set a deadline for a goal\n\n```\ngit goals deadline <goal_id> <deadline>\n```\n' README.md

# Update USAGE.md with deadline command example
sed -i '/## Prioritize a goal/a\## Set a deadline for a goal\n```\n$ git goals deadline 20240101000000 2024-12-31\nGoal 20240101000000 deadline set to 2024-12-31\n```\n' USAGE.md

# Update test-git-goals.sh to include deadline test
sed -i '/# Test goal prioritization/a\# Test setting goal deadline\nrun_command git goals deadline "$goal_id" "2024-12-31"\n\n# Test goal show after setting deadline\nrun_command git goals show "$goal_id"\n' test-git-goals.sh

# Update CHANGELOG.md
echo "
## [0.2.2] - $(date +%Y-%m-%d)
### Added
- Deadline tracking feature
- Updated list and report commands to show deadline information
- Updated documentation and test suite for deadline feature" >> CHANGELOG.md

# Update version number
sed -i 's/VERSION="0.2.1"/VERSION="0.2.2"/' git-goals

echo "Deadline tracking feature implemented."

# Update IMPORTANT file
sed -i '1,2d' IMPORTANT

echo "Implementation of goal prioritization and deadline tracking features is complete."
echo "Sleeping for 60 seconds before the next iteration..."
sleep 60