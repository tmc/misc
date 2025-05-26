package testctr

import (
	"sync"
	"time"
)

// containerCoordinator manages global container creation to prevent resource contention
type containerCoordinator struct {
	mu               sync.Mutex
	activeContainers int
	maxConcurrent    int
	creationDelay    time.Duration
}

// globalContainerCoordinator manages all container creation
var globalContainerCoordinator = &containerCoordinator{
	maxConcurrent: 200,                   // Default, will be updated from flags
	creationDelay: 20 * time.Millisecond, // Default, will be updated from flags
}

// updateCoordinatorFromFlags updates the coordinator settings from command-line flags
func updateCoordinatorFromFlags() {
	globalContainerCoordinator.mu.Lock()
	defer globalContainerCoordinator.mu.Unlock()
	globalContainerCoordinator.maxConcurrent = *maxConcurrent
	globalContainerCoordinator.creationDelay = *createDelay
}

// requestContainerSlot blocks until a container creation slot is available
func (c *containerCoordinator) requestContainerSlot() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Wait if we have too many active containers
	for c.activeContainers >= c.maxConcurrent {
		c.mu.Unlock()
		time.Sleep(c.creationDelay)
		c.mu.Lock()
	}

	c.activeContainers++
}

// releaseContainerSlot releases a container creation slot
func (c *containerCoordinator) releaseContainerSlot() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeContainers > 0 {
		c.activeContainers--
	}
}
