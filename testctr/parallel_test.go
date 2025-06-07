package testctr_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/redis"
)

func TestParallelRedis(t *testing.T) {
	t.Parallel()
	// Create a single Redis container for all parallel tests using redis.Default()
	redisContainer := testctr.New(t, redis.DefaultRedisImage, redis.Default())

	t.Run("parallelRedisOperations", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			i := i // capture loop variable
			t.Run(fmt.Sprintf("operation_%d", i), func(t *testing.T) {
				t.Parallel() // Subtests run in parallel

				key := fmt.Sprintf("testkey:op%d", i)
				value := fmt.Sprintf("testvalue_%d", i)

				// SET using ExecSimple for conciseness
				setCmdOutput := redisContainer.ExecSimple("redis-cli", "SET", key, value)
				if setCmdOutput != "OK" { // redis-cli SET returns "OK" on success
					t.Errorf("Expected 'OK' from SET, got %q", setCmdOutput)
				}

				// GET
				getCmdOutput := redisContainer.ExecSimple("redis-cli", "GET", key)
				if getCmdOutput != value {
					t.Errorf("Expected value %q, got %q", value, getCmdOutput)
				}
			})
		}
	})
}

func TestConcurrentContainers_SimpleAlpine(t *testing.T) {
	t.Parallel()
	const numContainers = 5 // Reduced from 20 for faster, less resource-intensive local tests
	var wg sync.WaitGroup
	createdContainers := make([]*testctr.Container, numContainers)
	creationErrors := make([]error, numContainers) // To store errors from panics

	start := time.Now()
	for i := 0; i < numContainers; i++ {
		i := i // Capture loop variable
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					creationErrors[idx] = fmt.Errorf("panic during container creation: %v", r)
				}
			}()
			createdContainers[idx] = testctr.New(t, "alpine:latest",
				// Use sleep to keep container running
				testctr.WithCommand("sh", "-c", fmt.Sprintf("echo container %d ready && sleep infinity", idx)),
			)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	successCount := 0
	for i, c := range createdContainers {
		if creationErrors[i] != nil {
			t.Errorf("Container %d creation panicked: %v", i, creationErrors[i])
		} else if c == nil {
			t.Errorf("Container %d was not created (is nil) without a panic", i)
		} else {
			successCount++
			// Optionally, verify the container is responsive
			// output := c.ExecSimple("echo", "ping")
			// if output != "ping" {
			// 	t.Errorf("Container %d did not respond to ping correctly, got: %s", i, output)
			// }
		}
	}

	t.Logf("Attempted to create %d simple Alpine containers. Succeeded: %d. Time: %v", numContainers, successCount, elapsed)
	if successCount != numContainers {
		t.Errorf("Expected %d containers, but only %d were successfully created.", numContainers, successCount)
	}
}
