# code-to-gpt.sh

`code-to-gpt.sh` is a Bash script designed to read and print the contents of text files in a specified directory, while excluding certain directories and files. This script is useful for processing and displaying text files, ignoring specified patterns and directories.

## Features

- Recursively reads and prints text files in a directory.
- Excludes specified directories and files.
- Supports `.gitignore` patterns to exclude files.
- Customizable through command-line arguments.

## Usage

```bash
./code-to-gpt.sh [--exclude-dir <directory>]
```

### Options

- `--exclude-dir <directory>`: Add a directory to the list of directories to exclude.

## Default Configuration

- `DIRECTORY="."`: The default directory to process. You can replace this with your specific directory.
- `EXCLUDE_DIRS=("node_modules")`: An array of
 directories to exclude.
- `IGNORED_FILES=("go.sum" "package.json")`: An array of files to exclude.

## How It Works

1. **Directory Exclusion**: The script checks if a directory is in the exclude list and skips it if it is.
2. **.gitignore Patterns**: Reads `.gitignore` files in each directory and adds the patterns to an ignore list.
3. **File Processing**: Reads and prints text files, skipping those that match the ignore patterns or are in the exclude list.
4. **Recursive Processing**: Processes subdirectories recursively, applying the same rules.

## Example

To run the script and exclude the `build` directory:

```bash
./code-to-gpt.sh --exclude-dir build
```

## Script Breakdown

- **is_excluded_dir**: Checks if a directory is in the exclude list.
- **read_gitignore**: Reads `.gitignore` files and adds patterns to the ignore list.
- **is_ignored_file**: Checks if a file matches any of the ignore patterns.
- **read_directory_files**: Reads and prints files in a directory, recursively processing subdirectories.

## License

ISC License