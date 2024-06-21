#!/usr/bin/env bash
# code-to-gpt.sh
# Script to read and print contents of text files in a directory, excluding specified directories and files

DIRECTORY="." # Default directory to process, can be replaced with your specific directory
EXCLUDE_DIRS=("node_modules") # Array of directories to exclude
IGNORED_FILES=("go.sum" "go.work.sum" "yarn.lock" "yarn.error.log" "package-lock.json") # Array of files to ignore
IGNORE_PATTERNS=() # Array to store combined .gitignore patterns
COUNT_TOKENS=false # Flag to enable token counting

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
                # Add the pattern as-is
                IGNORE_PATTERNS+=("$line")
            fi
        done < "$gitignore_file"
    fi
}

# Function to check if a file matches any of the ignore patterns
function is_ignored_file {
    local file="$1"
    local rel_file="${file#$DIRECTORY/}"
    for pattern in "${IGNORE_PATTERNS[@]}"; do
        if [[ "$rel_file" == $pattern || "$rel_file" == *"/$pattern" ]]; then
            return 0
        fi
    done
    return 1
}

function is_ignored_filename {
    local filename="$1"
    local basename=$(basename "$filename")
    for ignored_file in "${IGNORED_FILES[@]}"; do
        if [[ "$basename" == "$ignored_file" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to count tokens in a file
function count_tokens {
    local file="$1"
    # Assuming 'tokencount' is a command that returns the number of tokens
    # Replace this with the actual command you use for token counting
    tokencount "$file" 2>/dev/null || echo "0"
}

# Function to read and print files in a directory, recursively processing subdirectories
function read_directory_files {
    local dir="$1"
    local rel_path="${2:-}"

    # Check if the directory is excluded
    is_excluded_dir "$dir" && return

    # Read .gitignore file patterns if exists
    read_gitignore "$dir"

    for filename in "$dir"/*; do
        # Skip specific files
        if is_ignored_filename "$filename"; then
            continue
        fi

        # Process text files
        if [ -f "$filename" ] && file "$filename" | grep -q text; then
            # Check if the file matches any .gitignore pattern
            if ! is_ignored_file "$filename"; then
                local file_rel_path="${rel_path:+$rel_path/}$(basename "$filename")"
                if [ "$COUNT_TOKENS" = true ]; then
                    local token_count=$(count_tokens "$filename")
                    echo "$token_count $file_rel_path"
                else
                    echo "=== $file_rel_path ==="
                    cat "$filename"
                    echo ""
                fi
            fi
        fi
    done

    # Recursively process subdirectories
    for subdir in "$dir"/*/; do
        if [ -d "$subdir" ]; then
            local subdir_name=$(basename "$subdir")
            local new_rel_path="${rel_path:+$rel_path/}$subdir_name"
            read_directory_files "$subdir" "$new_rel_path"
        fi
    done
}

# Read command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --exclude-dir)
            EXCLUDE_DIRS+=("$2")
            shift 2
            ;;
        --count-tokens)
            COUNT_TOKENS=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Start reading files from the specified directory
read_directory_files "$DIRECTORY" ""
if [ "$COUNT_TOKENS" = false ]; then
    echo "=== END OF INPUT ==="
fi
