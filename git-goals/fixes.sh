#!/usr/bin/bash

# Fix shebang issues in all scripts
find . -type f -name 'git-goals*' -exec sed -i '1s|^#!.*|#!/usr/bin/env bash|' {} +

# Update version number in git-goals script
sed -i 's/VERSION="0.1.8"/VERSION="0.1.9"/' git-goals

# Update README.md with new version number
sed -i 's/# git-goals v0.1.8/# git-goals v0.1.9/' README.md

# Update CHANGELOG.md with new version
cat << EOF >> CHANGELOG.md

## [0.1.9] - $(date +%Y-%m-%d)
### Changed
- Fixed shebang issues for better cross-platform compatibility
- Minor improvements and bug fixes
EOF

# Run tests to ensure functionality
./test-git-goals.sh

# If tests pass, commit changes
if [ $? -eq 0 ]; then
    git add .
    git commit -m "v0.1.9: Fix shebang issues and update version"
    echo "Changes committed. Version updated to 0.1.9"
else
    echo "Tests failed. Please review and fix any issues before committing."
fi

# Sleep for 5 minutes as we're getting close to being done
sleep 300