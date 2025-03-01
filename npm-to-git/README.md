# npm-to-git

A tool to convert npm packages to git repositories and track package evolution over time.

## Installation

```
go install github.com/tmc/misc/npm-to-git@latest
```

Or clone the repository and build it:

```
git clone https://github.com/tmc/misc/npm-to-git.git
cd npm-to-git
go build
```

## Features

- Convert any npm package to a git repository
- Download and track all published versions of a package
- Create a unified git repository with tags for each version
- Monitor packages for updates with automatic notifications
- Analyze code changes between versions
- Extract API evolution (functions and classes added/removed)
- Voice notifications (on macOS) when changes are detected

## Usage

### Basic Usage

```
npm-to-git -package <npm-package-name> [-output <output-directory>]
```

### Track All Versions

```
npm-to-git -package <npm-package-name> -all-versions [-output <output-directory>] [-history <history-file>]
```

### Monitor Package for Updates

```
npm-to-git -package <npm-package-name> -monitor [-output <output-directory>] [-history <history-file>]
```

### Options

- `-package`: The npm package name to convert to git (required)
- `-output`: Output directory (default: package name)
- `-all-versions`: Download and analyze all versions of the package
- `-monitor`: Start monitoring mode to check for new versions
- `-history`: Path to history file (default: .npm-to-git-history.txt)

### Example

```
npm-to-git -package lodash -all-versions
```

This will:
1. Download all versions of lodash from npm
2. Create a git repository with each version as a separate tagged commit
3. Analyze differences between versions
4. Create a detailed history file with analysis results

## Use Cases

- Create a git repository from an npm package for customization
- Examine the code of an npm package more easily
- Create a personal fork of an npm package
- Track all versions of a package to analyze its evolution
- Monitor important packages for updates and changes
- Analyze API changes between versions

## Requirements

- Go 1.16+
- Git
- npm