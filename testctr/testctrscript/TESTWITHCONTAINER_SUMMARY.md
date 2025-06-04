# TestWithContainer Implementation Summary

## Overview

Successfully implemented `TestWithContainer()` function that mirrors `scripttest.Test()` but runs script tests inside containerized environments. This enables testing scripts in clean, isolated, and reproducible container environments.

## Function Signature

```go
func TestWithContainer(t *testing.T, ctx context.Context, engine *script.Engine, env []string, pattern, containerImage string)
```

## Key Features ✅

### 1. **Mirrors scripttest.Test() Design**
- Same parameter structure and behavior patterns
- Identical timeout and grace period handling
- Parallel test execution support
- File pattern matching for test discovery

### 2. **Container Environment Management**
- Creates long-running container from specified image
- Sets up working directories and basic tools
- Automatic environment variable configuration
- Clean workspace isolation per test

### 3. **File System Integration**
- Extracts files from txtar archives into containers
- Preserves directory structure
- Safe text file transfer using shell here-docs
- Automatic cleanup of test workspaces

### 4. **Script Execution**
- Runs script content inside container environment
- Proper environment variable setup
- Exit code and output capture
- Error handling and reporting

## Usage Examples

### Basic Usage
```go
func TestMyContainerizedScripts(t *testing.T) {
    TestWithContainer(t, context.Background(), 
        &script.Engine{
            Cmds:  DefaultCmds(t),
            Conds: DefaultConds(),
        },
        nil, // environment variables
        "testdata/containerized/*.txt", 
        "alpine:latest")
}
```

### Custom Environment
```go
func TestWithCustomEnv(t *testing.T) {
    TestWithContainer(t, context.Background(),
        engine,
        []string{"DEBUG=1", "MODE=test"}, // custom env vars
        "testdata/advanced/*.txt",
        "ubuntu:20.04") // different base image
}
```

## Test File Format

Container tests use standard txtar format with script content in comments:

```
# Test script content goes here
echo "Hello from container!"
cat config.json

-- config.json --
{"test": true}

-- setup.sh --
#!/bin/sh
echo "Setup complete"
```

## Implementation Architecture

### 1. **Container Lifecycle**
```go
container := testctr.New(t, containerImage,
    testctr.WithCommand("sleep", "3600"), // Keep alive
)
```

### 2. **Workspace Management**
- Unique workspace per test: `/tmp/testwork/test_{name}_{timestamp}`
- Environment variables: `WORK=/tmp/testwork/...`
- Automatic cleanup after test completion

### 3. **File Transfer**
- Uses shell here-docs for safe text transfer
- Handles directory creation automatically
- Skips binary files with warnings
- Preserves file structure from txtar

### 4. **Script Execution**
```bash
#!/bin/sh
set -e
cd /tmp/testwork/test_name_timestamp
export WORK=/tmp/testwork/test_name_timestamp
export CUSTOM_VAR=value

# Original script content:
echo "Hello from container!"
```

## Benefits

### ✅ **Isolation**
- Clean container environment per test
- No host system dependencies
- Reproducible test conditions

### ✅ **Flexibility** 
- Any Docker image as test environment
- Custom environment variables
- File system setup via txtar

### ✅ **Integration**
- Seamless with existing script engine
- Standard testctr container management
- Automatic cleanup and error handling

### ✅ **Debugging**
- Full output capture and logging
- Clear error messages with context
- Test workspace preserved on failure

## Testing Results

### ✅ Successful Implementation
- All core tests pass
- Container tests run in isolated environments
- File extraction and script execution working
- Proper cleanup and error handling

### ✅ Test Separation
- Regular scripttest tests: `testdata/*.txt`
- Container tests: `testdata/containerized/*.txt`
- No interference between test types

### ✅ Example Tests
- **container_test.txt**: Basic environment verification
- **redis_in_container.txt**: Service installation testing

## Architecture Comparison

| Feature | scripttest.Test() | TestWithContainer() |
|---------|------------------|-------------------|
| Environment | Host system | Container |
| Isolation | Process-level | Container-level |
| Dependencies | Host tools | Container image |
| File System | Host FS | Container FS |
| Cleanup | Temp dirs | Container + workspaces |
| Reproducibility | Host-dependent | Image-dependent |

## Integration with testctr

The implementation leverages existing testctr infrastructure:
- Uses `testctr.New()` for container creation
- Inherits timeout and lifecycle management  
- Compatible with testctr options and backends
- Maintains zero-dependency philosophy

## Conclusion

`TestWithContainer()` successfully extends testctr's scripttest integration to support containerized test environments. It provides the same ease of use as `scripttest.Test()` while adding the benefits of container isolation and reproducibility.

The implementation is production-ready and provides a powerful tool for testing scripts in controlled, isolated environments using any Docker image as the test foundation.