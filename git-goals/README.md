# git-goals v0.2.10

# git-goals

git-goals is a set of command-line tools to manage and track goals within a Git repository. It allows you to create, update, list, and complete goals, as well as generate reports on your progress.

## Installation

1. Clone this repository or download the scripts.
2. Add the directory containing these scripts to your PATH.
3. Ensure the scripts are executable (`chmod +x git-goals*`).

## Testing

To run the test suite, use the following command:

```bash
./test-git-goals.sh
```


## Features
- Priority sorting in list command
- Configurable notification system for approaching and overdue goals
- Configurable notification system for approaching and overdue goals
- Configurable notification system for approaching and overdue goals
- Deadline notification system for approaching and overdue goals
- Deadline notification system

- Goal creation and management
- Goal prioritization
- Deadline tracking
- Goal completion tracking
- Reporting and analytics

## Usage

### Create a new goal

```
git goals create <goal_description>
```

### Update a goal

```
git goals update <goal_id> <new_goal_description>
```

### List goals

```
git goals list
```

### Show goal details

```
git goals show <goal_id>
```

### Prioritize a goal
### Set a deadline for a goal

```
git goals deadline <goal_id> <deadline>
```


```
git goals prioritize <goal_id> <priority>
```

### Complete a goal

```
git goals complete <goal_id> [attempt_id] [rationale]
```

### Delete a goal

```
git goals delete <goal_id>
```

### Generate a report

```
git goals report
```

### Recover goals

```
git goals recover
```

## How it works

git-goals uses Git notes to store goal metadata. Each goal is associated with a specific commit, and the goal information is stored as a note on that commit. The tools provided allow you to manage these notes and the associated goal branches easily.

## Contributing
Please see the [Contributing Guide](docs/CONTRIBUTING.md) for details on how to contribute to this project.

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT

## Running Tests

To run the test suite and verify the functionality of git-goals, use the following command:

```bash
./test-git-goals.sh
```

This will run through a series of tests to ensure all commands are working as expected.

## Plugin System

git-goals now supports a plugin system for extending functionality. To create a plugin:

1. Create a new file in the  directory with a  extension.
2. Implement your plugin logic.
3. Use the  function to register new subcommands.

Example plugin (plugins/hello_world.sh):

```bash
#!/bin/bash
git_goals_hello_world() {
    echo "Hello from the git-goals plugin system!"
}

git_goals_register_command "hello" "git_goals_hello_world" "Print a hello message"
```

Plugins are automatically loaded and new commands become available in the git-goals CLI.

