package influxdb

import "fmt"

// ConnectionString builds a connection string for InfluxDB.
// This is a simplified implementation that can be enhanced with authentication and parameters.
func ConnectionString(host, port, username, password, database string) string {
	return fmt.Sprintf("http://%s:%s", host, port)
}

// Additional connection helpers can be implemented:
// - ConnectionStringWithToken() for token-based authentication
// - ConnectionStringV1() for InfluxDB 1.x compatibility
