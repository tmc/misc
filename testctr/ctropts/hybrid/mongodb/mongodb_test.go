package mongodb

import (
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestMongoDBDefault(t *testing.T) {
	t.Parallel()

	// Test that the Default() option compiles and works
	container := testctr.New(t, "mongo:7", Default())
	
	// Verify the container is running
	if container == nil {
		t.Fatal("Expected container to be created")
	}

	// Test DSN generation
	provider := DSNProvider{}
	dsn := provider.FormatDSN(container, "testdb")
	if dsn == "" {
		t.Fatal("Expected DSN to be generated")
	}
	t.Logf("Generated DSN: %s", dsn)
}

func TestMongoDBWithOptions(t *testing.T) {
	t.Parallel()

	// Test custom configuration options
	container := testctr.New(t, "mongo:7",
		Default(),
		WithUsername("custom-user"),
		WithPassword("custom-pass"),
		WithDatabase("custom-db"),
	)

	if container == nil {
		t.Fatal("Expected container to be created with custom options")
	}
}

func TestConnectionString(t *testing.T) {
	// Test the enhanced connection string builder
	dsn := ConnectionString("localhost", "27017", "user", "pass", "mydb", map[string]string{
		"authSource": "admin",
		"ssl":        "true",
	})

	expected := "mongodb://user:pass@localhost:27017/mydb?authSource=admin&ssl=true"
	if dsn != expected && dsn != "mongodb://user:pass@localhost:27017/mydb?ssl=true&authSource=admin" {
		t.Errorf("Expected DSN to contain required parameters, got: %s", dsn)
	}
	t.Logf("Generated connection string: %s", dsn)
}