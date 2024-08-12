# Git Goals Usage Examples

Here are some examples of how to use git-goals:

## Create a new goal
```
$ git goals create "Implement new feature"
Created new goal with ID: 20240101000000
Description: Implement new feature
```

## List all goals
```
$ git goals list
Current Goals:
- 20240101000000 (active): Implement new feature
```

## Show goal details
```
$ git goals show 20240101000000
Goal Details:
=============
id: 20240101000000
type: goal
description: Implement new feature
status: active
created_at: 2024-01-01
```

## Update a goal
```
$ git goals update 20240101000000 "Implement new feature with improved performance"
Updated goal 20240101000000: Implement new feature with improved performance
```

## Complete a goal
```
$ git goals complete 20240101000000 "" "Feature implemented and tested"
Goal 20240101000000 marked as complete
Rationale: Feature implemented and tested
```

## Generate a report
```
$ git goals report
Goal Report
===========
Goal ID: 20240101000000
Description: Implement new feature with improved performance
Status: completed
Created: 2024-01-01
Completed: 2024-01-01
---
```

## Prioritize a goal
## Set a deadline for a goal
```
$ git goals deadline 20240101000000 2024-12-31
Goal 20240101000000 deadline set to 2024-12-31
```

```
$ git goals prioritize 20240101000000 high
Goal 20240101000000 priority set to high
```

## Delete a goal
```
$ git goals delete 20240101000000
Goal 20240101000000 deleted
```

## Recover goals
```
$ git goals recover
Processing note for commit <commit_hash>
Recovered goal:
id: 20240101000001
type: goal
description: Recovered goal description
status: active
created_at: 2024-01-01
---
```

For more detailed information on each command, use the --help option:
```
$ git goals <command> --help
```

### Prioritize a goal (coming soon)
## Set a deadline for a goal
```
$ git goals deadline 20240101000000 2024-12-31
Goal 20240101000000 deadline set to 2024-12-31
```


```
git goals prioritize <goal_id> <priority>
```

### Set a deadline for a goal (coming soon)

```
git goals deadline <goal_id> <deadline>
```


## Check for approaching deadlines
```
$ git goals notify
Checking for approaching deadlines...
WARNING: Goal 20240101000000 is due in 5 days!
ALERT: Goal 20240101000001 is overdue by 2 days!
```
