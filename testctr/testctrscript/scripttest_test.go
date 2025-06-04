package testctrscript_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	testctrscript "github.com/tmc/misc/testctr/testctrscript"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestScriptTestIntegration(t *testing.T) {
	t.Parallel()
	// Create engine with default commands and conditions
	engine := &script.Engine{
		Cmds:  testctrscript.DefaultCmds(t),
		Conds: testctrscript.DefaultConds(),
		Quiet: !testing.Verbose(),
	}

	// Run tests in testdata directory
	testdata := "testdata"
	files, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".txt" {
			continue
		}

		name := file.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Read script file
			scriptPath := filepath.Join(testdata, name)
			scriptFile, err := os.Open(scriptPath)
			if err != nil {
				t.Fatal(err)
			}
			defer scriptFile.Close()

			// Create initial state
			state, err := script.NewState(ctx, testdata, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Run the script
			scripttest.Run(t, engine, state, name, scriptFile)
		})
	}
}

func TestDockerInDockerContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping Docker-in-Docker test in short mode")
	}

	// Test Docker-in-Docker functionality with custom Dockerfile
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/docker_in_docker_test.txt",
		testctrscript.WithDockerInDocker(), // Enable Docker-in-Docker
		testctrscript.WithEnv("DOCKER_HOST=unix:///var/run/docker.sock"))
}

func TestImageNamingContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping image naming test in short mode")
	}

	// Test with a test name that has slashes to verify image naming
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/dockerfile_test.txt", // Uses Dockerfile
		testctrscript.WithImage("alpine:latest"))     // Should be ignored due to Dockerfile
}

func TestContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test using TestWithContainer function with Alpine
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/container_test.txt",
		testctrscript.WithImage("alpine:latest"))
}

func TestGolangContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test using TestWithContainer function with Golang image and development environment
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/golang_test.txt",
		testctrscript.WithImage("golang:1.21-alpine"),
		testctrscript.WithEnv("CGO_ENABLED=0", "GOOS=linux", "GO_ENV=development"))
}

func TestDockerfileContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test using TestWithContainer function with custom Dockerfile
	// The base image parameter will be ignored since a Dockerfile is present
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/dockerfile_test.txt",
		testctrscript.WithImage("alpine:latest")) // This will be ignored due to Dockerfile presence
}

func TestDefaultImageContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test using TestWithContainer function with default image (no image specified)
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/ubuntu_test.txt") // No image parameter - should default to ubuntu:latest
}

func TestWithEnvContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test using TestWithContainer function with environment variables
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/env_test.txt",
		testctrscript.WithImage("alpine:latest"),
		testctrscript.WithEnv("TEST_VAR=hello", "DEBUG=1"))
}

func TestMultipleOptionsContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test multiple options with a more complex scenario
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/multi_options_test.txt",
		testctrscript.WithImage("node:18-alpine"),
		testctrscript.WithEnv("NODE_ENV=test", "PORT=3000", "DEBUG=testctr:*"))
}

func TestDockerfilePriorityContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	// Test that Dockerfile takes priority over WithImage
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		"testdata/containerized/dockerfile_priority_test.txt",
		testctrscript.WithImage("ubuntu:latest"), // This should be ignored due to Dockerfile
		testctrscript.WithEnv("TEST_MODE=dockerfile_priority"))
}

func TestAPIVariationsContainerizedScripts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping containerized script tests in short mode")
	}

	t.Run("DefaultImage", func(t *testing.T) {
		t.Parallel()
		// Test 1: No options - should default to ubuntu:latest with warning
		testctrscript.TestWithContainer(t, context.Background(),
			&script.Engine{
				Cmds:  testctrscript.DefaultCmds(t),
				Conds: testctrscript.DefaultConds(),
			},
			"testdata/containerized/api_variations_test.txt")
	})

	t.Run("WithImageOnly", func(t *testing.T) {
		t.Parallel()
		// Test 2: WithImage only
		testctrscript.TestWithContainer(t, context.Background(),
			&script.Engine{
				Cmds:  testctrscript.DefaultCmds(t),
				Conds: testctrscript.DefaultConds(),
			},
			"testdata/containerized/api_variations_test.txt",
			testctrscript.WithImage("alpine:latest"))
	})

	t.Run("WithEnvOnly", func(t *testing.T) {
		t.Parallel()
		// Test 3: WithEnv only (should use default ubuntu:latest)
		testctrscript.TestWithContainer(t, context.Background(),
			&script.Engine{
				Cmds:  testctrscript.DefaultCmds(t),
				Conds: testctrscript.DefaultConds(),
			},
			"testdata/containerized/api_variations_test.txt",
			testctrscript.WithEnv("TEST_CASE=WithEnvOnly"))
	})

	t.Run("BothImageAndEnv", func(t *testing.T) {
		t.Parallel()
		// Test 4: Both WithImage and WithEnv
		testctrscript.TestWithContainer(t, context.Background(),
			&script.Engine{
				Cmds:  testctrscript.DefaultCmds(t),
				Conds: testctrscript.DefaultConds(),
			},
			"testdata/containerized/api_variations_test.txt",
			testctrscript.WithImage("alpine:latest"),
			testctrscript.WithEnv("TEST_CASE=BothImageAndEnv", "EXTRA_VAR=value"))
	})
}
