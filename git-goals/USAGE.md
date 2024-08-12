# git-goals Usage Guide

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/git-goals.git
   ```
2. Add the git-goals directory to your PATH:
   ```
   export PATH=$PATH:/path/to/git-goals
   ```
3. Ensure all scripts are executable:
   ```
   chmod +x /path/to/git-goals/git-goals*
   ```

## Basic Usage

### Create a new goal
```
git goals create "Implement new feature"
```

### List all goals
```
git goals list
```

### Show goal details
```
git goals show <goal_id>
```

### Update a goal
```
git goals update <goal_id> "Updated goal description"
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

For more detailed information on each command, use the --help option:
```
git goals <command> --help
```
