#!/bin/bash

# Analyze the sandbox output and suggest improvements
echo "Analyzing sandbox output..."
docker logs $(get-latest-sandbox) | grep -v "^$" | tail -n 20 | cgpt -s "Analyze this sandbox output and suggest improvements or next steps:" > analysis.txt

# Update OBSERVATIONS file
echo "Updating OBSERVATIONS file..."
cat OBSERVATIONS > OBSERVATIONS.previous
(echo "Observations: "; cat analysis.txt) | cgpt -s "we are fixing issues in this codebase. consider the contents in OBSERVATIONS, keep it to short lines, and consider updating it. output new contents of OBSERVATIONS to guide our next steps" -p 'Observations:' > OBSERVATIONS

# Generate fixes
echo "Generating fixes..."
(echo "we will fix what went wrong"; docker logs $(get-latest-sandbox); git st .) | cgpt -s "we are fixing issues in this codebase. consider the contents in OBSERVATIONS, and consider updating it. output a bash script to carry out fixes/improvements; if it appears we are getting close to being done, don't waste compute and begin sleeping for longer periods of time" -p '#!/bin/bash' > fixes.sh

# Apply fixes
echo "Applying fixes..."
chmod +x fixes.sh
./fixes.sh

# Commit changes
echo "Committing changes..."
git add -u .
git commit -m "git-notes: Fixing the codebase" --allow-empty

# Run tests
echo "Running tests..."
./test-git-goals.sh

echo "Iteration complete. Review the changes and run again if necessary."