#!/usr/bin/env bash
# code-to-gpt.sh
# Process files for LLM input, respecting .gitignore and custom exclusions.
# Usage: code-to-gpt.sh [options] [directory] [<pathspec>...] [-- <pathspec>...]
#
# For full usage instructions, run: ./code-to-gpt.sh --help
set -euo pipefail

# Default exclude patterns
DEFAULT_EXCLUDES=(
    ':!node_modules/**'
    ':!venv/**'
    ':!.venv/**'
    ':!.git/**'
    ':!go.sum'
    ':!go.work.sum'
    ':!yarn.lock'
    ':!yarn.error.log'
    ':!package-lock.json'
    ':!uv.lock'
)
DIRECTORY="."
COUNT_TOKENS=false
VERBOSE=false
USE_XML_TAGS=true
PATHSPEC=()

INCLUDE_SVG=false
INCLUDE_XML=false
WC_LIMIT=10000 # Maximum number of lines to process
TRACKED_ONLY=false
SKIP_TEMP_REPO=${SKIP_TEMP_REPO:-false}

# This defines a custom ignore file name that will be used in addition to .gitignore
ADDITIONAL_IGNORE_FILE_NAME=".ctx-src-ignore"

# Function to print usage information
print_usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS] [<directory>] [<pathspec>...] [-- <pathspec>...]

Process files in a directory for input to large language models, with flexible file selection.

Options:
  --count-tokens    Count tokens instead of outputting file contents
  --verbose         Enable verbose output
  --no-xml-tags     Disable XML tags around content
  --include-svg     Explicitly include SVG files
  --include-xml     Explicitly include XML files
  --tracked-only    Only include tracked files in Git repositories

Arguments:
  <directory>       Specify the directory to process (default: current directory)
  <pathspec>        One or more pathspec patterns to filter files

Notes:
  - The script processes a single directory, specified explicitly or defaulting to the current directory.
  - Pathspecs can be used to fine-tune file selection within the specified directory.
  - Default excludes (e.g., node_modules, .git) are applied unless overridden by pathspecs.
  - Use -- to explicitly mark the beginning of pathspecs, especially for paths starting with -.

Examples:
  $(basename "$0") --verbose /path/to/project
  $(basename "$0") . '*.js' '*.py'
  $(basename "$0") /path/to/project '**/*.txt' '!**/test/**'
  $(basename "$0") --tracked-only . -- -file-with-dash.txt
  $(basename "$0") /path/to/project '*.go' ':(exclude)vendor/**'

For more information, visit: https://github.com/yourusername/code-to-gpt
EOF
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count-tokens|--verbose|--no-xml-tags|--include-svg|--include-xml|--tracked-only)
            opt_name="$(echo "${1#--}" | sed 's/-/_/g' | tr '[:lower:]' '[:upper:]')"
            declare "${opt_name}=true"
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        --)
            shift
            PATHSPEC+=("$@")
            break
            ;;
        -*)
            echo "Error: Unknown option $1" >&2
            print_usage >&2
            exit 1
            ;;
        *)
            if [[ -z "$DIRECTORY" || "$DIRECTORY" == "." ]] && [[ -d "$1" ]]; then
                DIRECTORY="$1"
            else
                PATHSPEC+=("$1")
            fi
            shift
            ;;
    esac
done

# verbose echo
v_echo() {
    if $VERBOSE; then
        echo "$@" >&2
    fi
}

