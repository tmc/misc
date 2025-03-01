# Omni Design Document

## Overview

Omni is a tool designed to bridge the gap between the Go ecosystem and other popular programming language ecosystems. It allows Go programs to be easily packaged and distributed through multiple package managers, making them accessible to users of different programming languages without requiring them to install Go.

## Motivation

Go developers often create useful command-line tools that could benefit users of other programming languages. However, distribution is typically limited to:

1. Users who have Go installed (`go install`)
2. Binary releases that users need to manually download and install
3. OS-specific package managers (Homebrew, apt, etc.)

Omni addresses this problem by enabling Go programs to be easily packaged and published through language-specific package managers like PyPI (Python) and npm (Node.js), allowing users to install Go tools using their preferred package management systems.

## Core Principles

1. **Simplicity**: Omni should be simple to use for both authors and consumers
2. **Reliability**: Packages should work consistently across platforms
3. **Compatibility**: Follow each ecosystem's best practices and conventions
4. **Extensibility**: Design for adding support for more package managers
5. **Security**: Ensure secure handling of credentials and artifacts

## Architecture

### Components

1. **Command Interface**: The `omni` CLI tool
2. **Validator**: Validates input parameters and project state
3. **Builder**: Cross-compiles Go binaries for various platforms
4. **Packager**: Creates packages for each supported ecosystem
5. **Publisher**: Publishes packages to their respective registries

### High-Level Flow

```
                                            ┌──────────┐
                                            │  GitHub  │
                                            │ Releases │
                                            └────▲─────┘
                                                 │
                                            ┌────┴─────┐
                    ┌───────────┐          │  GitHub   │
┌─────────┐  build  │  ┌─────┐  │  publish │ Publisher │
│ Command ├─────────┼─►│Build├──┼──────────┤           │
│Interface│         │  └─────┘  │          └────┬─────┘
└─────────┘         │           │               │
                    │ ┌───────┐ │          ┌────▼─────┐
                    │ │Package│ │          │   PyPI    │
                    │ └───┬───┘ │          │ Publisher │
                    │     │     │          └────┬─────┘
                    │     ▼     │               │
                    │ ┌───────┐ │          ┌────▼─────┐
                    │ │Verify │ │          │   npm     │
                    │ └───────┘ │          │ Publisher │
                    └───────────┘          └──────────┘
```

## Package Format

### Python Package

A Python package contains:

1. A setup.py file for package metadata
2. A pyproject.toml file for modern Python packaging
3. The compiled Go binary for multiple platforms
4. A Python wrapper that selects and runs the appropriate binary

```
package/
├── setup.py
├── pyproject.toml
├── package_name/
│   ├── __init__.py    # Contains main() entry point
│   └── bin/           # Contains platform-specific binaries
│       ├── darwin_amd64/executable
│       ├── darwin_arm64/executable
│       ├── linux_amd64/executable
│       ├── linux_arm64/executable
│       └── windows_amd64/executable.exe
```

### Node.js Package

A Node.js package contains:

1. A package.json file for package metadata
2. The compiled Go binary for multiple platforms
3. A JavaScript wrapper that selects and runs the appropriate binary

```
package/
├── package.json
├── index.js         # Contains wrapper code
└── bin/             # Contains platform-specific binaries
    ├── darwin_amd64/executable
    ├── darwin_arm64/executable
    ├── linux_amd64/executable
    ├── linux_arm64/executable
    └── windows_amd64/executable.exe
```

## Workflow

### Building Packages

1. Validate input parameters and repository state
2. Cross-compile the Go binary for all supported platforms
3. Generate package metadata files using templates
4. Package files according to each ecosystem's conventions
5. Verify that packages are correctly structured

### Publishing Packages

1. Authenticate with package registries using provided credentials
2. Upload packages to their respective registries
3. Create a GitHub release with the built packages as assets
4. Verify that packages were successfully published and are accessible

## Roadmap

1. **Phase 1**: Support for PyPI and npm packaging
2. **Phase 2**: Add support for Homebrew formula generation
3. **Phase 3**: Support for Debian and RPM packaging
4. **Phase 4**: Plugin system for additional package formats

## Challenges and Solutions

### Binary Size

Go binaries can be large, which may be a concern for language-specific package managers where users expect smaller packages.

**Solution**: Implement optional binary compression and provide clear documentation about package size expectations.

### Platform Compatibility

Ensuring the correct binary is selected and executed on all supported platforms.

**Solution**: Implement robust platform detection in wrapper scripts and test extensively across platforms.

### Security

Secure handling of publishing credentials.

**Solution**: Use environment variables for credentials, never store them in the repository or configuration files.

## Implementation Details

See the code implementation for detailed technical specifications.

## Conclusion

Omni provides a straightforward solution for distributing Go programs across multiple language ecosystems, enhancing Go's interoperability with other programming communities. By following each ecosystem's conventions and practices, it creates packages that feel native to users of other languages while maintaining the performance and reliability benefits of Go.