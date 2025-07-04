// Code generated by parse-tc-module. DO NOT EDIT.

package postgres_test

import (
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/exp/gen/modules/postgres"
)

func TestPostgresContainer(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "postgres:15-alpine", postgres.Default())
	
	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}

	port := container.Port("5432")
	if port == "" {
		t.Fatal("container port should not be empty")
	}

	endpoint := container.Endpoint("5432")
	if endpoint == "" {
		t.Fatalf("failed to get endpoint for port 5432")
	}
}

func TestPostgresWithOptions(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "postgres:15-alpine",
		postgres.Default(),
		// Add custom options here
	)

	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}
}
