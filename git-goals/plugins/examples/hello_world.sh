#!/bin/bash
# Example plugin: Hello World

git_goals_hello_world() {
    echo "Hello from the git-goals plugin system!"
}

# Register the new subcommand
git_goals_register_command "hello" "git_goals_hello_world" "Print a hello message"
