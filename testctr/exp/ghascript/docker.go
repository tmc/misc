package ghascript

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	buildxCheckOnce sync.Once
	buildxAvailable bool
	buildxWarned    bool
)

// checkDockerBuildx detects if Docker buildx is available and warns if not
func checkDockerBuildx() {
	buildxCheckOnce.Do(func() {
		buildxAvailable = isBuildxAvailable()
		if !buildxAvailable {
			fmt.Printf("WARNING: Docker buildx is not available. Some advanced workflow features may not work.\n")
			fmt.Printf("  To install buildx: docker buildx install\n")
			fmt.Printf("  Or update to Docker Desktop 2.4.0+ / Docker CE 19.03+\n")
			buildxWarned = true
		}
	})
}

// isBuildxAvailable checks if Docker buildx is installed and functional
func isBuildxAvailable() bool {
	// Check if buildx command exists
	cmd := exec.Command("docker", "buildx", "version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if default builder exists
	cmd = exec.Command("docker", "buildx", "ls")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Look for default builder in output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "default") && !strings.Contains(line, "inactive") {
			return true
		}
	}

	return false
}

// checkDockerVersion checks Docker version and capabilities
func checkDockerVersion() (string, error) {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Docker version: %v", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// warnIfBuildxNeeded issues a warning if buildx is needed but not available
func warnIfBuildxNeeded(action string) {
	checkDockerBuildx()
	
	if !buildxAvailable && !buildxWarned {
		fmt.Printf("WARNING: Action '%s' may require Docker buildx, but it's not available.\n", action)
		fmt.Printf("  Some features like multi-platform builds may fail.\n")
		buildxWarned = true
	}
}

// getBuildxInfo returns information about buildx availability
func getBuildxInfo() map[string]interface{} {
	checkDockerBuildx()
	
	info := map[string]interface{}{
		"available": buildxAvailable,
		"warned":    buildxWarned,
	}

	if buildxAvailable {
		// Get buildx version
		cmd := exec.Command("docker", "buildx", "version")
		if output, err := cmd.Output(); err == nil {
			version := strings.TrimSpace(string(output))
			// Extract version from output like "github.com/docker/buildx v0.10.4"
			parts := strings.Fields(version)
			if len(parts) >= 2 {
				info["version"] = parts[len(parts)-1]
			}
		}

		// Get available builders
		cmd = exec.Command("docker", "buildx", "ls")
		if output, err := cmd.Output(); err == nil {
			var builders []string
			lines := strings.Split(string(output), "\n")
			for _, line := range lines[1:] { // Skip header
				if line = strings.TrimSpace(line); line != "" {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						builders = append(builders, parts[0])
					}
				}
			}
			info["builders"] = builders
		}
	}

	return info
}

// ensureDockerCapabilities checks that Docker has required capabilities
func ensureDockerCapabilities() error {
	// Check if Docker is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running or not accessible: %v", err)
	}

	// Check Docker version
	version, err := checkDockerVersion()
	if err != nil {
		return fmt.Errorf("failed to check Docker version: %v", err)
	}

	fmt.Printf("Using Docker version: %s\n", version)

	// Check buildx availability (will warn if not available)
	checkDockerBuildx()

	return nil
}