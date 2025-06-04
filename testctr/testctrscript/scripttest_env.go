// Package testctrscript provides rsc.io/script/scripttest compatible commands
// for testing with testctr containers.
package testctrscript

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"rsc.io/script"
)

// containerManager manages containers using environment variables for state sharing
type containerManager struct {
	t          *testing.T
	containers map[string]*testctr.Container // local cache
	mu         sync.RWMutex
}

// containerEnvKey returns the environment variable name for a container
func containerEnvKey(name string) string {
	return "TESTCTR_CONTAINER_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// SimpleCmdEnv creates a testctr command that uses environment variables for state.
// This eliminates global state and allows commands and conditions to share container information.
func SimpleCmdEnv(t *testing.T) script.Cmd {
	mgr := &containerManager{
		t:          t,
		containers: make(map[string]*testctr.Container),
	}
	
	return script.Command(
		script.CmdUsage{
			Summary: "manage test containers",
			Args:    "start|stop|exec|port|endpoint|wait image-or-name [name] [args...]",
			Detail: []string{
				"The testctr command manages containers for testing.",
				"",
				"Subcommands:",
				"  start image name [opts...]  - Start a container",
				"  stop name                   - Stop a container",
				"  exec name cmd [args...]     - Execute a command",
				"  port name port              - Get the host port",
				"  endpoint name port          - Get the endpoint",
				"  wait name                   - Wait for container",
			},
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, script.ErrUsage
			}
			
			switch args[0] {
			case "start":
				return mgr.startEnv(s, args[1:])
			case "stop":
				return nil, mgr.stopEnv(s, args[1:])
			case "exec":
				return mgr.execEnv(s, args[1:])
			case "port":
				return mgr.portEnv(s, args[1:])
			case "endpoint":
				return mgr.endpointEnv(s, args[1:])
			case "wait":
				return mgr.waitEnv(s, args[1:])
			default:
				return nil, fmt.Errorf("unknown subcommand: %s", args[0])
			}
		},
	)
}

func (m *containerManager) startEnv(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("start requires image and name")
	}
	
	image := args[0]
	name := args[1]
	
	// Check if already exists via environment
	if val, _ := s.LookupEnv(containerEnvKey(name)); val != "" {
		return nil, fmt.Errorf("container %q already exists", name)
	}
	
	// Parse options
	var opts []testctr.Option
	var async bool
	
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 < len(args) {
				i++
				opts = append(opts, testctr.WithPort(args[i]))
			}
		case "-e":
			if i+1 < len(args) {
				i++
				parts := strings.SplitN(args[i], "=", 2)
				if len(parts) == 2 {
					opts = append(opts, testctr.WithEnv(parts[0], parts[1]))
				}
			}
		case "--async":
			async = true
		case "--cmd":
			if i+1 < len(args) {
				opts = append(opts, testctr.WithCommand(args[i+1:]...))
			}
			break
		}
	}
	
	if async {
		return func(s *script.State) (stdout, stderr string, err error) {
			container := testctr.New(m.t, image, opts...)
			
			// Store in cache and environment
			m.mu.Lock()
			m.containers[name] = container
			m.mu.Unlock()
			
			s.Setenv(containerEnvKey(name), container.ID())
			return fmt.Sprintf("started container %s asynchronously\n", name), "", nil
		}, nil
	}
	
	// Synchronous start
	container := testctr.New(m.t, image, opts...)
	
	// Store in cache and environment
	m.mu.Lock()
	m.containers[name] = container
	m.mu.Unlock()
	
	s.Setenv(containerEnvKey(name), container.ID())
	s.Logf("started container %s\n", name)
	return nil, nil
}

func (m *containerManager) stopEnv(s *script.State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("stop requires name")
	}
	
	name := args[0]
	
	// Check environment first
	if val, _ := s.LookupEnv(containerEnvKey(name)); val == "" {
		return fmt.Errorf("container %s not found", name)
	}
	
	// Remove from environment
	s.Setenv(containerEnvKey(name), "")
	
	// Remove from cache
	m.mu.Lock()
	delete(m.containers, name)
	m.mu.Unlock()
	
	s.Logf("stopped container %s\n", name)
	return nil
}

func (m *containerManager) getContainer(s *script.State, name string) (*testctr.Container, error) {
	// Check if exists in environment
	if val, _ := s.LookupEnv(containerEnvKey(name)); val == "" {
		return nil, fmt.Errorf("container %s not found", name)
	}
	
	// Check cache first
	m.mu.RLock()
	container, ok := m.containers[name]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("container %s not in cache (may have been created by another command)", name)
	}
	
	return container, nil
}

