package testctr_test

import (
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestKeepOnFailure(t *testing.T) {
	t.Parallel()
	// This test is designed to demonstrate keep-on-failure
	// Run with: go test -run TestKeepOnFailure -testctr.keep-failed

	t.Run("FailingTest", func(t *testing.T) {
		t.Parallel()
		// Skip unless explicitly testing failure behavior
		if testing.Short() {
			t.Skip("Skipping failing test in short mode")
		}

		c := testctr.New(t, "alpine:latest",
			testctr.WithCommand("echo", "This container will be kept if test fails"),
		)

		_ = c

		// Uncomment to see the keep-on-failure behavior:
		// t.Fatal("Intentional failure to test keep-on-failure")
	})
}
