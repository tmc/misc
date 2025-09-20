/*
Package execgo provides functionality to execute Go code directly.

getgo simplifies the process of obtaining and setting up the Go development environment
by automating the download and installation of the official Go distribution.

Usage:

    getgo install [version]

If no version is specified, the latest stable release will be installed.
The tool will:
  - Detect the host operating system and architecture
  - Download the appropriate Go distribution
  - Verify the download checksum
  - Install Go to the standard location for the platform
  - Configure basic environment variables

Example:

    getgo install 1.20.3
    getgo install latest

The tool supports common platforms including:
  - Linux (amd64, arm64, 386)
  - macOS (amd64, arm64)
  - Windows (amd64, 386)

Environment Variables:

    GETGO_INSTALL_DIR    Override default installation directory
    GETGO_NO_VERIFY     Skip checksum verification (not recommended)

The default installation directories are:
  - Linux/macOS: /usr/local/go
  - Windows: C:\Go

After installation, users should add the Go binary directory to their PATH:
  - Linux/macOS: export PATH=$PATH:/usr/local/go/bin
  - Windows: Add C:\Go\bin to system PATH

For more details see: https://golang.org/doc/install

*/
package execgo