func (m *containerManager) execEnv(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("exec requires name and command")
	}
	
	name := args[0]
	container, err := m.getContainer(s, name)
	if err != nil {
		return nil, err
	}
	
	return func(s *script.State) (stdout, stderr string, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		exitCode, output, err := container.Exec(ctx, args[1:])
		if err != nil {
			return "", "", err
		}
		if exitCode != 0 {
			return "", output, fmt.Errorf("exit code %d", exitCode)
		}
		return output, "", nil
	}, nil
}

func (m *containerManager) portEnv(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("port requires name and port")
	}
	
	name := args[0]
	port := args[1]
	
	container, err := m.getContainer(s, name)
	if err != nil {
		return nil, err
	}
	
	return func(s *script.State) (stdout, stderr string, err error) {
		hostPort := container.Port(port)
		if hostPort == "" {
			return "", "", fmt.Errorf("port %s not mapped", port)
		}
		return hostPort + "\n", "", nil
	}, nil
}

func (m *containerManager) endpointEnv(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("endpoint requires name and port")
	}
	
	name := args[0]
	port := args[1]
	
	container, err := m.getContainer(s, name)
	if err != nil {
		return nil, err
	}
	
	return func(s *script.State) (stdout, stderr string, err error) {
		endpoint := container.Endpoint(port)
		return endpoint + "\n", "", nil
	}, nil
}

func (m *containerManager) waitEnv(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("wait requires name [timeout]")
	}
	
	name := args[0]
	timeout := 30 * time.Second // default timeout
	
	if len(args) > 1 {
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration: %v", err)
		}
		timeout = d
	}
	
	// Get container from cache for actual wait operations
	_, err := m.getContainer(s, name)
	if err != nil {
		return nil, err
	}
	
	return func(s *script.State) (string, string, error) {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		
		// Check if container is still running
		// In testctr, containers are ready after New(), but we could add health checks here
		select {
		case <-ctx.Done():
			return "", "", fmt.Errorf("timeout waiting for container %s after %v", name, timeout)
		default:
			// Container exists and is ready (testctr waits in New())
			s.Logf("container %s ready\n", name)
			return "", "", nil
		}
	}, nil
}

// ContainerCondEnv returns a condition that checks container existence via environment variables
func ContainerCondEnv() script.Cond {
	return script.PrefixCondition("test container existence", func(s *script.State, suffix string) (bool, error) {
		if suffix == "" {
			return false, fmt.Errorf("container condition requires a container name")
		}
		name := strings.TrimSpace(suffix)
		// Check if container exists by looking for its environment variable
		val, _ := s.LookupEnv(containerEnvKey(name))
		return val != "", nil
	})
}

// DefaultCmdsEnv returns commands using environment variable state sharing
func DefaultCmdsEnv(t *testing.T) map[string]script.Cmd {
	cmds := script.DefaultCmds()
	cmds["testctr"] = SimpleCmdEnv(t)
	// cmds["parse-tc-module"] = createParseTCModuleCommand() // TODO: implement if needed
	return cmds
}

// ContainerReadyCondEnv returns a condition that waits for a container to be ready.
// Usage: [container-ready name timeout] e.g., [container-ready myredis 30s]
func ContainerReadyCondEnv() script.Cond {
	return script.PrefixCondition("wait for container to be ready", func(s *script.State, suffix string) (bool, error) {
		parts := strings.Fields(suffix)
		if len(parts) < 1 {
			return false, fmt.Errorf("container-ready requires a container name")
		}
		
		name := parts[0]
		// timeout would be used for actual health checks
		// timeout := 10 * time.Second // default timeout
		
		if len(parts) > 1 {
			_, err := time.ParseDuration(parts[1])
			if err != nil {
				return false, fmt.Errorf("invalid timeout duration: %v", err)
			}
			// timeout = d
		}
		
		// Check if container exists
		val, _ := s.LookupEnv(containerEnvKey(name))
		if val == "" {
			return false, nil // Container doesn't exist yet
		}
		
		// In a real implementation, we'd check container health/readiness
		// For now, we'll just verify it exists (containers are ready after New())
		return true, nil
	})
}

// DefaultCondsEnv returns conditions using environment variable state sharing
func DefaultCondsEnv() map[string]script.Cond {
	conds := script.DefaultConds()
	conds["container"] = ContainerCondEnv()
	conds["container-ready"] = ContainerReadyCondEnv()
	return conds
}
