#!/usr/bin/env bash
DIRECTORY="." # Replace with the path to your directory containing the code files
EXCLUDE_DIRS=() # Array to store directories to exclude
IGNORED_FILES=("go.sum" "package.json") # Array to store files to exclude
function read_directory_files {
    # Check if .gitignore file exists
    if [ -f "$1/.gitignore" ]; then
        # Read the .gitignore file and store the patterns in an array
        mapfile -t ignore_patterns < "$1/.gitignore"
    else
        # If .gitignore doesn't exist, create an empty array
        ignore_patterns=()
    fi
    for filename in "$1"/**; do
        # Exclude specific files:
        case "$(basename "$filename")" in "${IGNORED_FILES[@]}")
            continue
            ;;
        esac
        # Exclude binary (non-text) files:
        if [ -f "$filename" ] && file "$filename" | grep -q text; then
            # Check if the file matches any pattern in .gitignore
            ignore=false
            for pattern in "${ignore_patterns[@]}"; do
                if [[ "$filename" == $1/$pattern ]]; then
                    ignore=true
                    break
                fi
            done
            # Check if the file is in an excluded directory
            for dir in "${EXCLUDE_DIRS[@]}"; do
                if [[ "$filename" == $1/$dir/* ]]; then
                    ignore=true
                    break
                fi
            done
            # If the file is not ignored, print its contents
            if [ "$ignore" = false ]; then
                echo "== $(basename "$filename") =="
                cat "$filename"
                echo ""
            fi
        fi
    done
    # Recursively process subdirectories
    for dir in "$1"/*/; do
        if [ -d "$dir" ]; then
            # Check if the directory is in the exclude list
            exclude=false
            for excluded_dir in "${EXCLUDE_DIRS[@]}"; do
                if [[ "$dir" == $1/$excluded_dir ]]; then
                    exclude=true
                    break
                fi
            done
            # If the directory is not excluded, process it recursively
            if [ "$exclude" = false ]; then
                read_directory_files "$dir"
            fi
        fi
    done
}
# Read directories to exclude from command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --exclude-dir)
            EXCLUDE_DIRS+=("$2")
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done
read_directory_files "$DIRECTORY"
echo "== END OF INPUT =="
