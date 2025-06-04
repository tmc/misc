package mongodb

import "fmt"

// ConnectionStringSimple builds a basic connection string for mongodb.
func ConnectionStringSimple(host, port, username, password, database string) string {
	return fmt.Sprintf("mongodb://%s:%s@%s:%s/%s", username, password, host, port, database)
}

// TODO: Add service-specific connection helpers:
// - ConnectionStringWithReplicaSet()
// - ConnectionStringWithAuth()
