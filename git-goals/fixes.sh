#!/usr/bin/bash

# Continue developing the plugin system
echo "Enhancing plugin system..."
mkdir -p plugins/examples
cat << EOF > plugins/examples/hello_world.sh
#!/bin/bash
# Example plugin: Hello World

git_goals_hello_world() {
    echo "Hello from the git-goals plugin system!"
}

# Register the new subcommand
git_goals_register_command "hello" "git_goals_hello_world" "Print a hello message"
EOF
chmod +x plugins/examples/hello_world.sh

# Update main git-goals script to support plugin registration
sed -i '/# Load plugins/a\
declare -A GIT_GOALS_COMMANDS\
\
git_goals_register_command() {\
    local command_name="$1"\
    local function_name="$2"\
    local description="$3"\
    GIT_GOALS_COMMANDS["$command_name"]="$function_name"\
    GIT_GOALS_COMMANDS["${command_name}_description"]="$description"\
}\
' git-goals

# Update usage function in git-goals to display plugin commands
sed -i '/print_usage()/,/^}/c\
print_usage() {\
    echo "Usage: git goals <subcommand> [<args>]"\
    echo "Available subcommands:"\
    echo "  create, list, show, update, delete, complete, report, recover, notify, prioritize, deadline"\
    for cmd in "${!GIT_GOALS_COMMANDS[@]}"; do\
        if [[ ! $cmd =~ _description$ ]]; then\
            echo "  $cmd: ${GIT_GOALS_COMMANDS[${cmd}_description]}"\
        fi\
    done\
    echo "Use --version to display version information"\
    echo "Use <subcommand> --help for more information about a specific subcommand"\
}' git-goals

# Update case statement in git-goals to handle plugin commands
sed -i '/case "$1" in/,/esac/c\
case "$1" in\
    notify|prioritize|deadline)\
        shift\
        "git-goals-$1" "$@"\
        ;;\
    create|list|show|update|delete|complete|report|recover)\
        shift\
        "git-goals-$1" "$@"\
        ;;\
    --version)\
        echo "git-goals version $VERSION"\
        exit 0\
        ;;\
    *)\
        if [[ -n "${GIT_GOALS_COMMANDS[$1]}" ]]; then\
            ${GIT_GOALS_COMMANDS[$1]} "$@"\
        else\
            echo "Error: Unknown subcommand '\''$1'\''" >&2\
            print_usage\
            exit 1\
        fi\
        ;;\
esac' git-goals

# Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
cat << EOF >> CHANGELOG.md

## [0.2.12] - $(date +%Y-%m-%d)
### Added
- Enhanced plugin system with example "hello world" plugin
- Plugin command registration and dynamic command listing
### Changed
- Updated main git-goals script to support plugin registration and execution
EOF

# Update version number
sed -i 's/VERSION="0.2.11"/VERSION="0.2.12"/' git-goals

# Update test suite
echo "Updating test suite..."
cat << EOF >> test-git-goals.sh

# Test plugin system
echo "Testing plugin system..."
source plugins/examples/hello_world.sh
output=\$(git_goals_hello_world)
if [[ "\$output" == "Hello from the git-goals plugin system!" ]]; then
    echo "Plugin test passed"
else
    echo "Plugin test failed"
    exit 1
fi
EOF

# Commit changes
git add plugins/ git-goals CHANGELOG.md test-git-goals.sh
git commit -m "Enhance plugin system with example and integration"

echo "Plugin system enhancement complete. Sleeping for 1 hour..."
sleep 3600