# find-long-go-tests

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/find-long-go-tests.svg)](https://pkg.go.dev/github.com/tmc/misc/find-long-go-tests)

Command find-long-go-tests identifies Go test functions that are skipped in short mode.

It scans Go test files and reports any test functions that contain the pattern:

	if testing.Short() {
		t.Skip(...)
	}

Usage:

	find-long-go-tests [-p] [path...]

The -p flag outputs the test names as a single line suitable for use with go test -run:

	find-long-go-tests -p    # outputs: TestLongA|TestLongB|TestLongC

Without -p, each test name is printed on its own line:

	find-long-go-tests       # outputs: TestLongA
	                        #          TestLongB
	                        #          TestLongC

Examples:

	# Run only the long tests:
	go test $(go list ./...) -run "$(find-long-go-tests -p)"

	# Skip the long tests:
	go test $(go list ./...) -run "^((?!$(find-long-go-tests -p)).)*$"
## Installation

<details>
<summary><b>Prerequisites: Go Installation</b></summary>

You'll need Go 1.20 or later. [Install Go](https://go.dev/doc/install) if you haven't already.

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

```console
go install github.com/tmc/misc/find-long-go-tests@latest
```

### Run directly

```console
go run github.com/tmc/misc/find-long-go-tests@latest [arguments]
```

