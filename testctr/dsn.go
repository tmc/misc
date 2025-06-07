package testctr

import (
	"regexp"
	"strings"
	"testing"
)

// DSNProvider manages database connection strings and test isolation.
// Implemented by database modules like ctropts/postgres and ctropts/mysql.
type DSNProvider interface {
	// CreateDatabase creates a new database and returns its DSN.
	CreateDatabase(c *Container, t testing.TB, dbName string) (string, error)

	// DropDatabase removes the database. Called automatically during cleanup.
	DropDatabase(c *Container, dbName string) error

	// FormatDSN returns the connection string for the named database.
	FormatDSN(c *Container, dbName string) string
}

var (
	// invalidDBNameChars matches characters that are generally not allowed or problematic in database names.
	// This includes slashes, spaces, hyphens (sometimes problematic), and dots.
	invalidDBNameChars = regexp.MustCompile(`[/\s.-]+`)
	// leadingNumber matches if the string starts with a number.
	leadingNumber = regexp.MustCompile(`^[0-9]`)
)

// sanitizeDBName converts test names to valid database names.
// Replaces special chars with underscores, converts to lowercase,
// prepends "t_" if starting with digit, truncates to 60 chars.
func sanitizeDBName(testName string) string {
	// Replace special characters with underscores
	name := invalidDBNameChars.ReplaceAllString(testName, "_")

	// Convert to lowercase first for consistent prefixing and length checks
	name = strings.ToLower(name)

	// Ensure it starts with a letter (or underscore, which is usually fine)
	if leadingNumber.MatchString(name) {
		name = "t_" + name // Using "t_" to make it more distinct
	}

	// Truncate if too long (PostgreSQL limit is often 63, MySQL 64)
	// Using 60 as a safe common limit.
	const maxLength = 60
	if len(name) > maxLength {
		name = name[:maxLength]
		// Ensure it doesn't end with an underscore after truncation
		name = strings.TrimRight(name, "_")
	}

	return name
}
