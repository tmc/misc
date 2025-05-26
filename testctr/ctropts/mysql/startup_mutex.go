package mysql

import (
	"sync"
	"time"
)

// mysqlStartupMutex ensures only one MySQL container starts at a time
var mysqlStartupMutex sync.Mutex

// lockStartup acquires the startup lock for MySQL containers
func lockStartup() {
	mysqlStartupMutex.Lock()
}

// unlockStartup releases the startup lock after a delay
func unlockStartup() {
	// Wait a bit to ensure MySQL has started before allowing the next one
	time.Sleep(2 * time.Second)
	mysqlStartupMutex.Unlock()
}