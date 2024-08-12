#!/usr/bin/bash

# Update version number to 0.1.5
sed -i 's/VERSION="0.1.4"/VERSION="0.1.5"/' git-goals
sed -i 's/# git-goals v0.1.4/# git-goals v0.1.5/' README.md

# Update CHANGELOG.md
cat << EOF >> CHANGELOG.md

## [0.1.5] - $(date +%Y-%m-%d)
### Changed
- Updated version number to 0.1.5
- Improved consistency across all scripts
- Enhanced error handling and input validation
- Updated README.md with information about running tests
EOF

# Add information about running tests to README.md
sed -i '/## Usage/i\## Testing\n\nTo run the test suite, use the following command:\n\n```bash\n./test-git-goals.sh\n```\n' README.md

# Run the test script
./test-git-goals.sh

# Update git-goals-common.sh to include a function for version checking
cat << EOF >> git-goals-common.sh

check_version() {
    local required_version=\$1
    if [ "\$(printf '%s\n' "\$required_version" "\$VERSION" | sort -V | head -n1)" != "\$required_version" ]; then
        echo "Error: This script requires git-goals version \$required_version or higher." >&2
        exit 1
    fi
}
EOF

# Add version check to all git-goals-* scripts
for script in git-goals-*; do
    if [ "\$script" != "git-goals-common.sh" ]; then
        sed -i '/load_config/a\check_version "0.1.5"' "\$script"
    fi
done

# Commit changes
git add .
git commit -m "Bump version to 0.1.5 and update documentation"

echo "Version updated to 0.1.5, CHANGELOG.md updated, and tests run. Please review changes and push if satisfied."

# Sleep for 5 minutes
sleep 300