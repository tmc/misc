//go:build testcontainers
// +build testcontainers

package testctr_tests

import (
	_ "github.com/tmc/misc/testctr/testctr-testcontainers" // Register testcontainers backend
)

// This file ensures the testcontainers backend is registered when using the build tag