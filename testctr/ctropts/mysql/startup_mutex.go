package mysql

import (
	"sync"
	"time"
)

// mysqlStartupMutex ensures only one MySQL container starts at a time.
// This helps prevent resource contention and race conditions when multiple
// MySQL containers are initialized in parallel, especially during the
// data directory initialization phase.
var mysqlStartupMutex sync.Mutex

// lockStartup acquires the startup lock for MySQL containers.
// It blocks until the lock is available.
func lockStartup() {
	mysqlStartupMutex.Lock()
}

// unlockStartup releases the startup lock after a short delay.
// The delay provides a small buffer to ensure MySQL has fully started
// and stabilized before allowing the next MySQL container creation to proceed.
func unlockStartup() {
	// Wait a bit to ensure MySQL has started before allowing the next one
	time.Sleep(2 * time.Second)
	mysqlStartupMutex.Unlock()
}
