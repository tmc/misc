/*
Clean-cc-git-history removes AI-generated and co-authorship attribution
lines from Git commit messages while preserving original commit information
through Git notes.

The tool processes Git commits on the current branch, removing lines containing
"Generated with" and "Co-Authored-By" from commit messages. By default, it only
processes unpushed commits. The original commit SHA and modification history are
preserved through Git notes for auditability.

# Usage

	clean-cc-git-history [flags]

The tool operates on the current Git repository and branch.

# Flags

The following flags control the tool's behavior:

	-all
		Process all commits on the current branch, not just unpushed ones.

	-dry-run
		Show what would be changed without making any modifications (default: true).

	-run
		Actually execute the changes (overrides dry-run).

	-verbose
		Enable verbose output showing detailed processing information.

	-help
		Show usage information.

	-limit N
		Limit the number of commits to process (0 = no limit, default: 0).

	-msg-command "command"
		Command to generate new commit message (receives cleaned message on stdin).

	-msg-command-limit N
		Limit the number of times msg-command is invoked (0 = no limit, falls back to cleaned messages).

	-msg-use-git-auto-commit-message
		Use git-auto-commit-message --message-only (shortcut for -msg-command).

	-generate-script
		Generate a bash script to review and run manually.

# Behavior

The tool performs the following operations for each qualifying commit:

 1. Identifies commits containing "🤖 Generated with" or "Co-Authored-By" lines
 2. Uses interactive rebase to reword commit messages, removing matching lines
 3. Optionally generates new commit messages using a custom command

Lines are removed if they match these patterns (case-insensitive):
  - Lines containing "🤖 Generated with" (or similar variations)
  - Lines starting with "Co-Authored-By"

When using -msg-command, the tool can generate entirely new commit messages
based on the cleaned content. This is useful for creating more descriptive
commit messages after removing attribution lines.

# Examples

Preview changes for unpushed commits (default behavior):

	clean-cc-git-history

Actually clean unpushed commits:

	clean-cc-git-history -run

Process all commits on the current branch:

	clean-cc-git-history -all -run

Preview changes with verbose output:

	clean-cc-git-history -verbose

Generate new commit messages using git-auto-commit-message:

	clean-cc-git-history -msg-use-git-auto-commit-message -run

Limit processing to first 5 commits with custom message generation:

	clean-cc-git-history -limit 5 -msg-command "my-commit-msg-tool" -run

Generate a reviewable script:

	clean-cc-git-history -generate-script > review-and-clean.sh

# Implementation Details

This tool uses Git's interactive rebase feature to modify commit messages.
Unlike approaches that create entirely new commits, rebase preserves more
of the original commit metadata while still allowing message modification.

The tool creates temporary shell scripts during execution:
  - A sequence editor to mark commits for rewording
  - A commit editor to apply the cleaned messages

When using -generate-script, these operations are packaged into a
standalone bash script for review and manual execution.

# Requirements

The tool requires:
  - Must be run from within a Git repository
  - Git must be available in PATH
  - Current branch must have commits to process
  - For unpushed commit detection, a remote repository must be configured

# Exit Codes

The tool uses the following exit codes:

	0	Success
	1	General error (invalid arguments, Git errors, etc.)
	2	Not in a Git repository
	3	No commits found to process

# Warning

This tool rewrites Git history. Ensure you have backups and coordinate with
team members before running on shared branches. Never run on commits that have
already been pushed to shared repositories unless you coordinate with all
collaborators.
*/
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md
