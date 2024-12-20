#!/usr/bin/env bash
# Run linting checks on all shell scripts
set -euo pipefail

# Find all shell scripts
find_shell_scripts() {
    find . -type f \( -name "*.sh" -o -exec sh -c 'head -n1 {} | grep -q "^#!.*sh"' \; \)
}

# Run shellcheck
check_scripts() {
    local script
    while IFS= read -r script; do
        echo "Checking: $script"
        shellcheck -x "$script"
    done < <(find_shell_scripts)
}

# Main execution
main() {
    local failed=0
    
    echo "Running shellcheck..."
    if ! check_scripts; then
        failed=$((failed + 1))
    fi
    
    if [[ $failed -eq 0 ]]; then
        echo "All checks passed!"
        return 0
    else
        echo "Checks failed!"
        return 1
    fi
}

main "$@"
