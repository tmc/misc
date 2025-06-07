package testctr

import (
	"sync"
	"time"
)

// containerCoordinator manages global container creation to prevent resource contention
// when using the default CLI backend.
type containerCoordinator struct {
	mu               sync.Mutex
	activeContainers int
	maxConcurrent    int
	creationDelay    time.Duration
}

// globalContainerCoordinator manages all container creation for the default CLI backend.
var globalContainerCoordinator = &containerCoordinator{
	maxConcurrent: 20,                     // Default, will be updated from flags
	creationDelay: 200 * time.Millisecond, // Default, will be updated from flags
}

// updateCoordinatorFromFlags updates the coordinator settings from command-line flags.
// This is called once by testctr.New.
func updateCoordinatorFromFlags() {
	globalContainerCoordinator.mu.Lock()
	defer globalContainerCoordinator.mu.Unlock()
	globalContainerCoordinator.maxConcurrent = *maxConcurrent
	globalContainerCoordinator.creationDelay = *createDelay
}

// requestContainerSlot blocks until a container creation slot is available for the CLI backend.
// It ensures that no more than `maxConcurrent` containers are being started simultaneously.
func (c *containerCoordinator) requestContainerSlot() {
	c.mu.Lock() // Initial lock acquisition

	// Wait if we have too many active containers
	// This loop must release the lock before sleeping and re-acquire it after.
	for c.activeContainers >= c.maxConcurrent {
		c.mu.Unlock() // Release lock before sleeping
		time.Sleep(c.creationDelay)
		c.mu.Lock() // Re-acquire lock to check condition
	}

	c.activeContainers++
	c.mu.Unlock() // Release lock after incrementing
}

// releaseContainerSlot releases a container creation slot for the CLI backend.
func (c *containerCoordinator) releaseContainerSlot() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeContainers > 0 {
		c.activeContainers--
	}
}
