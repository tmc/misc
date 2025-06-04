// Package cli provides the default CLI-based backend for testctr.
// This backend uses Docker/Podman/nerdctl command-line tools to manage containers.
//
// The CLI backend is the default when no other backend is specified.
// It automatically detects which container runtime is available in this order:
//  1. TESTCTR_RUNTIME environment variable
//  2. docker
//  3. podman
//  4. nerdctl
//
// This backend requires the appropriate CLI tool to be installed and accessible
// in the system PATH.
package cli
