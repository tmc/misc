package testctr

import (
	"strings"
	"testing"
)

// DSNProvider defines how to create and manage databases for a container type
type DSNProvider interface {
	// CreateDatabase creates a database and returns the DSN
	CreateDatabase(c *Container, t testing.TB, dbName string) (string, error)
	// DropDatabase drops a database
	DropDatabase(c *Container, dbName string) error
	// FormatDSN formats a DSN for the given database name
	FormatDSN(c *Container, dbName string) string
}

// sanitizeDBName converts a test name into a valid database name
func sanitizeDBName(testName string) string {
	// Replace special characters with underscores
	name := strings.ReplaceAll(testName, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")

	// Ensure it starts with a letter
	if len(name) > 0 && (name[0] >= '0' && name[0] <= '9') {
		name = "t" + name
	}

	// Truncate if too long (MySQL has 64 char limit)
	if len(name) > 60 {
		name = name[:60]
	}

	// Convert to lowercase for consistency
	return strings.ToLower(name)
}
