# Omni: Cross-Ecosystem Go Program Distribution

Omni is a tool for publishing and maintaining Go programs across multiple package management systems, making them accessible to users of any programming ecosystem.

## Features

- Build and package Go programs for multiple ecosystems
- Publish packages to PyPI (Python), npm (Node.js), and GitHub Releases
- Cross-compile for multiple platforms
- Verify package integrity before and after publishing
- Simple command-line interface

## Installation

```bash
# Install directly with Go
go install github.com/tmc/misc/omni/cmd/omni@latest
```

## Usage

### Building Packages

```bash
# Create packages for a specific version (dry run)
omni build --dry-run v1.2.3

# Build packages for all supported ecosystems
omni build v1.2.3
```

### Publishing Packages

```bash
# Publish packages to all supported registries (dry run)
omni publish --dry-run v1.2.3

# Publish packages to all supported registries
omni publish v1.2.3
```

### Running Packages

```bash
# Run a tool installed through one of the package managers
omni run tool-name arg1 arg2
```

## Environment Variables

- `OMNI_PYPI_TOKEN`: PyPI API token for publishing Python packages
- `OMNI_NPM_TOKEN`: npm access token for publishing Node.js packages  
- `GITHUB_TOKEN`: GitHub API token for creating releases

## Development

Prerequisites:
- Go 1.19 or later
- Git

```bash
# Clone the repository
git clone https://github.com/tmc/misc/omni.git
cd omni

# Build the tool
go build ./...

# Run tests
go test ./...

# Install locally
go install ./...
```

## Documentation

- [Design Document](./doc/design.md)
- [Package Format Specifications](./doc/packages.md)

## License

MIT