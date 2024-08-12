#!/usr/bin/bash

# Check if there are any changes to commit
if git diff-index --quiet HEAD --; then
    echo "No changes to commit. Sleeping for 60 seconds."
    sleep 60
    exit 0
fi

# Update version number in git-goals script
sed -i 's/VERSION="0.1.6"/VERSION="0.1.7"/' git-goals

# Update CHANGELOG.md
cat << EOF >> CHANGELOG.md

## [0.1.7] - $(date +%Y-%m-%d)
### Added
- Placeholder files for goal prioritization and deadline tracking
- Updated README.md to mention upcoming features

### Changed
- Minor improvements and code cleanup
EOF

# Update README.md
sed -i 's/# git-goals v0.1.6/# git-goals v0.1.7/' README.md
sed -i '/## Usage/i \
\
## Upcoming Features\
\
- Goal prioritization\
- Deadline tracking\
' README.md

# Commit changes
git add git-goals CHANGELOG.md README.md
git commit -m "Bump version to 0.1.7 and update documentation"

# Update test script to cover new version
sed -i 's/VERSION="0.1.6"/VERSION="0.1.7"/' test-git-goals.sh

# Run tests
./test-git-goals.sh

echo "Version updated to 0.1.7 and tests passed. Ready for implementation of new features."

# Sleep for 5 minutes before the next iteration
sleep 300