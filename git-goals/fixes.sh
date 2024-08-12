#!/usr/bin/bash

# This script will make minor improvements to the git-goals codebase

# Update version number in git-goals and README.md
sed -i 's/VERSION="0.1.3"/VERSION="0.1.4"/' git-goals
sed -i 's/# git-goals v0.1.3/# git-goals v0.1.4/' README.md

# Update CHANGELOG.md
cat << EOF >> CHANGELOG.md

## [0.1.4] - $(date +%Y-%m-%d)
### Changed
- Minor improvements and code cleanup
- Updated error handling in git-goals-common.sh
- Improved consistency in script headers
EOF

# Update git-goals-common.sh with better error handling
cat << EOF > git-goals-common.sh
#!/usr/bin/bash

set -euo pipefail

load_config() {
    if [ -f ".git-goals-config" ]; then
        source ".git-goals-config"
    else
        echo "Warning: .git-goals-config not found. Using default settings." >&2
    fi

    NOTE_REF_NAME=\${NOTE_REF_NAME:-goals}
    DATE_FORMAT=\${DATE_FORMAT:-%Y-%m-%d}
    MAX_GOALS_DISPLAY=\${MAX_GOALS_DISPLAY:-0}
}

print_help() {
    echo "Usage: git goals \$1 [options]"
    echo "Options:"
    echo "  --help    Show this help message"
}

check_args() {
    if [ \$# -eq 0 ]; then
        echo "Error: No arguments provided" >&2
        print_help "\$1"
        exit 1
    fi
}
EOF

# Update headers of all git-goals-* scripts
for script in git-goals-*; do
    sed -i '1,/^$/c\#!/usr/bin/bash\n\nset -euo pipefail\n\nsource "$(dirname "$0")/git-goals-common.sh"\n\nload_config' "$script"
done

# Remove unnecessary comments from git-goals-* scripts
sed -i '/^# Input validation and error handling/d' git-goals-*
sed -i '/^# Load configuration/d' git-goals-*

# Update git-goals script with better subcommand handling
cat << EOF > git-goals
#!/usr/bin/bash

set -euo pipefail

VERSION="0.1.4"

print_usage() {
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: create, list, show, update, delete, complete, report, recover"
    echo "Use --version to display version information"
    echo "Use <subcommand> --help for more information about a specific subcommand"
}

if [ \$# -eq 0 ]; then
    print_usage
    exit 1
fi

case "\$1" in
    --version)
        echo "git-goals version \$VERSION"
        exit 0
        ;;
    create|list|show|update|delete|complete|report|recover)
        subcommand="\$1"
        shift
        "git-goals-\$subcommand" "\$@"
        ;;
    *)
        echo "Error: Unknown subcommand '\$1'" >&2
        print_usage
        exit 1
        ;;
esac
EOF

# Commit changes
git add .
git commit -m "Improve codebase consistency and error handling"

echo "Codebase improvements complete. Version updated to 0.1.4."