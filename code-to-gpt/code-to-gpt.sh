#!/usr/bin/env bash
# code-to-gpt.sh
# Process files for LLM input, respecting .gitignore and custom exclusions.
# Usage: code-to-gpt.sh [options] <directory1> [<directory2> ...] [<pathspec>...] [-- <pathspec>...]
#
# For full usage instructions, run: ./code-to-gpt.sh --help
set -euo pipefail

# Default exclude patterns
DEFAULT_EXCLUDES=(
    ':!node_modules/**'
    ':!venv/**'
    ':!.venv/**'
    ':!go.sum'
    ':!go.work.sum'
    ':!yarn.lock'
    ':!yarn.error.log'
    ':!package-lock.json'
    ':!uv.lock'
)
DIRECTORIES=()
COUNT_TOKENS=false
VERBOSE=true
USE_XML_TAGS=true
INCLUDE_SVG=false
INCLUDE_XML=false
WC_LIMIT=10000 # Maximum number of lines to process
TRACKED_ONLY=false
PATHSPEC=("${DEFAULT_EXCLUDES[@]}")

root_path=""

# Function to print usage information
print_usage() {
    echo "Usage: $0 [OPTIONS] <directory1> [<directory2> ...] [<pathspec>...] [-- <pathspec>...]"
    echo "Options:"
    echo "  --count-tokens: Count tokens instead of outputting file contents"
    echo "  --verbose: Enable verbose output"
    echo "  --no-xml-tags: Disable XML tags around content"
    echo "  --include-svg: Explicitly include SVG files"
    echo "  --include-xml: Explicitly include XML files"
    echo "  --tracked-only: Only include tracked files in Git repositories"
    echo "  <directory>: Specify one or more directories to process"
    echo "  <pathspec>: One or more pathspec patterns to filter files (overrides default excludes)"
    echo ""
    echo "Notes:"
    echo "  - Options must come before directories and pathspecs."
    echo "  - All arguments after the last directory are treated as pathspecs."
    echo "  - Use -- to explicitly mark the beginning of pathspecs, especially for paths starting with -."
    echo ""
    echo "Examples:"
    echo "  $0 --verbose /path/to/dir1 /path/to/dir2"
    echo "  $0 --verbose /path/to/dir1 /path/to/dir2 '*.js' '*.py'"
    echo "  $0 --verbose /path/to/dir1 /path/to/dir2 -- -file-with-dash.txt"
    echo "  $0 /path/to/dir1 /path/to/dir2 '*.txt' '!excluded.txt'"
    echo "  $0 /path/to/dir -- -file-with-dash.txt normal-file.txt"
}
# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count-tokens|--verbose|--no-xml-tags|--exclude-svg|--exclude-xml|--include-svg|--include-xml|--tracked-only)
            # Handle boolean options
            # Option name, converted to uppercase, with dashes replaced by underscores
            # e.g. --count-tokens -> COUNT_TOKENS
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
            PATHSPEC=("${DEFAULT_EXCLUDES[@]}" "$@")
            break
            ;;
        -*)
            echo "Unknown option: $1" >&2
            print_usage
            exit 1
            ;;
        *)
            if [[ -d "$1" ]]; then
                DIRECTORIES+=("$1")
                shift
            else
                PATHSPEC=("$@")
                break
            fi
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
    local file="$1"
    local root_path="$2"

    # Expand the tilde in root_path if present
    root_path="${root_path/#\~/$HOME}"

    local abs_file=$(realpath "$file")
    local rel_path="${abs_file#$root_path/}"

    if [ "$rel_path" = "$abs_file" ]; then
        echo "$abs_file"
    else
        echo "./$rel_path"
    fi
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
    local file="$1"
    local expanded_root_path="${root_path/#\~/$HOME}"
    local relative_path=$(get_relative_path "$file" "$expanded_root_path")
    local mime_type=$(file -b --mime-type "$file")
    local line_count=$(wc -l < "$file")

    if ! is_text_file "$file"; then
        if $VERBOSE; then
            echo "Skipping non-text file: $relative_path (MIME: $mime_type)" >&2
        fi
        return
    fi

    if [ "$line_count" -gt "$WC_LIMIT" ]; then
        if $VERBOSE; then
            echo "Skipping large file: $relative_path ($line_count lines)" >&2
        fi
        return
    fi

    if $VERBOSE; then
        echo "Processing file: $relative_path (MIME: $mime_type)" >&2
    fi

    if $USE_XML_TAGS; then
        echo "<file path=\"$relative_path\">"
    fi

    if $COUNT_TOKENS; then
        token_count=$(tokencount "$file")
        echo "$token_count $relative_path"
    else
        cat "$file" | sed -e 's/^/  /'
    fi

    if $USE_XML_TAGS; then
        echo "</file>"
    fi
}

# Force off the use of XML tags if counting tokens:
if $COUNT_TOKENS; then
    USE_XML_TAGS=false
fi

is_git_repository() {
    git -C "$1" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

# Function to get files respecting Git ignore rules and pathspec
get_files() {
    if [ ${#PATHSPEC[@]} -gt 0 ]; then
        
        v_echo "Using pathspec: ${PATHSPEC[*]}"
        if git -C "$DIRECTORY" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
            v_echo "Using git ls-files with pathspec"
            git -C "$DIRECTORY" ls-files -z --exclude-standard "${PATHSPEC[@]}" | tr '\0' '\n'
        else
            v_echo "Using find with pathspec"
            find "$DIRECTORY" "${PATHSPEC[@]}" -type f
        fi
    else
        if git -C "$DIRECTORY" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
            if $TRACKED_ONLY; then
                git -C "$DIRECTORY" ls-files -z | tr '\0' '\n'
            else
                git -C "$DIRECTORY" ls-files -z --exclude-standard --others | tr '\0' '\n'
            fi
        else
            find "$DIRECTORY" -type f
        fi
    fi
}

# Main processing logic
# - Iterate over directories
# - For each directory, get files and process them
# - Optionally wrap the output in XML tags
# - Optionally count tokens instead of outputting file contents

# default to current directory if none are specified
if [ ${#DIRECTORIES[@]} -eq 0 ]; then
    DIRECTORIES=(".")
fi
# If we wrapping with xml, and have more than one directory, we need to wrap the output in a <source-code-roots> tag
if $USE_XML_TAGS; then
    if [ ${#DIRECTORIES[@]} -gt 1 ]; then
        echo "<source-code-roots>"
    fi
fi
for DIRECTORY in "${DIRECTORIES[@]}"; do
    if $USE_XML_TAGS; then
        root_path="$(get_home_relative_dirpath "$DIRECTORY")"
        echo "<root path=\"$root_path\">"
    fi
    if is_git_repository "$DIRECTORY"; then
        
        v_echo "Git repository detected in $DIRECTORY. Using git commands."
        (
            cd "$DIRECTORY" || exit 1
            get_files | while read -r file; do
                if [ -f "$file" ]; then
                    process_file "$file"
                elif $VERBOSE; then
                    echo "Skipping non-existent file: $file" >&2
                fi
            done
        )
    else
        
        v_echo "Processing directory: $DIRECTORY"
        get_files | while read -r file; do
            if [ -f "$DIRECTORY/$file" ]; then
                process_file "$DIRECTORY/$file"
            elif $VERBOSE; then
                echo "Skipping non-existent file: $DIRECTORY/$file" >&2
            fi
        done
    fi
    if $USE_XML_TAGS; then
        echo "</root>"
    fi
done
if $USE_XML_TAGS; then
    if [ ${#DIRECTORIES[@]} -gt 1 ]; then
        echo "</source-code-roots>"
    fi
fi
