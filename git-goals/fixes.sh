#!/usr/bin/bash

# Update version number in git-goals script
sed -i 's/VERSION="0.1.5"/VERSION="0.1.6"/' git-goals

# Update README.md with new version number
sed -i 's/# git-goals v0.1.5/# git-goals v0.1.6/' README.md

# Update CHANGELOG.md with new version
cat << EOF >> CHANGELOG.md

## [0.1.6] - $(date +%Y-%m-%d)
### Changed
- Minor improvements and code cleanup
- Updated error handling in git-goals-common.sh
- Improved consistency across all scripts
EOF

# Remove meta-improvement tools
rm -f make-fix-suggestion.sh make-safe-progress.sh eval-make-safe-progress.sh

# Update .gitignore to exclude meta-improvement tools
echo "make-fix-suggestion.sh" >> .gitignore
echo "make-safe-progress.sh" >> .gitignore
echo "eval-make-safe-progress.sh" >> .gitignore

# Update USAGE.md with latest examples
cat << EOF > USAGE.md
# Git Goals Usage Examples

Here are some examples of how to use git-goals:

## Create a new goal
\`\`\`
$ git goals create "Implement new feature"
Created new goal with ID: 20240101000000
Description: Implement new feature
\`\`\`

## List all goals
\`\`\`
$ git goals list
Current Goals:
- 20240101000000 (active): Implement new feature
\`\`\`

## Show goal details
\`\`\`
$ git goals show 20240101000000
Goal Details:
=============
id: 20240101000000
type: goal
description: Implement new feature
status: active
created_at: 2024-01-01
\`\`\`

## Update a goal
\`\`\`
$ git goals update 20240101000000 "Implement new feature with improved performance"
Updated goal 20240101000000: Implement new feature with improved performance
\`\`\`

## Complete a goal
\`\`\`
$ git goals complete 20240101000000 "" "Feature implemented and tested"
Goal 20240101000000 marked as complete
Rationale: Feature implemented and tested
\`\`\`

## Generate a report
\`\`\`
$ git goals report
Goal Report
===========
Goal ID: 20240101000000
Description: Implement new feature with improved performance
Status: completed
Created: 2024-01-01
Completed: 2024-01-01
---
\`\`\`

## Delete a goal
\`\`\`
$ git goals delete 20240101000000
Goal 20240101000000 deleted
\`\`\`

## Recover goals
\`\`\`
$ git goals recover
Processing note for commit <commit_hash>
Recovered goal:
id: 20240101000001
type: goal
description: Recovered goal description
status: active
created_at: 2024-01-01
---
\`\`\`

For more detailed information on each command, use the --help option:
\`\`\`
$ git goals <command> --help
\`\`\`
EOF

# Update README.md with information about running tests
cat << EOF >> README.md

## Running Tests

To run the test suite and verify the functionality of git-goals, use the following command:

\`\`\`bash
./test-git-goals.sh
\`\`\`

This will run through a series of tests to ensure all commands are working as expected.
EOF

# Remove THOUGHTS file as it's no longer needed
rm -f THOUGHTS

# Update IMPORTANT file
cat << EOF > IMPORTANT
- Consider setting up a continuous integration pipeline for automated testing
- Gather user feedback on the current feature set and user experience
- Investigate potential integrations with popular Git hosting platforms or project management tools
- Consider implementing more advanced features like goal prioritization or deadline tracking
- Prepare documentation for contributors, including coding standards and contribution guidelines
EOF

echo "Updates completed. Please review changes and commit if satisfactory."