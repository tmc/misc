# go-cov - Coverage-enabled Go installer

A tool that dynamically generates coverage-enabled Go installers, similar to `golang.org/dl/go1.24.3`.

## Usage

Install a coverage-enabled Go version:

```bash
go run go.tmc.dev/go-cov1.24.3@latest
```

This will:
1. Download the official Go source code
2. Compile it from source with `GOEXPERIMENT=coverageredesign` enabled
3. Install the coverage-enabled Go to `$HOME/sdk/go1.24.3-cov/`

## Setup

Add the coverage-enabled Go to your PATH:

```bash
export PATH=$HOME/sdk/go1.24.3-cov/bin:$PATH
```

## Available Versions

Any Go version available from https://go.dev/dl/ - just replace the version number:
- `go run go.tmc.dev/go-cov1.21.0@latest`
- `go run go.tmc.dev/go-cov1.22.5@latest`
- `go run go.tmc.dev/go-cov1.23.4@latest`
- `go run go.tmc.dev/go-cov1.24.3@latest`

## Architecture

This tool consists of:

1. **Dynamic module generation**: Each Go version gets its own module path with dynamically generated content
2. **Web service**: Serves module metadata and synthesizes installer programs on-the-fly
3. **Installer template**: A Go program template that gets customized for each version

## Deployment

### Run the service

```bash
go run cmd/server/main.go
```

The service handles:
- Module discovery (`?go-get=1`)
- Module proxy requests (`/@v/` endpoints) 
- Dynamic zip generation with version-specific installers

## How it works

Similar to `golang.org/dl`, this creates pseudo-modules for each Go version:

- `go.tmc.dev/go-cov1.24.3@latest` maps to a dynamically generated module
- The service synthesizes a `main.go` with the version embedded at request time  
- Users run `go run go.tmc.dev/go-cov1.24.3@latest` to download and execute the installer
- The installer downloads, extracts, and builds coverage-enabled Go to `$HOME/sdk/go1.24.3-cov/`

## Features

- ✅ Mimics golang.org/dl pattern exactly
- ✅ Downloads official Go source code
- ✅ Compiles from source with coverage redesign experiment
- ✅ Uses existing Go installation as bootstrap compiler
- ✅ Installs to separate SDK directory
- ✅ Supports any Go version dynamically
- ✅ Works with existing Go tooling (`go run`)
- ✅ No pre-generated files needed

## Requirements

- An existing Go installation (used as bootstrap compiler)
- C compiler (gcc/clang) for building Go from source
- Standard build tools (make, bash, etc.)

## Example

```bash
# Install coverage-enabled Go 1.24.3
go run go.tmc.dev/go-cov1.24.3@latest

# Use it
export PATH=$HOME/sdk/go1.24.3-cov/bin:$PATH
go version
# go version go1.24.3 darwin/amd64

# Now go test -cover will use the enhanced coverage
go test -cover ./...
```

## Dynamic Generation

The service dynamically generates installer programs by:

1. Extracting the Go version from the module path (`go-cov1.24.3` → `go1.24.3`)
2. Templating a complete Go source-building program with the version embedded
3. Creating a module zip with `go.mod` and `main.go` on-the-fly
4. Serving it through standard Go module proxy protocol

The generated installer:

1. Downloads Go source code (`.src.tar.gz`) instead of binaries
2. Finds an existing Go installation to use as bootstrap compiler
3. Compiles Go from source with `GOEXPERIMENT=coverageredesign`
4. Installs the coverage-enabled build to `$HOME/sdk/go1.X.Y-cov/`

This means any Go version can be supported without pre-generating files, and you get a fully coverage-enabled Go toolchain built specifically for your system.