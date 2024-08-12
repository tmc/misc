#!/usr/bin/bash

# Since we've just implemented the notification system, let's focus on testing and refining it.

# 1. Update the test suite to include tests for the new notification feature
echo "Updating test suite..."
cat << EOF >> test-git-goals.sh

# Test notification system
echo "Testing notification system..."
test_goal_id=\$(git goals create "Test goal with deadline")
git goals deadline \$test_goal_id "2023-12-31"
notification_output=\$(git goals notify)
if [[ \$notification_output == *"WARNING: Goal \$test_goal_id is due"* ]]; then
    echo "Notification test passed"
else
    echo "Notification test failed"
    exit 1
fi

# Clean up test goal
git goals delete \$test_goal_id
EOF

# 2. Add more detailed documentation for the notification feature
echo "Updating README.md with notification feature details..."
sed -i '/## Features/a - Deadline notification system for approaching and overdue goals' README.md

# 3. Refine the notification system
echo "Refining the notification system..."
sed -i 's/warning_days=7/warning_days=${WARNING_DAYS:-7}/' git-goals-notify
sed -i '1a\WARNING_DAYS=${WARNING_DAYS:-7}' git-goals-notify

# 4. Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << EOF >> CHANGELOG.md

## [0.2.5] - $(date +%Y-%m-%d)
### Added
- Enhanced notification system with configurable warning period
- Updated test suite to include notification system tests
### Changed
- Improved documentation for the notification feature
EOF

# 5. Update version number
sed -i 's/VERSION="0.2.4"/VERSION="0.2.5"/' git-goals

# 6. Run tests
echo "Running tests..."
./test-git-goals.sh

# 7. Commit changes
git add test-git-goals.sh README.md git-goals-notify CHANGELOG.md git-goals
git commit -m "Refine notification system and update documentation"

echo "Improvements completed. Please review the changes and run manual tests if needed."

# Sleep for a short period to avoid unnecessary compute
sleep 5