# clean-cc-git-history

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/clean-cc-git-history.svg)](https://pkg.go.dev/github.com/tmc/misc/clean-cc-git-history)

Clean-cc-git-history removes AI-generated and co-authorship attribution
lines from Git commit messages while preserving original commit information
through Git notes.

The tool processes Git commits on the current branch, removing lines containing
"Generated with" and "Co-Authored-By" from commit messages. By default, it only
processes unpushed commits. The original commit SHA and modification history are
preserved through Git notes for auditability.
## Installation

<details>
<summary><b>Prerequisites: Go Installation</b></summary>

You'll need Go 1.21 or later. [Install Go](https://go.dev/doc/install) if you haven't already.

<details>
<summary><b>Setting up your PATH</b></summary>

After installing Go, ensure that `$HOME/go/bin` is in your PATH:

<details>
<summary><b>For bash users</b></summary>

Add to `~/.bashrc` or `~/.bash_profile`:
```bash
export PATH="$PATH:$HOME/go/bin"
```

Then reload your configuration:
```bash
source ~/.bashrc
```

</details>

<details>
<summary><b>For zsh users</b></summary>

Add to `~/.zshrc`:
```bash
export PATH="$PATH:$HOME/go/bin"
```

Then reload your configuration:
```bash
source ~/.zshrc
```

</details>

</details>

</details>

### Install

```bash
go install github.com/tmc/misc/clean-cc-git-history@latest
```

### Run directly

```bash
go run github.com/tmc/misc/clean-cc-git-history@latest [arguments]
```

## Usage


    clean-cc-git-history [flags]

The tool operates on the current Git repository and branch.

## Flags


The following flags control the tool's behavior:

    -all
    	Process all commits on the current branch, not just unpushed ones.

    -dry-run
    	Show what would be changed without making any modifications.

    -verbose
    	Enable verbose output showing detailed processing information.

    -help
    	Show usage information.

## Behavior


The tool performs the following operations for each qualifying commit:

 1. Identifies commits containing "Generated with" or "Co-Authored-By" lines
 2. Creates a new commit with the cleaned message (removing matching lines)
 3. Attaches two Git notes to the new commit:
    - "original-commit": Contains the SHA of the original commit
    - "clean-cc-tool": Records that this commit was modified by clean-cc-git-history

Lines are removed if they match these patterns (case-insensitive):
  - Lines starting with "Generated with"
  - Lines starting with "Co-Authored-By"

## Examples


Process unpushed commits on the current branch:

    clean-cc-git-history

Process all commits on the current branch:

    clean-cc-git-history -all

Preview changes without modifying the repository:

    clean-cc-git-history -dry-run -verbose

## Git Notes


This tool modifies Git history by creating new commits with cleaned messages.
The original commits are preserved through Git notes but are no longer part
of the active branch history.

Git notes are stored in the following refs:
  - refs/notes/original-commit: Maps new commits to original commit SHAs
  - refs/notes/clean-cc-tool: Records tool modification metadata

## Requirements


The tool requires:
  - Must be run from within a Git repository
  - Git must be available in PATH
  - Current branch must have commits to process
  - For unpushed commit detection, a remote repository must be configured

## Exit Codes


The tool uses the following exit codes:

    0	Success
    1	General error (invalid arguments, Git errors, etc.)
    2	Not in a Git repository
    3	No commits found to process

## Warning


This tool rewrites Git history. Ensure you have backups and coordinate with
team members before running on shared branches. Never run on commits that have
already been pushed to shared repositories unless you coordinate with all
collaborators.
## Types

### Commit

```go
type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    string
}
```

### NoCommitsError

```go
type NoCommitsError struct {
	// contains filtered or unexported fields
}
```

#### Methods

##### Error

```go
func (e *NoCommitsError) Error string
```

### NotInGitRepoError

```go
type NotInGitRepoError struct {
	// contains filtered or unexported fields
}
```

#### Methods

##### Error

```go
func (e *NotInGitRepoError) Error string
```

