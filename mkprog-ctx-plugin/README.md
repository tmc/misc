Certainly! I'll provide an updated README.md for the mkprog project based on the current state of the code and project structure. Here's the new README.md content:

=== README.md ===
# mkprog

mkprog is a Go program that generates structured content based on user input, specifically designed to create ctx plugins using AI language models via the langchaingo library.

## Features

- Generates complete, functional Go programs for ctx plugins based on user descriptions
- Uses the langchaingo library to interact with AI language models (Anthropic's Claude)
- Implements error handling and follows Go best practices
- Generates all necessary files for a runnable Go project, including main source file, README.md, LICENSE, and go.mod
- Supports custom system prompts and integrates with ctx plugin specifications
- Generates plugins with required flags (--capabilities, --plan-relevance) and placeholder logic

## Installation

1. Ensure you have Go 1.22 or later installed on your system.
2. Clone this repository:
   ```
   git clone https://github.com/tmc/mkprog.git
   cd mkprog
   ```
3. Build the program:
   ```
   go build
   ```

## Usage

```
./mkprog <plugin_name> <plugin_description>
```

- `<plugin_name>`: The name of the ctx plugin to be generated
- `<plugin_description>`: A description of the plugin's functionality

Example:
```
./mkprog my-ctx-plugin "A plugin that processes