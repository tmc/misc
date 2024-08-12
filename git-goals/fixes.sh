#!/bin/bash

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

git notes --ref=goals show "\$commit_hash"
EOL
chmod +x git-goals-show

# Update git-goals-delete script
cat > git-goals-delete << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -eq 0 ]; then
    echo "Usage: git goals delete <goal_id>"
    exit 1
fi

goal_id="\$1"

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo "Error: Goal with ID \$goal_id not found."
    exit 1
fi

git notes --ref=goals remove "\$commit_hash"

echo "Goal \$goal_id deleted"
EOL
chmod +x git-goals-delete

# Update git-goals-update script
cat > git-goals-update << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -lt 2 ]; then
    echo "Usage: git goals update <goal_id> <new_description>"
    exit 1
fi

goal_id="\$1"
shift
new_description="\$*"

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo "Error: Goal with ID \$goal_id not found."
    exit 1
fi

current_data=\$(git notes --ref=goals show "\$commit_hash")
updated_data=\$(echo "\$current_data" | sed "s/^description:.*\$/description: \$new_description/")

echo "\$updated_data" | git notes --ref=goals add -f -F - "\$commit_hash"

echo "Updated goal \$goal_id: \$new_description"
EOL
chmod +x git-goals-update

# Update git-goals-report script
cat > git-goals-report << EOL
#!/bin/bash
set -euo pipefail

echo "Goal Report"
echo "==========="

git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=\$(git notes --ref=goals show "\$commit_hash")

    id=\$(echo "\$goal_data" | grep "^id:" | cut -d" " -f2-)
    description=\$(echo "\$goal_data" | grep "^description:" | cut -d" " -f2-)
    status=\$(echo "\$goal_data" | grep "^status:" | cut -d" " -f2-)
    created_at=\$(echo "\$goal_data" | grep "^created_at:" | cut -d" " -f2-)
    completed_at=\$(echo "\$goal_data" | grep "^completed_at:" | cut -d" " -f2-)

    echo "Goal ID: \$id"
    echo "Description: \$description"
    echo "Status: \$status"
    echo "Created: \$created_at"
    if [ "\$status" = "completed" ]; then
        echo "Completed: \$completed_at"
    fi
    echo "---"
done
EOL
chmod +x git-goals-report

# Update git-goals-list script
cat > git-goals-list << EOL
#!/bin/bash
set -euo pipefail

echo "Current Goals:"
git notes --ref=goals list | while read -r note_ref commit_hash; do
    goal_data=\$(git notes --ref=goals show "\$commit_hash")
    id=\$(echo "\$goal_data" | grep "^id:" | cut -d" " -f2-)
    status=\$(echo "\$goal_data" | grep "^status:" | cut -d" " -f2-)
    description=\$(echo "\$goal_data" | grep "^description:" | cut -d" " -f2-)
    echo "- \$id (\$status): \$description"
done
EOL
chmod +x git-goals-list

# Update git-goals-create script
cat > git-goals-create << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -eq 0 ]; then
    echo "Error: Please provide a description for the goal."
    echo "Usage: git goals create <description>"
    exit 1
fi

description="\$*"
id=\$(date +%Y%m%d%H%M%S)

git notes --ref=goals add -m "id: \$id
type: goal
description: \$description
status: active
created_at: \$(date -I)" HEAD

echo "Goal ID: \$id"
echo "Description: \$description"
EOL
chmod +x git-goals-create

# Update git-goals-complete script
cat > git-goals-complete << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -lt 1 ]; then
    echo "Usage: git goals complete <goal_id> [attempt_id] [rationale]"
    exit 1
fi

goal_id="\$1"
attempt_id="\${2:-}"
rationale="\${3:-}"

commit_hash=\$(git notes --ref=goals list | grep "\$goal_id" | awk "{print \\\$1}")

if [ -z "\$commit_hash" ]; then
    echo "Error: Goal with ID \$goal_id not found."
    exit 1
fi

current_data=\$(git notes --ref=goals show "\$commit_hash")
updated_data=\$(echo "\$current_data" | sed "s/^status:.*\$/status: completed/")
updated_data+="
completed_at: \$(date -I)"

if [ -n "\$attempt_id" ]; then
    updated_data+="
attempt_id: \$attempt_id"
fi

if [ -n "\$rationale" ]; then
    updated_data+="
rationale: \$rationale"
fi

echo "\$updated_data" | git notes --ref=goals add -f -F - "\$commit_hash"

echo "Goal \$goal_id marked as complete"
if [ -n "\$rationale" ]; then
    echo "Rationale: \$rationale"
fi
EOL
chmod +x git-goals-complete

# Update git-goals script (main entry point)
cat > git-goals << EOL
#!/bin/bash
set -euo pipefail

if [ \$# -eq 0 ]; then
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: create, list, show, update, delete, complete, report"
    exit 1
fi

subcommand="\$1"
shift

case "\$subcommand" in
    create|list|show|update|delete|complete|report)
        "git-goals-\$subcommand" "\$@"
        ;;
    *)
        echo "Error: Unknown subcommand '\$subcommand'"
        echo "Available subcommands: create, list, show, update, delete, complete, report"
        exit 1
        ;;
esac
EOL
chmod +x git-goals

echo "All git-goals scripts have been updated."