#!/usr/bin/env bash
# code-to-gpt.sh
# Script to read and print contents of text files in a directory, excluding specified directories and files

DIRECTORY="." # Default directory to process, can be replaced with your specific directory
EXCLUDE_DIRS=("node_modules") # Array of directories to exclude
IGNORED_FILES=("go.sum" "package-lock.json") # Array of files to ignore
IGNORE_PATTERNS=() # Array to store combined .gitignore patterns

# Function to check if a directory is in the exclude list
function is_excluded_dir {
    local dir="$1"
    for excluded_dir in "${EXCLUDE_DIRS[@]}"; do
        if [[ "$dir" == "$DIRECTORY/$excluded_dir" || "$dir" == "$DIRECTORY/$excluded_dir/"* ]]; then
            return 0
        fi
    done
    return 1
}

# Function to read .gitignore files and add patterns to IGNORE_PATTERNS
function read_gitignore {
    local dir="$1"
    local gitignore_file="$dir/.gitignore"

    if [ -f "$gitignore_file" ]; then
        while IFS= read -r line || [[ -n "$line" ]]; do
            # Ignore comments and empty lines
            if [[ ! "$line" =~ ^# && ! "$line" =~ ^$ ]]; then
                # Add the pattern relative to the directory
                IGNORE_PATTERNS+=("$dir/$line")
            fi
        done < "$gitignore_file"
    fi
}

# Function to check if a file matches any of the ignore patterns
function is_ignored_file {
    local file="$1"
    for pattern in "${IGNORE_PATTERNS[@]}"; do
        if [[ "$file" == "$pattern" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to read and print files in a directory, recursively processing subdirectories
function read_directory_files {
    local dir="$1"
    
    # Check if the directory is excluded
    is_excluded_dir "$dir" && return
    
    # Read .gitignore file patterns if exists
    read_gitignore "$dir"
    
    for filename in "$dir"/*; do
        # Skip specific files
        case "$(basename "$filename")" in "${IGNORED_FILES[@]}")
            continue
            ;;
        esac
        
        # Process text files
        if [ -f "$filename" ] && file "$filename" | grep -q text; then
            # Check if the file matches any .gitignore pattern
            if ! is_ignored_file "$filename"; then
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

# Start reading files from the specified directory
read_directory_files "$DIRECTORY"
echo "== END OF INPUT =="
