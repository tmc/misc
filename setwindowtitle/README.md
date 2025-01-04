# setwindowtitle

Simple utility to set the terminal window title.

## Usage

```
setwindowtitle "New Window Title"
```

## Terminal Support

- iTerm2: Uses proprietary escape sequences for enhanced support
- Kitty: Uses Kitty-specific terminal protocol
- Windows Terminal: Uses ANSI escape sequences
- Traditional Windows Console: Uses Win32 API
- Other terminals (xterm, Terminal.app, gnome-terminal, etc): Uses standard ANSI escape sequences

## Installation

```
go install github.com/tmc/misc/setwindowtitle@latest
```

## Notes

The program automatically detects the terminal type and uses the most appropriate method to set the window title. For iTerm2, it sets both the badge and window title for better visibility.

