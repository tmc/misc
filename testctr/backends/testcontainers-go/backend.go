package testcontainers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Backend implements a testcontainers-go backend for testctr
// This shows how the same API can work with testcontainers-go

// Container wraps a testcontainers container
type Container struct {
	t           testing.TB
	image       string
	container   testcontainers.Container
	id          string
	env         map[string]string
	ports       []string
	mappedPorts []string
	cmd         []string
	labels      map[string]string
}

// New creates a new container using testcontainers-go
func New(t testing.TB, image string, opts ...Option) *Container {
	t.Helper()

	c := &Container{
		t:     t,
		image: image,
		env:   make(map[string]string),
		labels: map[string]string{
			"testctr":       "true",
			"testctr.test":  t.Name(),
			"testctr.image": image,
		},
		ports: []string{},
	}

	// Apply defaults based on image
	applyDefaults(c, image)

	// Apply options
	for _, opt := range opts {
		opt.apply(c)
	}

	ctx := context.Background()

	// Build wait strategy based on container type
	var waitStrategy wait.Strategy
	if strings.Contains(image, "mysql") {
		waitStrategy = wait.ForLog("ready for connections").WithStartupTimeout(60 * time.Second)
	} else if strings.Contains(image, "postgres") {
		waitStrategy = wait.ForLog("database system is ready to accept connections").WithStartupTimeout(60 * time.Second)
	} else if strings.Contains(image, "redis") {
		waitStrategy = wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second)
	} else {
		waitStrategy = wait.ForLog("").WithStartupTimeout(30 * time.Second)
	}

	req := testcontainers.ContainerRequest{
		Image:        image,
		Env:          c.env,
		ExposedPorts: c.ports,
		Cmd:          c.cmd,
		Labels:       c.labels,
		WaitingFor:   waitStrategy,
	}

	// Keep Alpine containers running if no command specified
	if strings.Contains(image, "alpine") && len(req.Cmd) == 0 {
		req.Cmd = []string{"sleep", "infinity"}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	c.container = container
	c.id = container.GetContainerID()

	// Map ports
	for _, port := range c.ports {
		// Parse port (e.g., "3306/tcp" -> "3306")
		portNum := strings.Split(port, "/")[0]
		natPort, err := nat.NewPort("tcp", portNum)
		if err != nil {
			t.Fatalf("Failed to parse port %s: %v", port, err)
		}
		mappedPort, err := container.MappedPort(ctx, natPort)
		if err != nil {
			t.Fatalf("Failed to get mapped port: %v", err)
		}
		c.mappedPorts = append(c.mappedPorts, mappedPort.Port())
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	return c
}

// applyDefaults applies sensible defaults based on container type
func applyDefaults(c *Container, image string) {
	if strings.Contains(image, "mysql") {
		c.env["MYSQL_ROOT_PASSWORD"] = "password"
		c.env["MYSQL_DATABASE"] = "test"
		c.ports = append(c.ports, "3306/tcp")
	} else if strings.Contains(image, "postgres") {
		c.env["POSTGRES_PASSWORD"] = "password"
		c.env["POSTGRES_DB"] = "test"
		c.ports = append(c.ports, "5432/tcp")
	} else if strings.Contains(image, "redis") {
		c.ports = append(c.ports, "6379/tcp")
	}
}

// Exec executes a command in the container
func (c *Container) Exec(cmd ...string) (string, error) {
	ctx := context.Background()

	exitCode, reader, err := c.container.Exec(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("exec failed: %w", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	if exitCode != 0 {
		return string(output), fmt.Errorf("command exited with code %d: %s", exitCode, output)
	}

	return string(output), nil
}

// Port returns the mapped host port for a container port
func (c *Container) Port(containerPort string) string {
	// Normalize port (e.g., "3306" or "3306/tcp")
	portNum := strings.Split(containerPort, "/")[0]

	for i, port := range c.ports {
		if strings.HasPrefix(port, portNum) {
			if i < len(c.mappedPorts) {
				return c.mappedPorts[i]
			}
		}
	}
	return ""
}

// Host returns the container host
func (c *Container) Host() string {
	return "localhost"
}

// Endpoint returns the host:port endpoint for a container port
func (c *Container) Endpoint(containerPort string) string {
	port := c.Port(containerPort)
	if port == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", c.Host(), port)
}

// ConnectionString returns a database connection string
func (c *Container) ConnectionString() string {
	if strings.Contains(c.image, "mysql") {
		return fmt.Sprintf("root:password@tcp(%s)/test", c.Endpoint("3306"))
	} else if strings.Contains(c.image, "postgres") {
		return fmt.Sprintf("postgresql://postgres:password@%s/test?sslmode=disable", c.Endpoint("5432"))
	} else if strings.Contains(c.image, "redis") {
		return fmt.Sprintf("redis://%s", c.Endpoint("6379"))
	}
	return ""
}
