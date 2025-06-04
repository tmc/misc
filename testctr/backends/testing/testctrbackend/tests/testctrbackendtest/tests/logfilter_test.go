package tests

import (
	"strings"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestLogFilter(t *testing.T) {
	t.Parallel()

	// Create a container with a log filter that excludes lines containing "monotonic"
	c := testctr.New(t, "redis:7-alpine",
		ctropts.WithLogs(),
		ctropts.WithLogFilter(func(line string) bool {
			// Filter out lines containing "monotonic" or "WARNING"
			return !strings.Contains(line, "monotonic") && !strings.Contains(line, "WARNING")
		}),
	)

	// The container should start successfully
	if c.Host() == "" {
		t.Fatal("Expected container to have a host")
	}
}

func TestLogFilterVerbose(t *testing.T) {
	t.Parallel()

	// This test shows that without a filter, all logs are visible
	c := testctr.New(t, "redis:7-alpine",
		ctropts.WithLogs(), // Enable log streaming without filter
	)

	// The container should start successfully
	if c.Host() == "" {
		t.Fatal("Expected container to have a host")
	}
}
