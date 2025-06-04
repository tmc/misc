# testctr/scripttest

Package scripttest provides [rsc.io/script/scripttest](https://pkg.go.dev/rsc.io/script/scripttest) compatible commands and conditions for testing with testctr containers.

## Usage

```go
import (
    testctrscript "github.com/tmc/misc/testctr/scripttest"
    "rsc.io/script/scripttest"
)

func TestMyScripts(t *testing.T) {
    scripttest.Run(t, scripttest.Params{
        Dir:   "testdata",
        Cmds:  testctrscript.DefaultCmds(t),
        Conds: testctrscript.DefaultConds(),
    })
}
```

## Commands

### testctr start

Start a new container:

```
testctr start image name [options...]
```

Options:
- `-p port` - Expose a port (can be repeated)
- `-e key=value` - Set environment variable (can be repeated)  
- `--cmd cmd [args...]` - Override container command

Example:
```
testctr start redis:7-alpine myredis -p 6379 -e REDIS_PASSWORD=secret
```

### testctr stop

Stop and remove a container:

```
testctr stop name
```

### testctr exec

Execute a command in a container:

```
testctr exec name command [args...]
```

Example:
```
testctr exec myredis redis-cli PING
stdout PONG
```

### testctr port

Get the host port for a container port:

```
testctr port name port
```

### testctr endpoint

Get the full host:port endpoint:

```
testctr endpoint name port
```

### testctr wait

Wait for a container to be ready:

```
testctr wait name [timeout]
```

Default timeout is 30s.

## Conditions

### container

Check if a container exists and is running:

```
[container name]     # Container exists
[!container name]    # Container does not exist
```

## Example Script

```
# Start Redis
testctr start redis:7-alpine cache -p 6379

# Wait for it to be ready
testctr wait cache

# Check it exists
[container cache]

# Run commands
testctr exec cache redis-cli PING
stdout PONG

# Get connection info
testctr endpoint cache 6379
stdout 127.0.0.1:

# Cleanup (automatic, but can be explicit)
testctr stop cache
[!container cache]
```

## TestWithContainer

The `TestWithContainer` function allows running script tests inside a container environment, mirroring the functionality of `scripttest.Test()` but executing tests in a containerized environment.

### Usage

```go
func TestMyContainerizedScripts(t *testing.T) {
    TestWithContainer(t, context.Background(), 
        &script.Engine{
            Cmds:  DefaultCmds(t),
            Conds: DefaultConds(),
        },
        "testdata/*.txt", // test file pattern
        WithImage("alpine:latest"))
}

// With environment variables:
func TestWithEnv(t *testing.T) {
    TestWithContainer(t, context.Background(), 
        &script.Engine{
            Cmds:  DefaultCmds(t),
            Conds: DefaultConds(),
        },
        "testdata/*.txt",
        WithImage("alpine:latest"),
        WithEnv("DEBUG=1", "MODE=test"))
}

// Or with default Ubuntu image:
func TestWithDefaultImage(t *testing.T) {
    TestWithContainer(t, context.Background(), 
        &script.Engine{
            Cmds:  DefaultCmds(t),
            Conds: DefaultConds(),
        },
        "testdata/*.txt") // No options - defaults to ubuntu:latest
}
```

### Options

- **`WithImage(image string)`**: Specify the base container image (default: "ubuntu:latest")
- **`WithEnv(vars ...string)`**: Add environment variables to the container
- **`WithDockerInDocker()`**: Enable Docker-in-Docker support (requires volume mounting in future)

### How It Works

1. **Option Processing**: Applies configuration options (image, environment variables)
2. **Dockerfile Detection**: Checks if txtar archive contains a Dockerfile or dockerfile
3. **Image Selection**: Uses Dockerfile build > WithImage option > ubuntu:latest default
4. **Container Creation**: Creates a long-running container from the selected image
5. **Environment Setup**: Sets up working directories and installs basic tools
6. **File Extraction**: Copies files from txtar archives into the container
7. **Script Execution**: Runs script tests inside the container environment with configured environment
8. **Cleanup**: Automatically cleans up workspaces, containers, and custom images

### Benefits

- **Isolated Environment**: Tests run in clean, reproducible container environments
- **Custom Images**: Use any Docker image as the test environment
- **Custom Dockerfiles**: Automatically builds and uses custom images when Dockerfile is present
- **File Support**: Automatically extracts and manages files from txtar archives
- **Standard Interface**: Uses the same script engine and patterns as regular scripttest

## API Examples

The new `ContainerOption` API provides clean, extensible configuration:

```go
// Different API patterns:

// 1. Default ubuntu:latest (with warning)
TestWithContainer(t, ctx, engine, "testdata/*.txt")

// 2. Specific image
TestWithContainer(t, ctx, engine, "testdata/*.txt", 
    WithImage("alpine:latest"))

// 3. Image with environment
TestWithContainer(t, ctx, engine, "testdata/*.txt",
    WithImage("golang:1.21-alpine"),
    WithEnv("CGO_ENABLED=0", "GOOS=linux"))

// 4. Environment only (uses default ubuntu)
TestWithContainer(t, ctx, engine, "testdata/*.txt",
    WithEnv("NODE_ENV=test", "PORT=3000"))

// 5. Dockerfile override (ignores WithImage)
TestWithContainer(t, ctx, engine, "testdata/*.txt",
    WithImage("ubuntu:latest"), // Ignored if Dockerfile present
    WithEnv("TEST_MODE=dockerfile"))

// 6. Docker-in-Docker (future feature)
TestWithContainer(t, ctx, engine, "testdata/*.txt",
    WithImage("docker:latest"),
    WithDockerInDocker(),
    WithEnv("DOCKER_HOST=unix:///var/run/docker.sock"))
```

### Example Test Files

**Basic container test:**
```
# Test running in container environment
echo "Hello from container!"

# Files are automatically available
cat config.json

-- config.json --
{"test": true}

-- setup.sh --
#!/bin/sh
echo "Setup complete"
```

**Golang development test:**
```
echo "Testing Go environment..."
which go
go version
echo "Building and running Go program..."
go run main.go
echo "Testing go build..."
go build -o app main.go
./app

-- main.go --
package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println("Hello from Go!")
	fmt.Printf("Current Go version: %s\n", runtime.Version())
	fmt.Printf("Running on: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

-- go.mod --
module test

go 1.21
```

**Custom Dockerfile test:**
```
echo "Testing custom container environment..."
which curl
curl --version
node --version
echo "console.log('Hello from Node.js!');" | node
cat /hello.txt

-- Dockerfile --
FROM alpine:latest

# Install curl and nodejs
RUN apk add --no-cache curl nodejs npm

# Copy files from build context
COPY hello.txt /hello.txt

# Set working directory
WORKDIR /app

-- hello.txt --
Hello from the build context!
```

## Notes

- All containers are automatically cleaned up when the test ends
- Container names must be unique within a test
- The `testctr start` command waits for the container to be ready before returning
- Commands are executed with a 30-second timeout by default
- `TestWithContainer` requires txtar files with script content in the comment section
- **Container Image Selection**: TestWithContainer uses this priority order:
  1. Custom Dockerfile build (if `Dockerfile` or `dockerfile` present in txtar)
  2. `WithImage()` option value  
  3. Default `ubuntu:latest` (with warning if no options provided)
  - Custom images are automatically cleaned up after tests complete
  - Custom image names use format: `testctr-{lowercase-test-name-with-dashes}:{timestamp}`
- **File Transfer**: All txtar archive contents are cleanly transferred to containers
  - Text files use efficient here-doc transfer
  - Binary/large files use base64 encoding for reliable transfer
  - File permissions are preserved (scripts get execute permissions)
- **State Sharing**: Commands and conditions share container state through environment variables
  - Container IDs are stored as `TESTCTR_CONTAINER_<NAME>` environment variables
  - This eliminates global state and ensures thread-safe parallel test execution
  - All state is scoped to the script test environment for perfect isolation