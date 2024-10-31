# Scripttestctl

Scripttestctl is a command-line tool for managing scripttest testing in a codebase. It provides functionality for running, generating, and managing scripttests with AI assistance.

## Installation

1. Clone the repository:
   ```shell
   go install https://github.com/tmc/misc/scripttestctl@latest
   ```
## Usage

Scripttestctl supports the following commands:

- `run`: Run existing scripttest files
- `generate`: Generate new scripttest files
- `init`: Initialize the tool and set up dependencies
- `prompts`: View and manage AI prompts used by the tool

### Examples

1. Run tests:
   ```
   scripttestctl run 'testdata/*.txt'
   ```

2. Generate a new test:
   ```
   scripttestctl generate "Test file creation and content verification" new_test.txt
   ```

3. Initialize the tool:
   ```
   scripttestctl init
   ```

4. View available prompts:
   ```
   scripttestctl prompts
   ```

## Customizing Prompts

You can customize the AI prompts used by scripttestctl in three ways:

1. Set environment variables:
   ```
   export SCRIPTTESTCTL_PROMPT_INIT="Your custom init prompt"
   ```

2. Create a `.scripttestctlrc` file in your home directory with YAML or JSON-formatted prompts:
   ```yaml
   Init: "Your custom init prompt"
   GenerateTest: "Your custom generate test prompt"
   ```

3. Set the `SCRIPTTESTCTL_RC` environment variable to specify a custom rc file location:
   ```
   export SCRIPTTESTCTL_RC="/path/to/your/custom/rcfile"
   ```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

