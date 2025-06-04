package testctrscript // Package testctrscript provides rsc.io/script integration for running test scripts inside containers.
//
// This package enables script-based testing with containerized environments, supporting
// both simple script execution and Dockerfile-based custom container builds. It extends
// the [rsc.io/script] testing framework with [github.com/tmc/misc/testctr] container capabilities.
//
// # Basic Script Testing
//
// The core function [TestWithContainer] runs script tests inside containers:
//
//	func TestMyScript(t *testing.T) {
//	    testctrscript.TestWithContainer(t, context.Background(),
//	        &script.Engine{
//	            Cmds:  testctrscript.DefaultCmds(t),
//	            Conds: testctrscript.DefaultConds(),
//	        },
//	        "testdata/scripts/*.txt")
//	}
//
// # Container Options
//
// Containers can be customized using [ContainerOption] functions:
//
//	// Use specific image and environment
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithImage("golang:1.21-alpine"),
//	    testctrscript.WithEnv("CGO_ENABLED=0", "GOOS=linux"))
//
//	// Enable Docker-in-Docker support
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithDockerInDocker())
//
//	// Custom Docker build arguments
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithBuildArgs("--platform=linux/amd64"),
//	    testctrscript.WithBuildx())
//
// # Dockerfile Support
//
// Scripts can include Dockerfiles for custom container environments.
// When a Dockerfile is present in the script archive, it will be built
// automatically and used instead of the base image:
//
//	# Script: testdata/custom-env.txt
//	echo "Testing custom environment..."
//	go version
//	node --version
//
//	-- Dockerfile --
//	FROM golang:1.21-alpine
//	RUN apk add --no-cache nodejs npm
//	WORKDIR /app
//
// Built images are automatically labeled with metadata for cleanup and identification:
//   - `testctr=true` - Base identification label
//   - `testctr.testname` - Full test name
//   - `testctr.script` - Script name
//   - `testctr.timestamp` - Creation timestamp
//   - `testctr.type=script-built` - Type identifier
//   - `testctr.created-by=testctrscript` - Package identifier
//
// # Script Commands
//
// The package provides container-aware script commands through [DefaultCmds]:
//
//	testctr start <image> <name> [options]  # Start a named container
//	testctr stop <name>                     # Stop a container
//	testctr exec <name> <command...>        # Execute command in container
//	testctr wait <name>                     # Wait for container to be ready
//	testctr endpoint <name> <port>          # Get container endpoint
//	testctr port <name> <port>              # Get host port mapping
//
// Example script using these commands:
//
//	# Start services
//	testctr start redis:7-alpine cache -p 6379
//	testctr start postgres:15 db -p 5432 -e POSTGRES_PASSWORD=test
//
//	# Wait for services to be ready
//	testctr wait cache
//	testctr wait db
//
//	# Test connectivity
//	testctr exec cache redis-cli ping
//	stdout PONG
//
//	testctr exec db psql -U postgres -c 'SELECT 1'
//
//	# Get endpoints for application use
//	testctr endpoint cache 6379
//	testctr endpoint db 5432
//
// # Script Conditions
//
// The package provides container-aware conditions through [DefaultConds]:
//
//	[container name]     # Container exists and is running
//	[!container name]    # Container does not exist or is not running
//
// Example script using conditions:
//
//	testctr start nginx:alpine web -p 80
//	testctr wait web
//
//	[container web]      # Verify container is running
//
//	testctr stop web
//	[!container web]     # Verify container is stopped
//
// # Image Management and Cleanup
//
// testctrscript provides comprehensive image management with configurable cleanup:
//
//	-testctr.cleanup-images=true/false    # Clean up script-built images (default: true)
//	-testctr.warn-images=true/false       # Warn about old images (default: true)
//	-testctr.cleanup-orphans=true/false   # Clean up orphaned images (default: true)
//	-testctr.cleanup-age=5m               # Age threshold for cleanup
//
// Images are cleaned up automatically after tests complete, respecting the
// `testctr.keep-failed` flag for debugging failed tests.
//
// # Advanced Build Features
//
// Support for advanced Docker build features:
//
//	// Multi-platform builds
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithPlatform("linux/arm64"))
//
//	// Docker Buildx support
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithBuildx(),
//	    testctrscript.WithBuildArgs("--cache-from=type=gha"))
//
//	// Custom build arguments
//	testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt",
//	    testctrscript.WithBuildArgs("--build-arg", "VERSION=1.0"))
//
// # Error Handling and Debugging
//
// The package includes comprehensive panic recovery and error handling:
//
//   - Panic recovery in all critical functions (container operations, Docker builds, script execution)
//   - Detailed error logging with container context
//   - Failed test container preservation with `-testctr.keep-failed`
//   - Container inspection commands for debugging
//
// # Thread Safety and Parallel Testing
//
// All operations are thread-safe and support parallel test execution:
//
//	func TestParallelScripts(t *testing.T) {
//	    t.Parallel()
//	    testctrscript.TestWithContainer(t, ctx, engine, "testdata/*.txt")
//	    // Each test gets isolated containers and images
//	}
//
// Container state is shared between script commands and conditions using environment
// variables stored in the script.State, eliminating global state and ensuring perfect
// test isolation. Container IDs are stored as TESTCTR_CONTAINER_<NAME> environment
// variables, providing thread-safe access across all script operations.
//
// # File and Archive Handling
//
// Scripts support [golang.org/x/tools/txtar] archive format for including
// multiple files:
//
//	# Test with application files
//	echo "Building and testing application..."
//
//	# Build application
//	go build -o app main.go
//	./app --version
//
//	-- main.go --
//	package main
//	import "fmt"
//	func main() { fmt.Println("Hello, World!") }
//
//	-- go.mod --
//	module testapp
//	go 1.21
//
// Files are automatically extracted to the container workspace with proper
// permissions and directory structure.
//
// # Integration with testctr
//
// testctrscript uses [github.com/tmc/misc/testctr] for container management,
// inheriting all its features including backend support, database DSN providers,
// and configuration flags. See the testctr package documentation for details
// on container options and backend configuration.
