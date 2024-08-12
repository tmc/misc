#!/bin/bash

# Script to update git-goals-create

set -euo pipefail

# Function to update a script
update_script() {
  local script_name=$1
  local content=$2

  echo "Updating $script_name..."
  echo "$content" > "$script_name"
  chmod +x "$script_name"
}

# Update git-goals-create
update_script "git-goals-create" '#!/bin/bash
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "Error: Please provide a description for the goal."
    echo "Usage: git goals create <description>"
    exit 1
fi

description="$*"
id=$(date +%Y%m%d%H%M%S)

git notes --ref=goals add -m "id: $id
type: goal
description: $description
status: active
created_at: $(date -I)" HEAD

echo "Goal ID: $id"
echo "Description: $description"
'

echo "git-goals-create script has been updated."