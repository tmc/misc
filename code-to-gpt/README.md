# code-to-gpt.sh

`code-to-gpt.sh` is a Bash script designed to prepare content from text files in a specified directory for feeding into a large language model. It can read and print the contents of text files or count tokens, while respecting Git ignore rules and excluding specified directories and files.

## Features

- Recursively processes text files in a directory.
- Respects Git ignore rules when in a Git repository.
- Excludes specified directories and files.
- Option to count tokens instead of outputting file contents.
- Verbose mode for detailed output.
- Customizable through command-line arguments.

## Usage

```bash
./code-to-gpt.sh [--count-tokens] [--exclude-dir <dir>] [--verbose] [<directory>]
```

### Options

- `--count-tokens`: Count tokens instead of outputting file contents.
- `--exclude-dir <dir>`: Add a directory to the list of directories to exclude.
- `--verbose`: Enable verbose output.
- `<directory>`: Specify the directory to process (default: current directory).

## Default Configuration

- `DIRECTORY="."`: The default directory to process.
- `EXCLUDE_DIRS=("node_modules" "venv" ".venv")`: An array of directories to exclude.
- `IGNORED_FILES=("go.sum" "go.work.sum" "yarn.lock" "yarn.error.log" "package-lock.json")`: An array of files to exclude.

## How It Works

1. **Git Integration**: Uses Git commands to list files when in a Git repository, respecting `.gitignore` rules.
2. **Directory Exclusion**: Checks if a directory is in the exclude list and skips it.
3. **File Processing**: Reads and prints text files or counts tokens, skipping those in the exclude list.
4. **Token Counting**: Uses the `tokencount` tool when the `--count-tokens` option is specified.

## Examples

To run the script and print file contents:

```bash
./code-to-gpt.sh
```

To count tokens and exclude the `build` directory:

```bash
./code-to-gpt.sh --count-tokens --exclude-dir build
```

To process a specific directory with verbose output:

```bash
./code-to-gpt.sh --verbose /path/to/directory
```

## Dependencies

- Bash
- Git (optional, for better file listing in Git repositories)
- Go (required for token counting)
- `tokencount` tool (automatically installed if not present when using `--count-tokens`)

## License

ISC License