package testctr

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	// Flags for controlling testctr behavior
	verbose       = flag.Bool("testctr.verbose", false, "Stream container logs to test output")
	keepOnFailure = flag.Bool("testctr.keep-failed", false, "Keep containers running when tests fail (for debugging)")
	labelPrefix   = flag.String("testctr.label", "testctr", "Label prefix for containers")
	warnOld       = flag.Bool("testctr.warn-old", true, "Warn about testctr containers older than cleanup-age")
	cleanupOld    = flag.Bool("testctr.cleanup-old", false, "Clean up testctr containers older than cleanup-age")
	cleanupAge    = flag.Duration("testctr.cleanup-age", 5*time.Minute, "Age threshold for old container cleanup/warning")
	maxConcurrent = flag.Int("testctr.max-concurrent", 20, "Maximum number of containers that can start simultaneously")
	createDelay   = flag.Duration("testctr.create-delay", 200*time.Millisecond, "Delay between container creations")
)

var (
	// modulePath is detected once at startup
	modulePath string
	// checkOldOnce ensures we only check for old containers once per test run
	checkOldOnce sync.Once
)

func init() {
	// Detect module path for namespacing
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path != "" {
		modulePath = info.Main.Path
		// Module path is fine as-is for Docker labels
	}

	// Also check environment variables
	if os.Getenv("TESTCTR_VERBOSE") == "true" || os.Getenv("TESTCTR_VERBOSE") == "1" {
		*verbose = true
	}
	if os.Getenv("TESTCTR_KEEP_FAILED") == "true" || os.Getenv("TESTCTR_KEEP_FAILED") == "1" {
		*keepOnFailure = true
	}
	if label := os.Getenv("TESTCTR_LABEL"); label != "" {
		*labelPrefix = label
	}
	if os.Getenv("TESTCTR_WARN_OLD") == "true" || os.Getenv("TESTCTR_WARN_OLD") == "1" {
		*warnOld = true
	}
	if os.Getenv("TESTCTR_CLEANUP_OLD") == "true" || os.Getenv("TESTCTR_CLEANUP_OLD") == "1" {
		*cleanupOld = true
	}
	if age := os.Getenv("TESTCTR_CLEANUP_AGE"); age != "" {
		if d, err := time.ParseDuration(age); err == nil {
			*cleanupAge = d
		}
	}
}

// containerLabels returns the labels to apply to a container
func containerLabels(t testing.TB, image string) map[string]string {
	labels := map[string]string{
		*labelPrefix:                "true",
		*labelPrefix + ".test":      t.Name(),
		*labelPrefix + ".image":     image,
		*labelPrefix + ".timestamp": time.Now().Format(time.RFC3339),
	}

	// Add module path for namespacing if available
	if modulePath != "" {
		labels[*labelPrefix+".module"] = modulePath
	}

	// Add package name if we can get it
	if pc, _, _, ok := runtime.Caller(3); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			fullName := fn.Name()
			// Extract package path (everything before the last dot)
			lastDot := strings.LastIndex(fullName, ".")
			if lastDot > 0 {
				packagePath := fullName[:lastDot]
				// Package path is fine as-is for Docker labels
				labels[*labelPrefix+".package"] = packagePath
			}
		}
	}

	return labels
}


// checkOldContainers warns about or cleans up old containers
func checkOldContainers(t testing.TB) {
	if !*warnOld && !*cleanupOld {
		return
	}

	// Use sync.Once to ensure we only check once per test run
	checkOldOnce.Do(func() {
		doCheckOldContainers(t)
	})
}

// doCheckOldContainers performs the actual checking (called once via sync.Once)
func doCheckOldContainers(t testing.TB) {
	// For warnings, get ALL testctr containers
	// For cleanup, only get containers from this module
	warnFilter := fmt.Sprintf("label=%s", *labelPrefix)

	// First get container IDs
	cmd := exec.Command(getContainerRuntime(), "ps", "-a", "--filter", warnFilter, "--format", "{{.ID}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	containerIDs := strings.Fields(string(output))
	if len(containerIDs) == 0 {
		return
	}

	// Now inspect each container for detailed info
	var oldContainers []string
	now := time.Now()

	for _, containerID := range containerIDs {
		// Get detailed info about the container
		cmd := exec.Command(getContainerRuntime(), "inspect", containerID, "--format", "{{.Created}} {{.Name}} {{.Config.Labels}}")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		parts := strings.Fields(string(output))
		if len(parts) < 2 {
			continue
		}

		// Parse ISO format timestamp (e.g., 2025-05-25T09:45:23.123456789Z)
		created, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			// Try without nanoseconds
			created, err = time.Parse(time.RFC3339, parts[0])
			if err != nil {
				continue
			}
		}

		age := now.Sub(created)
		if age > *cleanupAge {
			// Extract container name (remove leading /)
			containerName := strings.TrimPrefix(parts[1], "/")

			// Extract labels
			labels := ""
			if len(parts) > 2 {
				labels = strings.Join(parts[2:], " ")
			}

			// Check if this container belongs to our module
			isOurModule := false
			if modulePath != "" && strings.Contains(labels, fmt.Sprintf("%s.module:%s", *labelPrefix, modulePath)) {
				isOurModule = true
			} else if modulePath == "" {
				// If no module path, consider all testctr containers as ours
				isOurModule = true
			}

			if *warnOld {
				moduleInfo := ""
				if !isOurModule && modulePath != "" {
					moduleInfo = " (from different project)"
				}
				t.Logf("WARNING: Found old testctr container %s (%s) created %v ago%s", containerName, containerID[:12], age.Round(time.Second), moduleInfo)
			}

			if *cleanupOld && isOurModule {
				oldContainers = append(oldContainers, containerID)
			}
		}
	}

	// Clean up old containers if requested
	if *cleanupOld && len(oldContainers) > 0 {
		args := append([]string{"rm", "-f"}, oldContainers...)
		if err := exec.Command(getContainerRuntime(), args...).Run(); err == nil {
			t.Logf("Cleaned up %d old testctr containers", len(oldContainers))
		}
	}
}
