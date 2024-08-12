# git-goals

git-goals is a set of command-line tools to manage and track goals within a Git repository. It allows you to create, update, list, and complete goals, as well as generate reports on your progress.

## Installation

1. Clone this repository or download the scripts.
2. Add the directory containing these scripts to your PATH.
3. Ensure the scripts are executable (`chmod +x git-goals-*`).

## Usage

### Create a new goal

```
git goals create <goal_description>
```

This creates a new goal branch with the given description. If you're already on a goal branch, it creates a subgoal.

### Update a goal

```
git goals update <new_goal_description>
```

Updates the current goal or creates a new one if it doesn't exist.

### List goals

```
git goals list
```

Displays a list of all current goals with their IDs, statuses, and descriptions.

### Show goal details

```
git goals show <goal_id>
```

Displays detailed information about a specific goal.

### Complete a goal

```
git goals complete <goal_id> [attempt_id] [rationale]
```

Marks a goal as complete, optionally with an attempt selection and rationale.

### Delete a goal

```
git goals delete <goal_id>
```

Deletes a goal by its ID.

### Generate a report

```
git goals report
```

Generates a comprehensive report of all goals, including their statuses, descriptions, creation dates, and completion dates.

## How it works

git-goals uses Git notes to store goal metadata. Each goal is associated with a specific commit, and the goal information is stored as a note on that commit. The tools provided allow you to manage these notes and the associated goal branches easily.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
