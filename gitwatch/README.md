# Gitwatch

A simple, real-time Git repository visualization tool that displays commit history and worktree information in the terminal.

## Features

- Shows Git commit history with graph visualization
- Displays worktree information
- Auto-refreshes to show latest changes
- Multiple display formats with auto-rotation
- Compact vertical spacing option
- Configurable number of commits and refresh rate
- Uses Git's native coloring and formatting

## Usage

```
./gitwatch [flags]
```

### Flags

- `-n <number>`: Number of commits to display (default: 20)
- `-r <duration>`: Refresh rate (e.g. 1s, 500ms) (default: 2s)
- `-compact`: Use compact vertical spacing for worktrees
- `-rotate`: Automatically rotate through different display formats
- `-format <number>`: Select a specific format (0-3)
  - Format 0: Standard with hash, refs, message, time, and author
  - Format 1: Compact with hash, refs, and message
  - Format 2: Detailed with hash, refs, message, date, author, and email
  - Format 3: Stats-focused with additions/deletions

### Examples

```
# Show 10 commits with default format
./gitwatch -n 10

# Compact mode with format rotation
./gitwatch -compact -rotate

# Use detailed format
./gitwatch -format 2
```

## Building from Source

```
go build
```

## Implementation

Gitwatch directly runs Git commands with proper formatting to display commit history and worktree information.
It uses Git's built-in ANSI coloring for a clean, readable display.