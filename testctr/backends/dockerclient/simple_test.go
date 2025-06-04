package dockerclient

import (
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestDockerClientBackend_BasicRedis(t *testing.T) {
	t.Parallel()

	redis := testctr.New(t, "redis:7-alpine",
		testctr.WithBackend("dockerclient"),
		testctr.WithPort("6379"),
		ctropts.WithWaitForLog("Ready to accept connections", 30*time.Second),
	)

	// Test endpoint
	endpoint := redis.Endpoint("6379")
	if endpoint == "" {
		t.Fatal("expected endpoint, got empty string")
	}

	// Test exec
	output := redis.ExecSimple("redis-cli", "PING")
	if output != "PONG" {
		t.Errorf("expected PONG, got %q", output)
	}
}

// TODO: Add file tests when WithFile is exported
// func TestDockerClientBackend_WithFiles(t *testing.T) {
// 	t.Parallel()
//
// 	container := testctr.New(t, "alpine:latest",
// 		testctr.WithBackend("dockerclient"),
// 		testctr.WithFile("/dev/stdin", "/test.txt"),
// 		testctr.WithCommand("sleep", "infinity"),
// 	)
//
// 	// Check file was copied
// 	output := container.ExecSimple("cat", "/test.txt")
// 	if output == "" {
// 		t.Error("expected file content, got empty")
// 	}
// }

func TestDockerClientBackend_Inspect(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "alpine:latest",
		testctr.WithBackend("dockerclient"),
		testctr.WithCommand("sleep", "30"),
	)

	info, err := container.Inspect()
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	if !info.State.Running {
		t.Error("expected container to be running")
	}

	if info.ID == "" {
		t.Error("expected container ID")
	}
}
