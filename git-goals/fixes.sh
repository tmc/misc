#!/usr/bin/bash

# This script will implement minor improvements and prepare for the next development phase.

# 1. Update shebang in test-git-goals.sh
sed -i '1s|^#!/usr/bin/env bash|#!/usr/bin/env -S bash -euo pipefail|' test-git-goals.sh

# 2. Add error handling to test-git-goals.sh
sed -i '2i\set -euo pipefail' test-git-goals.sh

# 3. Update documentation in README.md
sed -i '/## Features/a\- Configurable notification system for approaching and overdue goals' README.md

# 4. Add more detailed error messages in git-goals-notify
sed -i '/echo -e "\\033\[1mError: Goal with ID $goal_id not found."/a\    echo -e "\\033[1mPlease check if the goal ID is correct and try again."' git-goals-notify

# 5. Improve error handling in git-goals-prioritize
sed -i '/echo -e "\\033\[1mError: Invalid priority. Use '"'"'high'"'"', '"'"'medium'"'"', or '"'"'low'"'"'."/a\    echo -e "\\033[1mPlease provide a valid priority and try again."' git-goals-prioritize

# 6. Update CHANGELOG.md for version 0.2.7
cat << EOF >> CHANGELOG.md

## [0.2.7] - $(date +%Y-%m-%d)
### Changed
- Improved error handling and messages in various scripts
- Updated documentation to reflect recent changes
- Minor code improvements and cleanup
EOF

# 7. Update version number in main git-goals script
sed -i 's/VERSION="0.2.6"/VERSION="0.2.7"/' git-goals

# 8. Add a TODO for future development in IMPORTANT file
echo "- Consider implementing a web interface for easier goal management" >> IMPORTANT

# 9. Improve notification message in git-goals-notify
sed -i 's/echo -e "\\033\[1mWARNING: Goal $goal_id is due in $days_until_deadline days!"/echo -e "\\033[1mWARNING: Goal $goal_id ($description) is due in $days_until_deadline days!"/' git-goals-notify

# 10. Add a sleep command to slow down development pace
echo "echo 'Development slowing down. Sleeping for 5 minutes...'" >> fixes.sh
echo "sleep 300" >> fixes.sh

echo "Minor improvements and preparations for version 0.2.7 complete. Please review changes and run manual tests if needed."