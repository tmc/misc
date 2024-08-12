#!/usr/bin/bash

# Fix shebang issues
find . -type f -name 'git-goals*' -exec sed -i '1s|^#!/usr/bin/bash|#!/usr/bin/env bash|' {} +
find . -type f -name 'test-git-goals.sh' -exec sed -i '1s|^#!/bin/bash|#!/usr/bin/env bash|' {} +

# Update version number in git-goals script
sed -i 's/VERSION="0.1.7"/VERSION="0.1.8"/' git-goals

# Update README.md with new version number
sed -i 's/# git-goals v0.1.7/# git-goals v0.1.8/' README.md

# Update CHANGELOG.md with new version
cat << EOF >> CHANGELOG.md

## [0.1.8] - $(date +%Y-%m-%d)
### Changed
- Fixed shebang issues for better cross-platform compatibility
- Minor improvements and bug fixes
EOF

# Run tests to ensure everything is working
./test-git-goals.sh

# If tests pass, commit changes
if [ $? -eq 0 ]; then
    git add .
    git commit -m "Update to version 0.1.8 and fix shebang issues"
    echo "Updated to version 0.1.8 and fixed shebang issues. Commit created."
else
    echo "Tests failed. Please review and fix any issues before committing."
fi

# Sleep for a longer period as we're getting close to completion
sleep 300