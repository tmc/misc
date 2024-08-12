#!/usr/bin/bash

# Since the codebase appears to be in a stable and well-organized state, 
# we'll focus on testing and minor improvements.

# Run the test script to ensure all functionality is working as expected
echo "Running test script..."
./test-git-goals.sh

# Update version number in README.md to match the current version
echo "Updating version number in README.md..."
sed -i 's/# git-goals v[0-9]\.[0-9]\.[0-9]/# git-goals v0.1.5/' README.md

# Ensure all scripts have the correct shebang
echo "Checking and updating shebangs..."
for file in git-goals*; do
    if [[ -f "$file" && "$file" != "git-goals-common.sh" ]]; then
        sed -i '1s|^#!.*|#!/usr/bin/bash|' "$file"
    fi
done

# Add a note about running tests in the README
echo "Adding test information to README.md..."
if ! grep -q "## Testing" README.md; then
    echo -e "\n## Testing\n\nTo run the test suite, execute:\n\n\`\`\`\n./test-git-goals.sh\n\`\`\`" >> README.md
fi

# Ensure .gitignore includes common files to ignore
echo "Updating .gitignore..."
cat << EOF >> .gitignore

# Ignore backup files
*~

# Ignore Visual Studio Code settings
.vscode/

# Ignore any local test files
test_*.sh
EOF

# Sleep for a while as we're getting close to being done
echo "Sleeping for 60 seconds..."
sleep 60

echo "Script completed. Please review changes and commit if satisfactory."