# scripttest Ideas

## Running Script Tests Inside Containers

The idea of serializing test state and running the remainder of a script test inside a container is intriguing. Here's how it could potentially work:

### Concept

When a script test calls `testctr start` with a special flag (e.g., `--enter`), the test would:

1. Serialize the current script state (environment variables, working directory, etc.)
2. Start a container with the test binary and script files mounted
3. Execute the remainder of the script inside the container
4. Return results back to the host

### Example

```
# Start of test runs on host
env MYVAR=hello

# This would switch execution to inside the container
testctr start golang:latest test-env --enter

# Everything below runs inside the container
exec go version
stdout 'go version'

# Access to environment from host
exec sh -c 'echo $MYVAR'
stdout hello
```

### Implementation Challenges

1. **State Serialization**: Need to capture and restore:
   - Environment variables
   - Working directory and files
   - Script position/continuation point
   - Test state and assertions

2. **Binary Compatibility**: The test binary needs to work inside the container
   - May need to compile for container architecture
   - Or use a container with matching architecture

3. **Script Continuation**: How to resume script execution at the right point
   - Could use a marker/checkpoint system
   - Or split scripts into host/container sections

4. **Result Propagation**: Getting test results back to the host
   - Exit codes
   - stdout/stderr
   - Test assertions and failures

### Alternative Approaches

1. **Explicit Sections**: Mark container vs host sections explicitly
   ```
   [host]
   # Setup on host
   
   [container golang:latest]
   # This section runs in container
   
   [host]
   # Back to host
   ```

2. **Container-Only Scripts**: Run entire script in a container from the start
   ```
   testctr-script run golang:latest mytest.txt
   ```

3. **Nested testctr**: Allow testctr commands inside container scripts
   - Would need Docker-in-Docker or socket mounting

This is definitely an interesting direction for making container-based testing even more flexible!