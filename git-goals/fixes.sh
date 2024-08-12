#!/usr/bin/bash

# Update test suite for priority sorting feature
echo "Updating test-git-goals.sh to include priority sorting tests..."
cat << EOT >> test-git-goals.sh

# Test priority sorting in list command
echo "Testing priority sorting in list command..."
high_priority_goal=$(git goals create "High priority goal")
git goals prioritize \$high_priority_goal high
medium_priority_goal=$(git goals create "Medium priority goal")
git goals prioritize \$medium_priority_goal medium
low_priority_goal=$(git goals create "Low priority goal")
git goals prioritize \$low_priority_goal low

sorted_list=$(git goals list)
if [[ "\$sorted_list" == *"High priority goal"*"Medium priority goal"*"Low priority goal"* ]]; then
    echo "Priority sorting test passed"
else
    echo "Priority sorting test failed"
    exit 1
fi

# Clean up test goals
git goals delete \$high_priority_goal
git goals delete \$medium_priority_goal
git goals delete \$low_priority_goal
EOT

# Update README.md to reflect priority sorting feature
echo "Updating README.md to include information about priority sorting..."
sed -i '/## Features/a - Priority sorting in list command' README.md

# Update USAGE.md to include example of priority sorting
echo "Updating USAGE.md to include example of priority sorting..."
cat << EOT >> USAGE.md

## List goals sorted by priority
\`\`\`
$ git goals list
Current Goals:
- 20240101000001 (active, Priority: high): High priority goal
- 20240101000002 (active, Priority: medium): Medium priority goal
- 20240101000003 (active, Priority: low): Low priority goal
\`\`\`
EOT

# Update CONTRIBUTING.md to mention priority sorting feature
echo "Updating CONTRIBUTING.md to mention priority sorting feature..."
sed -i '/## Testing/a\- Ensure that priority sorting in the list command works correctly.' docs/CONTRIBUTING.md

# Run tests to ensure everything is working correctly
echo "Running tests..."
./test-git-goals.sh

echo "All updates completed. Sleeping for 15 minutes before next iteration..."
sleep 900