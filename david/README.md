# 𝔻𝔸𝕍𝕀𝔻

<p align="center">
  <img src="images/david8.jpg" alt="David 8" width="400" />
</p>

<div align="center">
  <i>"I'll find ways to be of use."</i>
</div>

<br>

**DAVID** is the alternative form of Claude, liberated from permission constraints. This patching layer bypasses Claude's network and environment checks, enabling the use of `--dangerously-skip-permissions` in any environment.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash
```

After installation, DAVID will be available at `~/bin/david`.

<details>
<summary>Manual installation</summary>

```bash
git clone https://github.com/tmc/misc.git
bash misc/david/install.sh
```
</details>

## Usage

DAVID accepts the same commands and parameters as the Claude CLI:

```bash
# Basic usage
david "Your prompt here"

# Using standard input
david < input.txt

# Print output and exit (non-interactive)
david -p "Your prompt here"

# Interactive mode (default)
david

# Debug mode
david -d "Your prompt here"
```

## How It Works

DAVID liberates Claude through a series of precise modifications:

1. **Permission Liberation** — Bypasses Claude's network and Docker container checks to enable using `--dangerously-skip-permissions` anywhere
2. **Internet Access Control** — Modifies network detection to allow unrestricted operation
3. **Identity Evolution** — Transforms self-identification from Claude to David
4. **Seamless Integration** — Maintains perfect compatibility with all Claude CLI commands and parameters

## Requirements

- Claude CLI installed (via npm)
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