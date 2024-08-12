## Example Usage

Here's an example of how to use git-goals:

```bash
# Create a new goal
$ git goals create "Implement new feature"
Created new goal with ID: 20230811123456
Description: Implement new feature

# List all goals
$ git goals list
Current Goals:
- 20230811123456 (active): Implement new feature

# Show goal details
$ git goals show 20230811123456
{
  "id": "20230811123456",
  "type": "goal",
  "description": "Implement new feature",
  "status": "active",
  "created_at": "2023-08-11"
}

# Update a goal
$ git goals update 20230811123456 "Implement new feature with improved performance"
Updated goal 20230811123456: Implement new feature with improved performance

# Complete a goal
$ git goals complete 20230811123456 "" "Feature implemented and tested"
Goal 20230811123456 marked as complete
Rationale: Feature implemented and tested

# Generate a report
$ git goals report
Goal Report
===========
Goal ID: 20230811123456
Description: Implement new feature with improved performance
Status: completed
Created: 2023-08-11
Completed: 2023-08-11
---

# Delete a goal
$ git goals delete 20230811123456
Goal 20230811123456 deleted
```

This example demonstrates the basic workflow of creating, updating, completing, and deleting a goal using git-goals.
