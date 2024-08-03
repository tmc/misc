Based on the provided `main.go` file, here's an updated README.md for the template-populator project:

```markdown
# Template Populator

Template Populator is a simple command-line tool written in Go that replaces placeholders in a template file with values from environment variables and command-line flags.

## Features

- Reads template from a file or stdin
- Replaces placeholders with values from environment variables
- Replaces placeholders with values from command-line flags
- Supports strict mode for error handling
- Provides warnings for unreplaced placeholders

## Usage

```
template-populator [flags]
```

### Flags

- `-template string`: Path to the template file (use '-' for stdin) (default "-")
- `-strict`: Exit with an error if any placeholder is not replaced

Any additional flags provided will be used to replace placeholders in the template.

## Placeholder Format

Placeholders in the template should be in the format `{{placeholder_name}}`.

## Examples

1. Using a template file:

```
template-populator -template input.txt -name John -age 30
```

2. Using stdin:

```
echo "Hello, {{name}}! You are {{age}} years old." | template-populator -name Alice -age 25
```

3. Using environment variables:

```
export GREETING="Hello"
export NAME="Bob"
echo "{{GREETING}}, {{NAME}}!" | template-populator
```

4. Using strict mode:

```
template-populator -template input.txt -strict -name John
```

## Building

To build the project, make sure you have Go installed, then run:

```
go build
```

## Running

After building, you can run the executable:

```
./template-populator [flags]
```

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here]
```

This README provides an overview of the Template Populator tool, its features, usage instructions, examples, and basic information about building and running the project. You may want to add more specific details about licensing, contribution guidelines, or any other relevant information for your project.