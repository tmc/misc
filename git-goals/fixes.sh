#!/usr/bin/bash

# Update README.md with information about the plugin system
echo "Updating README.md with plugin system information..."
cat << EOF >> README.md

## Plugin System

git-goals now supports a plugin system for extending functionality. To create a plugin:

1. Create a new file in the `plugins` directory with a `.sh` extension.
2. Implement your plugin logic.
3. Use the `git_goals_register_command` function to register new subcommands.

Example plugin (plugins/hello_world.sh):

\`\`\`bash
#!/bin/bash
git_goals_hello_world() {
    echo "Hello from the git-goals plugin system!"
}

git_goals_register_command "hello" "git_goals_hello_world" "Print a hello message"
\`\`\`

Plugins are automatically loaded and new commands become available in the git-goals CLI.

EOF

# Update USAGE.md with an example of using a plugin
echo "Updating USAGE.md with plugin usage example..."
cat << EOF >> USAGE.md

## Using Plugins

After installing a plugin, you can use its commands like any other git-goals subcommand:

\`\`\`
$ git goals hello
Hello from the git-goals plugin system!
\`\`\`

EOF

# Perform some code cleanup
echo "Performing code cleanup..."
sed -i 's/\t/    /g' git-goals
sed -i 's/\t/    /g' git-goals-*

# Update version number
echo "Updating version number..."
sed -i 's/VERSION="0.2.12"/VERSION="0.2.13"/' git-goals

# Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << EOF >> CHANGELOG.md

## [0.2.13] - $(date +%Y-%m-%d)
### Added
- Updated README.md with information about the plugin system
- Updated USAGE.md with example of using a plugin
### Changed
- Performed code cleanup, replacing tabs with spaces
- Updated version number to 0.2.13
EOF

# Commit changes
echo "Committing changes..."
git add README.md USAGE.md git-goals git-goals-* CHANGELOG.md
git commit -m "Update documentation for plugin system, perform code cleanup, and bump version to 0.2.13"

echo "All tasks completed. Sleeping for 8 hours before next iteration..."
sleep 28800