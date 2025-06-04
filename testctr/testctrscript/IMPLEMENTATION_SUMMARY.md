# testctr/scripttest Implementation Summary

## What We Built

Successfully implemented `DefaultCmds(t *testing.T)` and `DefaultConds()` functions that enable testctr integration with rsc.io/script/scripttest for declarative container testing.

## Key Features Implemented ✅

### Commands
- **`testctr start`** - Start containers with options (-p for ports, -e for env vars, --cmd for commands)
- **`testctr stop`** - Stop and remove containers
- **`testctr exec`** - Execute commands in containers with stdout/stderr capture
- **`testctr port`** - Get host port mappings
- **`testctr endpoint`** - Get full host:port endpoints  
- **`testctr wait`** - Wait for container readiness

### Integration
- **`DefaultCmds(t *testing.T)`** - Returns script commands including testctr
- **`DefaultConds()`** - Returns script conditions (container existence checking)
- **Environment-based state sharing** - Container state shared via environment variables
- **Thread-safe parallel execution** - No global state, perfect test isolation
- **Automatic cleanup** - Containers cleaned up when test ends

## Example Usage

### Basic Redis Test
```
# Start Redis container in background
testctr start redis:7-alpine myredis -p 6379

# Test Redis operations
testctr exec myredis redis-cli PING
stdout PONG

testctr exec myredis redis-cli SET key value
stdout OK

# Get connection info
testctr endpoint myredis 6379
stdout '127.0.0.1:'

# Cleanup
testctr stop myredis
```

### Multiple Containers
```
# Start multiple services
testctr start redis:7-alpine cache -p 6379
testctr start nginx:alpine web -p 80

# Test both services
testctr exec cache redis-cli SET background-test success
stdout OK

testctr exec web nginx -t
stdout 'syntax is ok'

# Data persists across commands
testctr exec cache redis-cli GET background-test
stdout success
```

## Why DefaultCmds() Accepts testing.T

The `DefaultCmds(t *testing.T)` function requires a `*testing.T` parameter for several critical reasons:

### 1. **Container Lifecycle Management**
- Each test gets its own isolated container registry
- Containers are automatically cleaned up when the test ends via `t.Cleanup()`
- Prevents container leaks between test runs

### 2. **Test Context Integration**
```go
func DefaultCmds(t *testing.T) map[string]script.Cmd {
    cmds := script.DefaultCmds()
    cmds["testctr"] = SimpleCmd(t)  // t is passed to container manager
    return cmds
}
```

### 3. **Proper Error Handling**
- Container creation failures are properly reported through test framework
- `testctr.New(t, ...)` uses test context for logging and error reporting
- Failed containers can be kept for debugging with testctr flags

### 4. **Resource Management**
- Test-scoped resource allocation and cleanup
- Prevents resource conflicts between parallel tests
- Enables proper test isolation

### 5. **Registry Sharing**
- Commands within the same test share a container registry
- Different tests get separate registries
- Registry cleanup is tied to test lifecycle

## Architecture Benefits

### Environment Variable State Sharing
```go
// Container IDs stored as environment variables
func containerEnvKey(name string) string {
    return "TESTCTR_CONTAINER_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// Store container in environment when created
s.Setenv(containerEnvKey(name), container.ID())

// Retrieve container existence from environment
if val, _ := s.LookupEnv(containerEnvKey(name)); val != "" {
    // Container exists
}
```

### Test Isolation
- Container state stored in script.State environment variables
- No global state - eliminates race conditions completely  
- Thread-safe parallel test execution with no synchronization needed
- Perfect test isolation - each script has its own environment
- Automatic cleanup when script execution completes

### Integration with testctr
- Uses existing testctr.New() API for consistency
- Inherits all testctr features (wait strategies, DSN providers, etc.)
- Maintains zero-dependency philosophy
- Compatible with existing testctr options

## Recent Improvements (Race Condition Fix)

### Problem Solved
- **Race conditions** in concurrent map access when running `go test -race`
- **Global state complexity** with mutex synchronization requirements
- **Thread safety concerns** for parallel test execution

### Solution: Environment Variable Approach
- **Eliminated global state** by using script.State environment variables
- **Simplified API** - consolidated to single environment-based implementation  
- **Thread-safe by design** - no synchronization needed
- **Perfect test isolation** - each script has independent state

### Migration Summary
1. **Identified race conditions** in `containerRegistry.containers` map access
2. **First fix**: Added `sync.RWMutex` protection (worked but complex)
3. **Final solution**: Refactored to use environment variables exclusively
4. **API consolidation**: Removed duplicate "Env" versions, unified to single clean API
5. **Validation**: All tests pass with `go test -race` with no detected races

## Testing Results

All test scripts pass successfully with race detection:
- ✅ **background.txt** - Multiple container coordination
- ✅ **basic.txt** - Basic container lifecycle  
- ✅ **environment.txt** - Environment variables and custom commands
- ✅ **multiple.txt** - Multiple container management
- ✅ **redis.txt** - Redis-specific operations
- ✅ **simple.txt** - Simple Redis test
- ✅ **Race detection** - All tests pass with `go test -race -count=2`

## Usage in Tests

```go
import (
    testctrscript "github.com/tmc/misc/testctr/scripttest"
    "rsc.io/script/scripttest"
)

func TestMyContainerScripts(t *testing.T) {
    scripttest.Run(t, scripttest.Params{
        Dir:   "testdata", 
        Cmds:  testctrscript.DefaultCmds(t),  // t required here
        Conds: testctrscript.DefaultConds(),
    })
}
```

The `*testing.T` parameter is essential for proper test integration, resource management, and container lifecycle handling. It's not just a convenience - it's a requirement for correct operation of the scripttest integration.