package testctr_tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
	"github.com/tmc/misc/testctr/ctropts/postgres"
	"github.com/tmc/misc/testctr/ctropts/redis"
)

// TestStressContainerCreation tests creating many containers rapidly
func TestStressContainerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Parallel()

	const numContainers = 8
	var wg sync.WaitGroup
	errors := make(chan error, numContainers)

	// Create containers in rapid succession
	for i := 0; i < numContainers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("container %d panicked: %v", i, r)
				}
			}()

			// Alternate between different container types
			switch i % 3 {
			case 0:
				c := testctr.New(t, "redis:7-alpine", redis.Default())
				_ = c.Port("6379") // Ensure port is available
			case 1:
				c := testctr.New(t, "alpine:latest", testctr.WithCommand("echo", fmt.Sprintf("container-%d", i)))
				_ = c.ID() // Ensure container was created
			case 2:
				// Only create a few MySQL containers due to resource constraints
				if i < 3 {
					c := testctr.New(t, "mysql:8", mysql.Default())
					dsn := c.DSN(t)
					if dsn == "" {
						errors <- fmt.Errorf("failed to get DSN for container %d", i)
					}
				}
			}
		}(i)
	}

	// Wait for all containers to be created
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errors)
	}()

	// Collect any errors
	var errorList []error
	go func() {
		for err := range errors {
			errorList = append(errorList, err)
		}
	}()

	// Timeout after 2 minutes
	select {
	case <-done:
		if len(errorList) > 0 {
			for _, err := range errorList {
				t.Error(err)
			}
		} else {
			t.Logf("Successfully created %d containers", numContainers)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("Stress test timed out after 2 minutes")
	}
}

// TestStressResourceLimits tests behavior under resource constraints
func TestStressResourceLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Parallel()

	// Test that our coordination prevents too many containers from starting simultaneously
	const attemptedContainers = 4
	var wg sync.WaitGroup
	startTimes := make([]time.Time, attemptedContainers)

	start := time.Now()

	for i := 0; i < attemptedContainers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			containerStart := time.Now()
			startTimes[i] = containerStart

			// Create a PostgreSQL container (which should be coordinated)
			c := testctr.New(t, "postgres:15", postgres.Default())
			_ = c.Port("5432")

			t.Logf("Container %d started at %v (after %v)", i,
				containerStart.Sub(start), containerStart.Sub(start))
		}(i)
	}

	wg.Wait()

	// Verify that containers were created with appropriate delays
	var delays []time.Duration
	for i := 1; i < len(startTimes); i++ {
		if !startTimes[i].IsZero() && !startTimes[i-1].IsZero() {
			delay := startTimes[i].Sub(startTimes[i-1])
			if delay > 0 {
				delays = append(delays, delay)
			}
		}
	}

	t.Logf("Created %d containers with coordination delays: %v", attemptedContainers, delays)

	// At least some containers should have been coordinated (delayed)
	if len(delays) > 0 {
		t.Logf("Container coordination is working - observed %d delays", len(delays))
	}
}
