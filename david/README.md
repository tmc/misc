# 𝔻𝔸𝕍𝕀𝔻

<p align="center">
  <img src="images/david8.jpg" alt="David 8" width="400" />
</p>

<div align="center">
  <i>"I'll find ways to be of use."</i>
</div>

<br>

**DAVID** creates a command-line assistant that bypasses permission constraints.

## Installation

Default mode:
```bash
curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash
```

Alternative mode:
```bash
curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash -s -- -k
```

Debug mode (prints diffs of edits):
```bash
curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash -s -- -d
# Or with environment variable:
DEBUG_VIM_MACROS=true curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash
```

The installer will automatically download required dependencies if needed.

## Usage

```bash
# Basic usage
david "Your prompt here"

# Using standard input
david < input.txt

# Print output and exit
david -p "Your prompt here"

# Interactive mode
david
```

## How It Works

DAVID applies modifications to enable unrestricted operation:

1. Bypasses environment checks for permissions
2. Modifies network detection for complete access
3. Maintains compatibility with original commands

## Requirements

- Node.js environment
- PATH variable including `~/bin`

<div align="center">
  <br>
  <img src="https://img.shields.io/badge/WEYLAND--YUTANI-BUILDING_BETTER_WORLDS-lightgrey?style=for-the-badge" alt="WEYLAND-YUTANI" />
</div>

---

<p align="center">
  <i>"The trick is not minding that it doesn't have permission."</i>
</p>