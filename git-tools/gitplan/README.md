# GitPlan

GitPlan is a Go-based CLI tool that generates candidate git commands based on natural language prompts. It helps users find the right git command for their intended operation by analyzing the current git context and the user's description of what they want to do.

## Features

- Accepts natural language input describing a git operation
- Evaluates read-only git commands for context preparation
- Generates and presents candidate git commands that match the user's intent
- Provides explanations for each candidate command
- Requires explicit user confirmation for any file-mutating operations
- Estimates user preference for selected commands and optionally asks for feedback
- Stores user preference data in a plain text file in the user's home directory
- Includes an option to disable preference tracking

## Installation

1. Ensure you have Go 1.20 or later installed on your system.
2. Clone this repository:
   ```
   git clone https://github.com/yourusername/gitplan.git
   ```
3. Change to the project directory:
   ```
   cd gitplan
   ```
4. Build the project:
   ```
   go build
   ```

## Usage

Run GitPlan with a natural language prompt describing the git operation you want to perform:

```
./gitplan "show me the recent commits"
```

GitPlan will analyze your current git context and present a list of candidate git commands that match your intent. You can then select a command to execute or quit the program.

To disable preference tracking, use the `-no-pref` flag:

```
./gitplan -no-pref "create a new branch"
```

## User Preference Data

GitPlan stores user preference data in a file named `.gitplan_preferences.txt` in the user's home directory. This file contains JSON-formatted entries with the user's prompts, selected commands, and feedback ratings.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

