#!/bin/bash

# Implement a main git-goals script to dispatch to subcommands
cat << 'EOF' > git-goals
#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: create, list, show, update, delete, complete, report, recover"
    exit 1
fi

subcommand="$1"
shift

case "$subcommand" in
    create|list|show|update|delete|complete|report|recover)
        "git-goals-$subcommand" "$@"
        ;;
    *)
        echo "Error: Unknown subcommand '$subcommand'"
        echo "Available subcommands: create, list, show, update, delete, complete, report, recover"
        exit 1
        ;;
esac
EOF

chmod +x git-goals

# Add error handling and input validation to existing scripts
for script in git-goals-*; do
    sed -i '2i\
# Input validation and error handling\
if [ $# -eq 0 ]; then\
    echo "Usage: $0 <args>"\
    exit 1\
fi\
' "$script"
done

# Improve output formatting for better readability
for script in git-goals-*; do
    sed -i 's/echo "/echo -e "\\033[1m/' "$script"
    sed -i 's/echo "/echo -e "\\033[0m/' "$script"
done

# Add version information and --version flag
echo '
VERSION="0.1.0"

if [ "$1" = "--version" ]; then
    echo "git-goals version $VERSION"
    exit 0
fi
' >> git-goals

# Update README.md with new information
cat << 'EOF' > README.md
# git-goals

git-goals is a set of command-line tools to manage and track goals within a Git repository. It allows you to create, update, list, and complete goals, as well as generate reports on your progress.

## Installation

1. Clone this repository or download the scripts.
2. Add the directory containing these scripts to your PATH.
3. Ensure the scripts are executable (`chmod +x git-goals*`).

## Usage

### Create a new goal

```
git goals create <goal_description>
```

### Update a goal

```
git goals update <goal_id> <new_goal_description>
```

### List goals

```
git goals list
```

### Show goal details

```
git goals show <goal_id>
```

### Complete a goal

```
git goals complete <goal_id> [attempt_id] [rationale]
```

### Delete a goal

```
git goals delete <goal_id>
```

### Generate a report

```
git goals report
```

### Recover goals

```
git goals recover
```

## How it works

git-goals uses Git notes to store goal metadata. Each goal is associated with a specific commit, and the goal information is stored as a note on that commit. The tools provided allow you to manage these notes and the associated goal branches easily.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
EOF

# Create a simple configuration file
cat << 'EOF' > .git-goals-config
# Git Goals Configuration

# Custom note reference name (default: goals)
NOTE_REF_NAME=goals

# Date format for goal creation and completion (default: %Y-%m-%d)
DATE_FORMAT=%Y-%m-%d

# Maximum number of goals to display in list view (0 for unlimited)
MAX_GOALS_DISPLAY=0
EOF

# Update scripts to use configuration file
for script in git-goals-*; do
    sed -i '3i\
# Load configuration\
if [ -f ".git-goals-config" ]; then\
    source ".git-goals-config"\
fi\
\
NOTE_REF_NAME=${NOTE_REF_NAME:-goals}\
DATE_FORMAT=${DATE_FORMAT:-%Y-%m-%d}\
MAX_GOALS_DISPLAY=${MAX_GOALS_DISPLAY:-0}\
' "$script"
done

echo "Updates and improvements have been applied to the git-goals scripts."