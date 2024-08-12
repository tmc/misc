#!/bin/bash

# Function to update a script
update_script() {
  local script_name=$1
  local content=$2

  echo "Updating $script_name..."
  echo "$content" > "$script_name"
  chmod +x "$script_name"
}

# Update git-goals-show
update_script "git-goals-show" '#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: git goals show <goal_id>"
    exit 1
fi

goal_id="$1"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use '\''git goals create'\'' to add a new goal."
    exit 1
fi

# Search for the goal with the given ID
goal=$(grep "^#$goal_id |" .git-goals)

if [ -z "$goal" ]; then
    echo "Error: Goal with ID $goal_id not found."
    exit 1
fi

# Parse and display the goal details
IFS='\''|'\'' read -r _ status description <<< "$goal"
echo "ID: $goal_id"
echo "Status: ${status## }"
echo "Description: ${description## }"
'

# Update git-goals-delete
update_script "git-goals-delete" '#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Error: Please provide a goal ID to delete."
    echo "Usage: git goals delete <id>"
    exit 1
fi

id="$1"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use '\''git goals create'\'' to add a new goal."
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
'

# Update git-goals-update
update_script "git-goals-update" '#!/bin/bash
set -euo pipefail

if [ $# -lt 2 ]; then
    echo "Error: Please provide a goal ID and the new status."
    echo "Usage: git goals update <id> <new_status>"
    exit 1
fi

id="$1"
new_status="$2"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use '\''git goals create'\'' to add a new goal."
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
'

# Update git-goals-report
update_script "git-goals-report" '#!/bin/bash
set -euo pipefail

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "No goals found. Use '\''git goals create'\'' to add a new goal."
    exit 0
fi

echo "Goal Report"
echo "==========="

while IFS='\''|'\'' read -r id status description; do
    id=${id#\#}
    echo "Goal ID: ${id## }"
    echo "Status: ${status## }"
    echo "Description: ${description## }"
    echo "---"
done < .git-goals
'

# Update git-goals-complete
update_script "git-goals-complete" '#!/bin/bash
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: git goals complete <goal_id> [rationale]"
    exit 1
fi

id="$1"
rationale="${2:-}"

# Check if .git-goals file exists
if [ ! -f .git-goals ]; then
    echo "Error: No goals found. Use '\''git goals create'\'' to add a new goal."
    exit 1
fi

# Update the goal status to completed
sed -i "s/^#$id |[^|]*|/#$id | COMPLETED |/" .git-goals

# Check if the update was successful
if grep -q "^#$id | COMPLETED |" .git-goals; then
    echo "Goal $id marked as complete"
    if [ -n "$rationale" ]; then
        echo "Rationale: $rationale"
        # TODO: Add rationale to the goal entry
    fi
else
    echo "Error: Failed to complete goal $id. Make sure the ID exists."
    exit 1
fi
'

echo "All git-goals-* scripts have been updated."