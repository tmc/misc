#!/bin/bash

# This script will make minor improvements to the git-goals project.

# 1. Update git-goals main script to include version information
cat << EOF > git-goals
#!/bin/bash
set -euo pipefail

VERSION="0.1.1"

if [ "\$1" = "--version" ]; then
    echo "git-goals version \$VERSION"
    exit 0
fi

if [ \$# -eq 0 ]; then
    echo "Usage: git goals <subcommand> [<args>]"
    echo "Available subcommands: create, list, show, update, delete, complete, report, recover"
    echo "Use --version to display version information"
    exit 1
fi

subcommand="\$1"
shift

case "\$subcommand" in
    create|list|show|update|delete|complete|report|recover)
        "git-goals-\$subcommand" "\$@"
        ;;
    *)
        echo "Error: Unknown subcommand '\$subcommand'"
        echo "Available subcommands: create, list, show, update, delete, complete, report, recover"
        exit 1
        ;;
esac
EOF

chmod +x git-goals

# 2. Update README.md to include version information
sed -i '1i# git-goals v0.1.1\n' README.md

# 3. Add a CHANGELOG.md file
cat << EOF > CHANGELOG.md
# Changelog

## [0.1.1] - $(date +%Y-%m-%d)
### Added
- Version information to main script
- CHANGELOG.md file

### Changed
- Updated README.md to include version information

## [0.1.0] - Initial Release
- Basic functionality implemented
EOF

# 4. Update .gitignore to exclude common development files
echo "*.log
*.swp
.DS_Store" >> .gitignore

# 5. Add a simple Makefile for common tasks
cat << EOF > Makefile
.PHONY: test install

test:
	@echo "Running tests..."
	@bash test-git-goals.sh

install:
	@echo "Installing git-goals..."
	@cp git-goals* /usr/local/bin/

clean:
	@echo "Cleaning up..."
	@rm -f *.log
EOF

echo "Improvements have been made to the git-goals project."
echo "Please review the changes and commit them if satisfactory."