# Function to get relative path to a directory from home directory with tilde
get_home_relative_dirpath() {
    local dir="$1"
    local abs_path=$(cd "$dir" && pwd)
    local home_path=$HOME
    if [[ "$abs_path" == "$home_path" ]]; then
        echo "~"
    elif [[ "$abs_path" == "$home_path"/* ]]; then
        echo "~/${abs_path#$home_path/}"
    else
        echo "$abs_path"
    fi
}

get_home_relative_filepath() {
    local file="$1"
    local abs_path=$(realpath "$file")
    local home_path=$HOME
    if [[ "$abs_path" == "$home_path" ]]; then
        echo "~"
    elif [[ "$abs_path" == "$home_path"/* ]]; then
        echo "~/${abs_path#$home_path/}"
    else
        echo "$abs_path"
    fi
}

realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

get_relative_path() {
    local source="$1"
    local target="$2"

    # Normalize paths
    source="/${source#/}"
    target="/${target#/}"

    # Split paths into arrays
    IFS='/' read -ra source_parts <<< "$source"
    IFS='/' read -ra target_parts <<< "$target"

    # Find common prefix
    common_parts=0
    for ((i=0; i<${#source_parts[@]} && i<${#target_parts[@]}; i++)); do
        if [[ "${source_parts[i]}" != "${target_parts[i]}" ]]; then
            break
        fi
        ((common_parts++))
    done

    # Build relative path
    result=""
    for ((i=common_parts; i<${#source_parts[@]}; i++)); do
        result="../$result"
    done
    for ((i=common_parts; i<${#target_parts[@]}; i++)); do
        result="${result}${target_parts[i]}/"
    done

    # Remove trailing slash and return
    echo "${result%/}"
}

# Function to check if a file is a text file and should be included
is_text_file() {
    local file="$1"
    local mime_type=$(file -b --mime-type "$file")
    local extension="${file##*.}"
    local file_type=$(file -b "$file")
    # Check for text files without extensions
    if [[ "$mime_type" == text/* ]] || [[ "$file_type" == *"text"* ]]; then
        return 0
    elif [[ "$mime_type" == application/x-empty ]] || [[ "$mime_type" == inode/x-empty ]]; then
        return 0
    elif [[ "$mime_type" == image/svg+xml ]] && $INCLUDE_SVG; then
        return 0
    elif [[ "$mime_type" == application/xml ]] && $INCLUDE_XML; then
        return 0
    elif [[ "$mime_type" == "application/json" ]]; then
        return 0
    elif [[ "$mime_type" == "application/x-yaml" ]]; then
        return 0
    # Check for compressed text files
    elif [[ "$mime_type" == application/x-gzip ]] && [[ "${file%.*}" == *.txt ]]; then
        return 0
    # Additional checks for Unicode text files with BOM
    elif [[ "$file_type" == *"Unicode text"* ]]; then
        return 0
    # Check for symbolic links to text files
    elif [[ -L "$file" ]] && is_text_file "$(readlink -f "$file")"; then
        return 0
    else
        # Perform a more thorough check for text content
        if head -c 1000 "$file" | LC_ALL=C grep -q '[^[:print:][:space:]]'; then
            return 1
        else
            return 0
        fi
    fi
}

# Function to process a file
process_file() {
    local full_path="$1"
    local base_dir="$2"

    local relative_path=$(get_relative_path "$base_dir" "$full_path")

    local mime_type=$(file -b --mime-type "$full_path")
    local line_count=$(wc -l < "$full_path")

    if ! is_text_file "$full_path"; then
        v_echo "Skipping non-text file: $relative_path (MIME: $mime_type)"
        return
    fi

    if [ "$line_count" -gt "$WC_LIMIT" ]; then
        v_echo "Skipping large file: $relative_path ($line_count lines)"
        return
    fi

    v_echo "Processing file: $relative_path (MIME: $mime_type)"

    if $USE_XML_TAGS; then
        echo "  <file path=\"$relative_path\">"
    fi

    if $COUNT_TOKENS; then
        command -v tokencount &> /dev/null || {
            echo "tokencount is required for token counting" >&2
            echo "you can install it with go install github.com/tmc/tokencount@latest" >&2
            exit 1
        }
        token_count=$(tokencount "$full_path")
        echo "    $token_count"
    else
        sed 's/^/    /' "$full_path"
    fi

    if $USE_XML_TAGS; then
        echo "  </file>"
    fi
}

# Force off the use of XML tags if counting tokens:
if $COUNT_TOKENS; then
    USE_XML_TAGS=false
fi

# Helper function to check if a directory is a Git repository
is_git_repository() {
    git -C "$1" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

# Helper function to create a temporary Git repository
create_temp_git_repo() {
    local temp_dir
    temp_dir=$(mktemp -d) || { echo "Failed to create temp directory" >&2; return 1; }
    git init "$temp_dir" >/dev/null 2>&1 || { echo "Failed to initialize temp Git repo" >&2; rm -rf "$temp_dir"; return 1; }
    echo "$temp_dir"
}

# Helper function to get the Git directory
get_git_dir() {
    local dir="$1"
    local git_dir
    git_dir=$(git -C "$dir" rev-parse --git-dir) || { echo "Failed to get Git directory" >&2; return 1; }
    echo "$git_dir"
}

IGNORE_FILE_NAME=".ctx-src-ignore"

apply_custom_ignore() {
    local ignore_file="$GIT_WORK_TREE/$ADDITIONAL_IGNORE_FILE_NAME"
    if [ -f "$ignore_file" ]; then
        while IFS= read -r pattern || [[ -n "$pattern" ]]; do
            # Ignore empty lines and comments
            if [[ -n "$pattern" && ! "$pattern" =~ ^# ]]; then
                git_args+=(":(exclude)$pattern")
            fi
        done < "$ignore_file"
    fi
}

# Helper function to get files in a directory (DIRECTORY).
get_files() {
    local target_dir="$1"
    local temp_dir=""
    local git_dir
    local git_root

    if ! git -C "$target_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        if [ "$SKIP_TEMP_REPO" = "true" ]; then
            v_echo "Skipping temporary repo creation for non-Git directory"
            find "$target_dir" -type f -print0 | sed -z "s|^$target_dir/||" | tr '\0' '\n'
            return
        fi
        temp_dir=$(mktemp -d) || { v_echo "Error: Failed to create temp directory"; return 1; }
        if ! git init "$temp_dir" > /dev/null 2>&1; then
            v_echo "Error: Failed to initialize temporary Git repository"
            rm -rf "$temp_dir"
            return 1
        fi
        git_dir="$temp_dir/.git"
        git_root="$temp_dir"
    else
        if ! git_dir=$(git -C "$target_dir" rev-parse --git-dir 2>/dev/null); then
            v_echo "Error: Failed to get Git directory for $target_dir"
            return 1
        fi
        if ! git_root=$(git -C "$target_dir" rev-parse --show-toplevel 2>/dev/null); then
            v_echo "Error: Failed to get Git root for $target_dir"
            return 1
        fi
    fi

    (
        export GIT_DIR="$git_dir"
        export GIT_WORK_TREE="$target_dir"

        local git_args=(-z --exclude-standard)
        if ! $TRACKED_ONLY; then
            git_args+=(--cached --others)
        fi

        apply_custom_ignore

        v_echo "PATHSPEC: ${PATHSPEC[*]}"
        v_echo "Running: git ls-files ${git_args[*]} -- ${PATHSPEC[*]}"
        git ls-files "${git_args[@]}" -- "${PATHSPEC[@]}" | tr '\0' '\n'
    )

    local exit_status=$?
    if [ -n "$temp_dir" ]; then
        rm -rf "$temp_dir" || v_echo "Warning: Failed to remove temporary directory $temp_dir"
    fi
    return $exit_status
}

# If no specific files or directories were specified by the user,
# set PATHSPEC to "*" to include all files in the current directory
if [ ${#PATHSPEC[@]} -eq 0 ]; then
    PATHSPEC=("*")
fi

# Append the default exclusion patterns to PATHSPEC
# This ensures that commonly ignored files (e.g., .git, node_modules)
# are always excluded from the search, regardless of user input
PATHSPEC+=("${DEFAULT_EXCLUDES[@]}")

# Main processing logic
if $USE_XML_TAGS; then
    root_path="$(get_home_relative_dirpath "$DIRECTORY")"
    echo "<root path=\"$root_path\">"
fi

get_files "$DIRECTORY" | while read -r file; do
    full_path="$DIRECTORY/$file"
    if [ -f "$full_path" ]; then
        process_file "$full_path" "$DIRECTORY"
    elif $VERBOSE; then
        echo "Skipping non-existent file: $full_path" >&2
    fi
done

if $USE_XML_TAGS; then
    echo "</root>"
fi
