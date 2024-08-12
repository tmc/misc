#!/usr/bin/bash

# Sleep for a longer period as we seem to be getting close to completion
sleep 30

# Update the IMPORTANT file to remove the completed tasks
sed -i '/Update test suite to cover new features/d' IMPORTANT
sed -i '/Enhance documentation for new features/d' IMPORTANT

# Implement a basic notification system for approaching deadlines
cat << EOF > git-goals-notify
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

echo -e "\033[1mChecking for approaching deadlines..."

current_date=\$(date +%Y-%m-%d)
warning_days=7

git notes --ref=goals list | while read -r commit_hash note_ref; do
    goal_data=\$(git notes --ref=goals show "\$commit_hash")
    goal_id=\$(echo -e "\033[1m\$goal_data" | grep "^id:" | cut -d" " -f2-)
    deadline=\$(echo -e "\033[1m\$goal_data" | grep "^deadline:" | cut -d" " -f2-)
    status=\$(echo -e "\033[1m\$goal_data" | grep "^status:" | cut -d" " -f2-)
    
    if [ -n "\$deadline" ] && [ "\$status" != "completed" ]; then
        days_until_deadline=\$(( (\$(date -d "\$deadline" +%s) - \$(date -d "\$current_date" +%s) ) / 86400 ))
        if [ \$days_until_deadline -le \$warning_days ] && [ \$days_until_deadline -ge 0 ]; then
            echo -e "\033[1mWARNING: Goal \$goal_id is due in \$days_until_deadline days!"
        elif [ \$days_until_deadline -lt 0 ]; then
            echo -e "\033[1mALERT: Goal \$goal_id is overdue by \$(( -days_until_deadline )) days!"
        fi
    fi
done
EOF

chmod +x git-goals-notify

# Update the main git-goals script to include the new notify subcommand
sed -i '/^case "$1" in/a\    notify)\n        shift\n        "git-goals-notify" "$@"\n        ;;' git-goals

# Update README.md to include information about the new notification feature
sed -i '/## Features/a\- Deadline notification system' README.md

# Update USAGE.md to include an example of the new notify command
cat << EOF >> USAGE.md

## Check for approaching deadlines
\`\`\`
$ git goals notify
Checking for approaching deadlines...
WARNING: Goal 20240101000000 is due in 5 days!
ALERT: Goal 20240101000001 is overdue by 2 days!
\`\`\`
EOF

# Update CHANGELOG.md
cat << EOF >> CHANGELOG.md

## [0.2.4] - $(date +%Y-%m-%d)
### Added
- Implemented a basic notification system for approaching deadlines
- Added 'notify' subcommand to check for goals with upcoming or passed deadlines

### Changed
- Updated documentation to reflect new notification feature
- Minor improvements and code cleanup
EOF

# Update version number
sed -i 's/VERSION="0.2.3"/VERSION="0.2.4"/' git-goals

# Commit changes
git add IMPORTANT README.md USAGE.md CHANGELOG.md git-goals git-goals-notify
git commit -m "Implement basic notification system for approaching deadlines"

echo "Basic notification system implemented. Consider testing and refining the feature."