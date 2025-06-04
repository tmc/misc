package postgres2

import (
	"fmt"
)

// ConnectionString builds a connection string for PostgreSQL.
func ConnectionString(host, port, username, password, database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, database)
}

// ConnectionStringWithSSL builds a connection string for PostgreSQL with SSL mode.
func ConnectionStringWithSSL(host, port, username, password, database, sslMode string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", username, password, host, port, database, sslMode)
}

// ConnectionStringWithOptions builds a connection string with additional options.
func ConnectionStringWithOptions(host, port, username, password, database string, options map[string]string) string {
	baseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", username, password, host, port, database)
	
	if len(options) == 0 {
		return baseURL + "?sslmode=disable"
	}
	
	query := ""
	for key, value := range options {
		if query != "" {
			query += "&"
		}
		query += fmt.Sprintf("%s=%s", key, value)
	}
	
	return baseURL + "?" + query
}

// GetDefaultRootPassword returns the default root password for PostgreSQL.
func GetDefaultRootPassword() string {
	return GetDefaultPassword()
}