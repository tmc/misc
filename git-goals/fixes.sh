#!/bin/bash

# Update git-goals-list script
cat > git-goals-list << 'EOF'
#!/bin/bash

# Function to parse and display goals in both old and new formats
parse_and_display_goals() {
    while IFS= read -r line; do
        if [[ $line == \#* ]]; then
            # New format: #ID | Status | Description
            IFS='|' read -r id status description <<< "${line#\#}"
            printf "%-5s %-10s %s\n" "${id## }" "${status## }" "${description## }"
        else
            # Old format: just print the line as-is
            echo "$line"
        fi
    done < "$1"
}

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "No goals found. Use 'git goals create' to add a new goal."
    exit 0
fi

# Display header
echo "ID    Status      Description"
echo "---   ------      -----------"

# Parse and display goals
parse_and_display_goals .git-goals
EOF

chmod +x git-goals-list

# Update git-goals-create script
cat > git-goals-create << 'EOF'
#!/bin/bash

# Check if a description is provided
if [ $# -eq 0 ]; then
    echo "Error: Please provide a description for the goal."
    echo "Usage: git goals create <description>"
    exit 1
fi

# Combine all arguments into a single description
description="$*"

# Generate a unique ID (timestamp-based)
id=$(date +%s)

# Add the new goal to the .git-goals file
echo "#$id | TODO | $description" >> .git-goals

echo "Goal created successfully:"
echo "ID: $id"
echo "Status: TODO"
echo "Description: $description"
EOF

chmod +x git-goals-create

# Update git-goals-show script
cat > git-goals-show << 'EOF'
#!/bin/bash

# Check if an ID is provided
if [ $# -eq 0 ]; then
    echo "Error: Please provide a goal ID."
    echo "Usage: git goals show <id>"
    exit 1
fi

id="$1"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use 'git goals create' to add a new goal."
    exit 1
fi

# Search for the goal with the given ID
goal=$(grep "^#$id |" .git-goals)

if [ -z "$goal" ]; then
    echo "Error: Goal with ID $id not found."
    exit 1
fi

# Parse and display the goal details
IFS='|' read -r _ status description <<< "$goal"
echo "ID: $id"
echo "Status: ${status## }"
echo "Description: ${description## }"
EOF

chmod +x git-goals-show

# Update git-goals-update script
cat > git-goals-update << 'EOF'
#!/bin/bash

# Check if ID and new status are provided
if [ $# -lt 2 ]; then
    echo "Error: Please provide a goal ID and the new status."
    echo "Usage: git goals update <id> <new_status>"
    exit 1
fi

id="$1"
new_status="$2"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use 'git goals create' to add a new goal."
    exit 1
fi

# Update the goal status
sed -i "s/^#$id |[^|]*|/#$id | $new_status |/" .git-goals

# Check if the update was successful
if grep -q "^#$id | $new_status |" .git-goals; then
    echo "Goal $id updated successfully. New status: $new_status"
else
    echo "Error: Failed to update goal $id. Make sure the ID exists."
    exit 1
fi
EOF

chmod +x git-goals-update

# Update git-goals-delete script
cat > git-goals-delete << 'EOF'
#!/bin/bash

# Check if an ID is provided
if [ $# -eq 0 ]; then
    echo "Error: Please provide a goal ID to delete."
    echo "Usage: git goals delete <id>"
    exit 1
fi

id="$1"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use 'git goals create' to add a new goal."
    exit 1
fi

# Delete the goal with the given ID
sed -i "/^#$id |/d" .git-goals

# Check if the deletion was successful
if ! grep -q "^#$id |" .git-goals; then
    echo "Goal $id deleted successfully."
else
    echo "Error: Failed to delete goal $id. Make sure the ID exists."
    exit 1
fi
EOF

chmod +x git-goals-delete

# Update git-goals script (main entry point)
cat > git-goals << 'EOF'
#!/bin/bash

# Check if a subcommand is provided
if [ $# -eq 0 ]; then
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: list, create, show, update, delete"
    exit 1
fi

subcommand="$1"
shift

case "$subcommand" in
    list|create|show|update|delete)
        "git-goals-$subcommand" "$@"
        ;;
    *)
        echo "Error: Unknown subcommand '$subcommand'"
        echo "Available subcommands: list, create, show, update, delete"
        exit 1
        ;;
esac
EOF

chmod +x git-goals

echo "Git goals scripts have been updated and fixed."