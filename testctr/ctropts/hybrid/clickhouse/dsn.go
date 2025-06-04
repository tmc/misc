package clickhouse

import "fmt"

// ConnectionString builds a connection string for ClickHouse.
// This is a simplified implementation that can be enhanced with additional parameters.
func ConnectionString(host, port, username, password, database string) string {
	return fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s", username, password, host, port, database)
}

// Additional connection helpers can be implemented:
// - HTTPConnectionString() for HTTP interface
// - NativeConnectionString() for native protocol
