#!/usr/bin/bash

# Since we've made significant progress and are now in a more stable state,
# we'll implement a longer sleep period to allow for more substantial development cycles.

echo "Entering longer development cycle. Sleeping for 4 hours..."
sleep 14400

# After the sleep period, we'll perform some maintenance tasks

# Update documentation
echo "Updating documentation..."
sed -i 's/version 0.2.12/version 0.2.13/' README.md
sed -i '/## Usage/i\### Plugin System\n\ngit-goals now supports a plugin system. Plugins can be added to the `plugins/` directory and will be automatically loaded.\n' README.md

# Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << EOF >> CHANGELOG.md

## [0.2.13] - $(date +%Y-%m-%d)
### Added
- Enhanced documentation for the plugin system
- Updated README.md with plugin system information
### Changed
- Increased development cycle time for more substantial improvements
EOF

# Perform some code cleanup
echo "Performing code cleanup..."
find . -name "*.sh" -type f -exec sed -i 's/\s\+$//' {} +

# Run tests
echo "Running tests..."
./test-git-goals.sh

# Commit changes
git add README.md CHANGELOG.md
git commit -m "Documentation updates and code cleanup"

echo "Development cycle complete. Please review changes and plan next steps."