package testctr

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	// Flags for controlling testctr behavior
	verbose       = flag.Bool("testctr.verbose", false, "Stream container logs to test output.")
	keepOnFailure = flag.Bool("testctr.keep-failed", false, "Keep containers running when tests fail (for debugging).")
	labelPrefix   = flag.String("testctr.label", "testctr", "Label prefix for containers created by testctr.")
	warnOld       = flag.Bool("testctr.warn-old", true, "Warn about testctr containers older than cleanup-age.")
	cleanupOld    = flag.Bool("testctr.cleanup-old", true, "Clean up testctr containers older than cleanup-age before test run.")
	cleanupAge    = flag.Duration("testctr.cleanup-age", 5*time.Minute, "Age threshold for old container cleanup/warning.")
	maxConcurrent = flag.Int("testctr.max-concurrent", 20, "Maximum number of containers that can start simultaneously by the default CLI backend.")
	createDelay   = flag.Duration("testctr.create-delay", 200*time.Millisecond, "Delay between container creations when max-concurrent is reached (default CLI backend).")
)

var (
	// modulePath is detected once at startup, used for namespacing labels.
	modulePath string
	// checkOldOnce ensures we only check for old containers once per test run.
	checkOldOnce sync.Once
)

func init() {
	// Detect module path for namespacing container labels.
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path != "" {
		// Sanitize module path for use in labels (e.g., replace slashes)
		modulePath = strings.ReplaceAll(info.Main.Path, "/", "_")
		modulePath = strings.ReplaceAll(modulePath, ".", "-")
	}

	// Override flags with environment variables if set.
	overrideBoolFlagFromEnv("TESTCTR_VERBOSE", verbose)
	overrideBoolFlagFromEnv("TESTCTR_KEEP_FAILED", keepOnFailure)
	overrideStringFlagFromEnv("TESTCTR_LABEL", labelPrefix)
	overrideBoolFlagFromEnv("TESTCTR_WARN_OLD", warnOld)
	overrideBoolFlagFromEnv("TESTCTR_CLEANUP_OLD", cleanupOld)
	overrideDurationFlagFromEnv("TESTCTR_CLEANUP_AGE", cleanupAge)
	overrideIntFlagFromEnv("TESTCTR_MAX_CONCURRENT", maxConcurrent)
	overrideDurationFlagFromEnv("TESTCTR_CREATE_DELAY", createDelay)
}

// Helper functions to override flags from environment variables.
func overrideBoolFlagFromEnv(envVar string, flagVal *bool) {
	if val := os.Getenv(envVar); val != "" {
		*flagVal = (val == "true" || val == "1")
	}
}

func overrideStringFlagFromEnv(envVar string, flagVal *string) {
	if val := os.Getenv(envVar); val != "" {
		*flagVal = val
	}
}

func overrideDurationFlagFromEnv(envVar string, flagVal *time.Duration) {
	if val := os.Getenv(envVar); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			*flagVal = d
		}
	}
}
func overrideIntFlagFromEnv(envVar string, flagVal *int) {
	if valStr := os.Getenv(envVar); valStr != "" {
		if valInt, err := strconv.Atoi(valStr); err == nil {
			*flagVal = valInt
		}
	}
}

// containerLabels returns the labels to apply to a container for the CLI backend.
// These labels help identify containers managed by testctr.
func containerLabels(t testing.TB, image string) map[string]string {
	labels := map[string]string{
		*labelPrefix:                "true", // Base label to identify all testctr containers
		*labelPrefix + ".testname":  sanitizeLabelValue(t.Name()),
		*labelPrefix + ".image":     sanitizeLabelValue(image),
		*labelPrefix + ".timestamp": time.Now().Format(time.RFC3339),
	}

	if modulePath != "" {
		labels[*labelPrefix+".module"] = sanitizeLabelValue(modulePath)
	}

	// Add package name if we can get it, useful for multi-module projects
	if pc, _, _, ok := runtime.Caller(3); ok { // Adjust caller depth if this moves
		if fn := runtime.FuncForPC(pc); fn != nil {
			fullName := fn.Name()
			lastDot := strings.LastIndex(fullName, ".")
			if lastDot > 0 {
				packagePath := fullName[:lastDot]
				labels[*labelPrefix+".package"] = sanitizeLabelValue(packagePath)
			}
		}
	}
	return labels
}

// sanitizeLabelValue ensures the value is valid for a Docker label.
// Docker label values have some restrictions (e.g., length, characters).
func sanitizeLabelValue(value string) string {
	sanitized := strings.ReplaceAll(value, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "-") // Colons can be problematic
	const maxLabelValueLength = 63
	if len(sanitized) > maxLabelValueLength {
		sanitized = sanitized[:maxLabelValueLength]
	}
	return sanitized
}

// checkOldContainers warns about or cleans up old containers based on flags.
// This is run once per test suite.
func checkOldContainers(t testing.TB) {
	if !*warnOld && !*cleanupOld {
		return
	}

	checkOldOnce.Do(func() {
		// This function will use the default CLI backend's runtime determination logic.
		// It assumes that old containers were also created by a CLI backend.
		rt := discoverContainerRuntime() // Get runtime
		doCheckOldContainersCLI(t, rt)
	})
}

