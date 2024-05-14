#!/usr/bin/env bash
DIRECTORY="." # Replace with the path to your directory containing the code files
EXCLUDE_DIRS=("node_modules") # Array to store directories to exclude
IGNORED_FILES=("go.sum" "package.json") # Array to store files to exclude

function is_excluded_dir {
    local dir="$1"
    for excluded_dir in "${EXCLUDE_DIRS[@]}"; do
        if [[ "$dir" == "$DIRECTORY/$excluded_dir" || "$dir" == "$DIRECTORY/$excluded_dir/"* ]]; then
            return 0
        fi
    done
    return 1
}

function read_directory_files {
    local dir="$1"
    
    # Check if the directory is excluded
    is_excluded_dir "$dir" && return
    
    # Check if .gitignore file exists
    if [ -f "$dir/.gitignore" ]; then
        # Read the .gitignore file and store the patterns in an array
        mapfile -t ignore_patterns < "$dir/.gitignore"
    else
        # If .gitignore doesn't exist, create an empty array
        ignore_patterns=()
    fi
    
    for filename in "$dir"/*; do
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
                if [[ "$filename" == "$dir/$pattern" ]]; then
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
    for subdir in "$dir"/*/; do
        if [ -d "$subdir" ]; then
            read_directory_files "$subdir"
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
