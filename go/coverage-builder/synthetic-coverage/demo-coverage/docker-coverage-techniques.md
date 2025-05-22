# Docker-Based Coverage Techniques for Go

This guide provides practical approaches for generating code coverage for Go applications running in Docker containers, with a focus on integrating with synthetic coverage techniques.

## Approach 1: Volume-Mounted Coverage Directory

This approach uses a shared volume to collect coverage data from containerized executions.

### Implementation:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Create coverage directory
	coverDir := "./coverage-docker"
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		fmt.Printf("Error creating coverage dir: %v\n", err)
		os.Exit(1)
	}
	
	// Get absolute path for proper mounting
	absPath, err := filepath.Abs(coverDir)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		os.Exit(1)
	}
	
	// Run Docker with coverage enabled and directory mounted
	cmd := exec.Command("docker", "run", 
		"-e", "GOCOVERDIR=/coverage",
		"-v", absPath+":/coverage",
		"your-image:tag",
		"/app/your-binary", "-test.coverprofile=/coverage/docker-coverage.out")
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running Docker: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Coverage data collected in", coverDir)
}
```

### Integration with synthetic coverage:

1. Run your tests in Docker with coverage enabled
2. Extract coverage data from the shared volume
3. Generate synthetic coverage for code paths not covered by Docker tests
4. Merge both coverage files using the text or binary format tools

## Approach 2: Multi-Stage Docker Build For Coverage

This technique uses a multi-stage Docker build to generate coverage data during the Docker build process itself.

### Dockerfile:

```dockerfile
# Build stage with coverage instrumentation
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go test -covermode=atomic -coverprofile=coverage.out ./...
RUN go tool cover -html=coverage.out -o coverage.html

# Extract coverage data
FROM alpine:latest AS coverage-extractor
WORKDIR /coverage
COPY --from=builder /app/coverage.out .
COPY --from=builder /app/coverage.html .

# Final application image
FROM golang:1.21-alpine
WORKDIR /app
COPY --from=builder /app/myapp .
CMD ["/app/myapp"]
```

### Integration with synthetic coverage:

```bash
# Extract coverage from Docker build
docker build --target coverage-extractor -t app-coverage .
docker create --name temp-container app-coverage
docker cp temp-container:/coverage/coverage.out ./docker-coverage.out
docker rm temp-container

# Merge with synthetic coverage
go run ../text-format/main.go -coverprofile=docker-coverage.out -synthetic=synthetic.txt -out=merged-coverage.out
```

## Approach 3: Docker Compose for Complex Testing

For more complex applications with multiple containers, Docker Compose provides a more sophisticated approach.

### docker-compose.yml:

```yaml
version: '3'

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile.test
    volumes:
      - ./coverage:/coverage
    environment:
      - GOCOVERDIR=/coverage
      - GO_TEST_COVERAGE=1
    command: >
      sh -c "go test -coverprofile=/coverage/app-coverage.out ./... && 
             chmod -R 777 /coverage"
  
  integration-tests:
    build:
      context: ./tests
    volumes:
      - ./coverage:/coverage
    depends_on:
      - app
    environment:
      - GOCOVERDIR=/coverage
    command: >
      sh -c "./run-integration-tests.sh && 
             chmod -R 777 /coverage"
```

### Integration with synthetic coverage:

```go
// Merge multiple coverage files from Docker Compose services
func mergeCoverageFromComposeServices() error {
    // Get all coverage files
    files, err := filepath.Glob("./coverage/*.out")
    if err != nil {
        return err
    }

    // Merge real coverage from containers
    mergedFile := "./coverage/merged.out"
    args := append([]string{"-o", mergedFile}, files...)
    cmd := exec.Command("gocovmerge", args...)
    if err := cmd.Run(); err != nil {
        return err
    }

    // Add synthetic coverage
    return addSyntheticCoverage(mergedFile, "./synthetic.txt", "./final-coverage.out")
}

func addSyntheticCoverage(real, synthetic, out string) error {
    cmd := exec.Command("go", "run", "../text-format/main.go", 
        "-coverprofile="+real, 
        "-synthetic="+synthetic, 
        "-out="+out)
    return cmd.Run()
}
```

## Best Practices

1. **Consistent Build Environment**: Ensure your Docker test environment matches your production environment
2. **Minimize Container Changes**: Keep test containers as close as possible to production containers
3. **Automated Coverage Collection**: Automate coverage extraction from containers
4. **Coverage Persistence**: Use volumes or Docker cp to ensure coverage data persists after container termination
5. **File Ownership**: Watch for file permission issues when sharing volumes between Docker and host
6. **Container Awareness**: Your code may need to detect if it's running in a container to enable coverage
7. **CI/CD Integration**: Integrate Docker-based coverage into your CI/CD pipeline