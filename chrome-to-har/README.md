# chrome-to-har

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/chrome-to-har.svg)](https://pkg.go.dev/github.com/tmc/misc/chrome-to-har)

Command chrome-to-har records browser activity and generates HAR (HTTP Archive) files.

This tool launches Chrome/Chromium, navigates to specified URLs, and captures all
network traffic in the standard HAR format. It supports both interactive browsing
and automated capture modes.

Usage:

    chrome-to-har [flags] [URL...]

Common flags:

    -o file        Output HAR file (default: output.har)
    -profile name  Use specific Chrome profile
    -timeout sec   Global timeout in seconds (default: 30)
    -headless      Run in headless mode
    -filter regex  Filter requests by URL pattern
    -block regex   Block requests matching pattern
    -interactive   Enable interactive JavaScript mode

Examples:

    # Record a single page
    chrome-to-har https://example.com

    # Use a specific Chrome profile
    chrome-to-har -profile "Default" https://github.com

    # Filter only API requests
    chrome-to-har -filter "api\." https://example.com

    # Block tracking scripts
    chrome-to-har -block "analytics|tracking" https://news.site.com

    # Interactive mode with JavaScript console
    chrome-to-har -interactive https://example.com

The tool captures detailed timing information, request/response headers,
cookies, and response content. The output HAR file can be analyzed using
various HAR viewers and development tools.

Interactive mode provides a JavaScript console for executing commands in
the browser context, useful for debugging and automation tasks.
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
go install github.com/tmc/misc/chrome-to-har@latest
```

### Run directly

```bash
go run github.com/tmc/misc/chrome-to-har@latest [arguments]
```

