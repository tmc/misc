# promptgen

promptgen is a Go program that generates prompts based on user input using the langchaingo library to interact with AI language models.

## Features

- Generates detailed and effective prompts for various purposes
- Uses the langchaingo library to interact with AI language models
- Supports custom temperature and max token settings
- Reads input from stdin or a specified file
- Implements streaming output for generated content
- Provides verbose logging option

## Installation

1. Ensure you have Go 1.22 or later installed on your system.
2. Clone this repository:
   ```
   git clone https://github.com/tmc/promptgen.git
   cd promptgen
   ```
3. Build the program:
   ```
   go build
   ```

## Usage

```
./promptgen [flags] < input.txt
```

Flags:
- `-temp float`: Set the temperature for AI generation (0.0 to 1.0, default: 0.1)
- `-max-tokens int`: Set the maximum number of tokens for the generated prompt (default: 2048)
- `-f string`: Input file (use '-' for stdin, default: '-')
- `-verbose`: Enable verbose logging

Examples:
```
echo "Create a writing prompt about time travel" | ./promptgen
./promptgen -f input.txt -temp 0.2 -max-tokens 4096
cat input.txt | ./promptgen -verbose
```

## Output

The program will generate a prompt based on the input and stream it to stdout.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
