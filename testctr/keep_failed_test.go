package testctr_test

import (
	"os"
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestKeepOnFailure(t *testing.T) {
	t.Parallel()
	// This test is designed to demonstrate keep-on-failure
	// Run with: go test -run TestKeepOnFailure -testctr.keep-failed
	// Or set environment variable: TESTCTR_KEEP_FAILED=true

	t.Run("FailingTestToCheckKeepBehavior", func(t *testing.T) {
		t.Parallel()
		// Skip unless explicitly testing failure behavior, e.g., via a specific tag or env var for this test
		if testing.Short() && os.Getenv("TEST_KEEP_FAILED_BEHAVIOR") == "" {
			t.Skip("Skipping failing test in short mode unless TEST_KEEP_FAILED_BEHAVIOR is set")
		}

		c := testctr.New(t, "alpine:latest",
			testctr.WithCommand("sh", "-c", "echo This container will be kept if test fails && sleep 30"),
		)
		// Ensure container is not nil, to avoid nil pointer if New fails before the intended t.Fatal
		if c == nil {
			t.Log("Container creation itself failed, cannot test keep-on-failure behavior as intended.")
			return // or t.Fatal if New should always succeed here
		}

		// To observe the keep-on-failure behavior:
		// 1. Set TESTCTR_KEEP_FAILED=true (or use -testctr.keep-failed flag).
		// 2. Set TEST_KEEP_FAILED_BEHAVIOR=true (or remove the testing.Short() skip).
		// 3. Uncomment the t.Fatal line below.
		// t.Fatal("Intentional failure to test keep-on-failure")

		// If not intending to fail, add a small assertion or log.
		t.Logf("Container %s created for keep-on-failure test. To see behavior, uncomment t.Fatal and run with keep-failed enabled.", c.ID()[:12])
	})
}
