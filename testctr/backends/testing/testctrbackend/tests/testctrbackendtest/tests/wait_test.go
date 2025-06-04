package tests

import (
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestDefaultWaitCondition_ContainerRuns(t *testing.T) {
	t.Parallel()

	// This container should start successfully with the default wait condition
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sleep", "infinity"),
	)

	// Verify we can execute commands
	output := c.ExecSimple("echo", "container is running")
	if output != "container is running" {
		t.Errorf("Expected 'container is running', got %q", output)
	}
}

func TestDefaultWaitCondition_VerboseLogging(t *testing.T) {
	t.Parallel()

	// Skip if not in verbose mode
	if !testing.Verbose() {
		t.Skip("Run with -v to see verbose wait condition logging")
	}

	// This should show the default wait condition logging
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sleep", "infinity"),
	)

	// The logs should show "Container X is running after Y"
	_ = c.ExecSimple("echo", "test")
}
