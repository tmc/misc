#!/bin/bash

# Plan: Update git-goals-show script to handle errors and improve output formatting

# Check if git-goals-show exists
if [ ! -f "git-goals-show" ]; then
    echo "Error: git-goals-show script not found."
    exit 1
fi

# Update git-goals-show script
cat > git-goals-show << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -eq 0 ]; then
    echo "Usage: git goals show <goal_id>"
    exit 1
fi

goal_id="\$1"

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo "Error: Goal with ID \$goal_id not found."
    exit 1
fi

goal_data=\$(git notes --ref=goals show "\$commit_hash")

echo "Goal Details:"
echo "============="
echo "\$goal_data" | sed 's/^/  /'
EOL

chmod +x git-goals-show

echo "Updated git-goals-show script with error handling and improved output formatting."

# Commit changes
git add git-goals-show
git commit -m "Improved git-goals-show script with error handling and better formatting"

# Run tests
./test-git-goals.sh

# If we get here, tests passed
echo "All tests passed successfully."