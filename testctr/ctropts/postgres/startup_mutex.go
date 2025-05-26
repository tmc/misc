package postgres

import (
	"sync"
	"time"
)

// postgresStartupMutex ensures only one PostgreSQL container starts at a time
var postgresStartupMutex sync.Mutex

// lockStartup acquires the startup lock for PostgreSQL containers
func lockStartup() {
	postgresStartupMutex.Lock()
}

// unlockStartup releases the startup lock after a delay
func unlockStartup() {
	// Wait longer to ensure PostgreSQL has fully started before allowing the next one
	time.Sleep(5 * time.Second)
	postgresStartupMutex.Unlock()
}