// doCheckOldContainersCLI performs the actual checking and cleanup of old containers using CLI.
func doCheckOldContainersCLI(t testing.TB, runtime string) {
	filter := fmt.Sprintf("label=%s", *labelPrefix)

	cmdPs := exec.Command(runtime, "ps", "-a", "--filter", filter, "--format", "{{.ID}}")
	outputPs, errPs := cmdPs.Output()
	if errPs != nil {
		t.Logf("Failed to list containers for cleanup check (runtime: %s): %v", runtime, errPs)
		return
	}

	containerIDs := strings.Fields(string(outputPs))
	if len(containerIDs) == 0 {
		return
	}

	var oldContainersToClean []string
	now := time.Now()

	for _, id := range containerIDs {
		cmdInspect := exec.Command(runtime, "inspect", id, "--format", "{{.Created}} {{.Name}} {{.Config.Labels}}")
		outputInspect, errInspect := cmdInspect.Output()
		if errInspect != nil {
			t.Logf("Failed to inspect container %s for cleanup (runtime: %s): %v", id, runtime, errInspect)
			continue
		}

		parts := strings.Fields(string(outputInspect))
		if len(parts) < 1 {
			continue
		}

		createdTimeStr := parts[0]
		var containerName string
		if len(parts) > 1 {
			containerName = strings.TrimPrefix(parts[1], "/")
		} else {
			containerName = id[:min(12, len(id))]
		}

		var labelsStr string
		if len(parts) > 2 {
			labelsStr = strings.Join(parts[2:], " ")
		}

		createdTime, err := time.Parse(time.RFC3339Nano, createdTimeStr)
		if err != nil {
			t.Logf("Failed to parse creation time '%s' for container %s (runtime: %s): %v", createdTimeStr, id, runtime, err)
			continue
		}

		age := now.Sub(createdTime)
		if age > *cleanupAge {
			isOurModuleContainer := false
			moduleLabelKey := *labelPrefix + ".module"
			if modulePath != "" && strings.Contains(labelsStr, fmt.Sprintf("%s:%s", moduleLabelKey, sanitizeLabelValue(modulePath))) {
				isOurModuleContainer = true
			} else if modulePath == "" {
				isOurModuleContainer = true
			}

			if *warnOld {
				sourceInfo := ""
				if modulePath != "" {
					if isOurModuleContainer {
						sourceInfo = fmt.Sprintf(" (from module: %s)", modulePath)
					} else {
						otherModuleLabel := extractLabelValue(labelsStr, moduleLabelKey)
						if otherModuleLabel != "" {
							sourceInfo = fmt.Sprintf(" (from different module: %s)", otherModuleLabel)
						} else {
							sourceInfo = " (module unknown or different project)"
						}
					}
				}
				t.Logf("WARNING: Found old testctr container %s (ID: %s) created %v ago%s (runtime: %s). Run tests with -testctr.cleanup-old flag to remove old containers.",
					containerName, id[:min(12, len(id))], age.Round(time.Second), sourceInfo, runtime)
			}

			if *cleanupOld && isOurModuleContainer {
				oldContainersToClean = append(oldContainersToClean, id)
			}
		}
	}

	if *cleanupOld && len(oldContainersToClean) > 0 {
		rmArgs := append([]string{"rm", "-f"}, oldContainersToClean...)
		cmdRm := exec.Command(runtime, rmArgs...)
		if errRm := cmdRm.Run(); errRm == nil {
			t.Logf("Cleaned up %d old testctr containers from this module (runtime: %s).", len(oldContainersToClean), runtime)
		} else {
			t.Logf("Failed to clean up old testctr containers (runtime: %s): %v", runtime, errRm)
		}
	}
}

func extractLabelValue(labelsStr, key string) string {
	searchKeyFormatted := fmt.Sprintf("%s:", key) // e.g., "testctr.module:"
	idx := strings.Index(labelsStr, searchKeyFormatted)
	if idx == -1 {
		// Try with map notation too e.g. map[testctr.module:value]
		searchKeyFormatted = fmt.Sprintf("%s:", key) // map key is usually just the key
		idx = strings.Index(labelsStr, searchKeyFormatted)
		if idx == -1 {
			return ""
		}
	}

	valStart := idx + len(searchKeyFormatted)
	if valStart >= len(labelsStr) {
		return ""
	}

	// Extract value; it might be quoted or unquoted, and followed by space or map end ']'
	valEnd := valStart
	inQuote := false
	if labelsStr[valStart] == '"' {
		inQuote = true
		valStart++
		valEnd = valStart
	}

	for valEnd < len(labelsStr) {
		if inQuote {
			if labelsStr[valEnd] == '"' {
				// Check for escaped quote
				if valEnd > valStart && labelsStr[valEnd-1] == '\\' {
					valEnd++
					continue
				}
				break // End of quoted value
			}
		} else {
			if labelsStr[valEnd] == ' ' || labelsStr[valEnd] == ']' {
				break // End of unquoted value
			}
		}
		valEnd++
	}

	if valStart >= valEnd { // Should not happen if idx was found
		return ""
	}

	return strings.ReplaceAll(labelsStr[valStart:valEnd], `\"`, `"`) // Unescape quotes
}
