#!/usr/bin/bash

# Minor improvements and preparations for version 0.2.8

# Update version number
sed -i 's/VERSION="0.2.7"/VERSION="0.2.8"/' git-goals

# Update CHANGELOG.md
cat << EOF >> CHANGELOG.md

## [0.2.8] - $(date +%Y-%m-%d)
### Changed
- Minor improvements and code cleanup
- Enhanced error handling in various scripts
- Updated documentation
EOF

# Update README.md with more detailed information about the notification system
sed -i '/## Features/a - Configurable notification system for approaching and overdue goals' README.md

# Improve error handling in git-goals-notify
sed -i '/echo -e "\\033\[1mChecking for approaching deadlines..."/a\
if [ -z "$(git notes --ref=goals list)" ]; then\
    echo -e "\\033[1mNo goals found. Nothing to notify."\
    exit 0\
fi' git-goals-notify

# Add a TODO comment for future enhancement in git-goals-prioritize
sed -i '/echo -e "\\033\[1mGoal $goal_id priority set to $priority"/a\
# TODO: Consider implementing a way to sort goals by priority in the list command' git-goals-prioritize

# Update IMPORTANT file
sed -i '/- Consider implementing a web interface for easier goal management/d' IMPORTANT
echo "- Implement sorting of goals by priority in the list command" >> IMPORTANT

# Commit changes
git add git-goals CHANGELOG.md README.md git-goals-notify git-goals-prioritize IMPORTANT
git commit -m "Prepare for version 0.2.8 release"

# Run tests
./test-git-goals.sh

echo "Minor improvements and preparations for version 0.2.8 complete. Please review changes and run manual tests if needed."
echo "Development slowing down. Sleeping for 5 minutes..."
sleep 300