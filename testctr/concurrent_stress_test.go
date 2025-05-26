//go:build stress

package testctr_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
)

func TestConcurrentContainersStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	tests := []struct {
		name       string
		count      int
		goroutines int
	}{
		{"Small", 10, 10},
		{"Medium", 25, 25},
		{"Large", 50, 50},
		{"VeryLarge", 100, 50}, // 100 containers but only 50 goroutines
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			containers := make([]*testctr.Container, tt.count)
			errors := make([]error, tt.count)

			start := time.Now()

			// Create a semaphore to limit goroutines
			sem := make(chan struct{}, tt.goroutines)

			for i := 0; i < tt.count; i++ {
				i := i
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Acquire semaphore
					sem <- struct{}{}
					defer func() { <-sem }()

					defer func() {
						if r := recover(); r != nil {
							errors[i] = fmt.Errorf("panic: %v", r)
						}
					}()

					containers[i] = testctr.New(t, "alpine:latest",
						testctr.WithCommand("sh", "-c", fmt.Sprintf("echo 'Container %d started' && sleep 0.1", i)),
					)
				}()
			}

			wg.Wait()
			elapsed := time.Since(start)

			// Count successes and failures
			successCount := 0
			var failedIndices []int
			for i, c := range containers {
				if c != nil {
					successCount++
				} else if errors[i] == nil {
					failedIndices = append(failedIndices, i)
				}
			}

			// Report errors
			for i, err := range errors {
				if err != nil {
					t.Errorf("Container %d failed with error: %v", i, err)
				}
			}

			if len(failedIndices) > 0 {
				t.Errorf("Containers failed without error: %v", failedIndices)
			}

			t.Logf("Created %d/%d containers in %v (%.2f containers/sec)",
				successCount, tt.count, elapsed, float64(successCount)/elapsed.Seconds())
		})
	}
}

func TestConcurrentMixedContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mixed container test in short mode")
	}

	// Test creating different types of containers concurrently
	images := []string{
		"alpine:latest",
		"busybox:latest",
		"redis:7-alpine",
		"nginx:alpine",
	}

	const containersPerImage = 5
	var wg sync.WaitGroup
	containers := make([]*testctr.Container, len(images)*containersPerImage)

	start := time.Now()
	for i, image := range images {
		for j := 0; j < containersPerImage; j++ {
			idx := i*containersPerImage + j
			img := image
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				containers[index] = testctr.New(t, img,
					testctr.WithCommand("echo", fmt.Sprintf("Container %s-%d", img, index)),
				)
			}(idx)
		}
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify all containers were created
	successCount := 0
	for i, c := range containers {
		if c == nil {
			t.Errorf("Container %d was not created", i)
		} else {
			successCount++
		}
	}

	t.Logf("Created %d/%d mixed containers in %v", successCount, len(containers), elapsed)
}
