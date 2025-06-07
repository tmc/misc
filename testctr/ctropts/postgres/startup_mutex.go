package postgres

import (
	"sync"
	"time"
)

// postgresStartupMutex ensures only one PostgreSQL container starts at a time.
// This helps prevent resource contention and race conditions when multiple
// PostgreSQL containers are initialized in parallel, especially during the
// data directory initialization phase.
var postgresStartupMutex sync.Mutex

// lockStartup acquires the startup lock for PostgreSQL containers.
// It blocks until the lock is available.
func lockStartup() {
	postgresStartupMutex.Lock()
}

// unlockStartup releases the startup lock after a delay.
// The delay provides a small buffer to ensure PostgreSQL has fully started
// and stabilized before allowing the next PostgreSQL container creation to proceed.
func unlockStartup() {
	// Wait longer to ensure PostgreSQL has fully started before allowing the next one
	time.Sleep(5 * time.Second)
	postgresStartupMutex.Unlock()
}
