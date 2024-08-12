# sandbox-exec

sandbox-exec is a tool that uses Docker to run operations in a sandboxed environment. It provides a secure and isolated workspace for executing commands and scripts, making it ideal for testing and development purposes.

## Installation

1. Ensure you have Docker installed on your system.
2. Clone this repository:
   ```
   git clone https://github.com/tmc/misc/sandbox-exec.git
   cd sandbox-exec
   ```
3. Make sure the `sandbox-exec` script is executable:
   ```
   chmod +x sandbox-exec
   ```

## Usage

To run a command in a sandboxed environment:

```
./sandbox-exec [OPTIONS] [COMMAND]
```

Options:
- `--copy`: Creates a temporary copy of the entire workspace before running the sandbox.
- `--rm`: Remove the container after it exits.
- `--namespace NAME`: Use a custom namespace for git notes (default: sandbox-exec).

If no command is provided, an interactive bash session will be started in the sandbox.

### Examples

1. Start an interactive bash session:
   ```
   ./sandbox-exec
   ```

2. Run a specific command:
   ```
   ./sandbox-exec ls -l
   ```

3. Create a temporary copy of the workspace:
   ```
   ./sandbox-exec --copy
   ```

4. Use a custom namespace:
   ```
   ./sandbox-exec --namespace my-custom-namespace
   ```

## Configuration

The sandbox environment can be customized by modifying the `.sandbox-exec.dockerfile` file. This Dockerfile defines the base image and installed tools for the sandbox.

Key configuration points:
- Base image: `golang:1-bookworm`
- Installed tools: Docker, docker-buildx, mkprog, cgpt

## Additional Tools

### attach-sandbox-content

This tool attaches bash history and Docker logs from the latest sandbox to the current commit.

Usage:
```
./sandbox-tools/attach-sandbox-content [OPTIONS]
```

Options:
- `-n, --namespace NAME`: Use a custom namespace for git notes (default: sandbox-exec)
- `-c, --commit-hash`: Specify the commit hash to attach the sandbox context to (default: HEAD)
- `-r, --replace`: Replace existing sandbox context notes

### get-latest-sandbox

Retrieves the most recent sandbox-exec container from git history.

Usage:
```
./sandbox-tools/get-latest-sandbox [OPTIONS] [GIT_REF]
```
Options:
- `-c`: Show the commit hash for the latest sandbox-exec container
- `-n, --namespace NAME`: Use a custom namespace for git notes (default: sandbox-exec)

## Common Issues

1. **Docker not running**: Ensure that the Docker daemon is running on your system.

2. **Permission denied**: You may need to run the script with sudo if your user is not in the docker group:
   ```
   sudo ./sandbox-exec
   ```

3. **Git repository not found**: Make sure you're running the script from within a git repository.

## Development

- The project uses git notes to track sandbox executions. You can view these notes using `git notes list`.
- The sandbox depth is tracked using the `SANDBOX_DEPTH` environment variable, allowing for nested sandbox executions.
- Custom `.bashrc` files can be used by creating a `.bashrc_sandbox-exec` file in your home directory.

## Ideas for Future Development

- Implement history preserving capabilities to maintain command history across sandbox sessions.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT
