package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	// If no arguments are provided, pass through to `go doc` for help.
	if len(os.Args) == 1 {
		cmd := exec.Command("go", "doc")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		os.Exit(2) // Exit with the same status code as `go doc -h`
	}

	// Run the main logic of the tool.
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1) // Exit with a non-zero status code on error.
	}
}

// run executes the main logic of the tool.
func run(args []string) error {
	// Construct the command string for output.
	cmdStr := strings.Join(append([]string{"go", "doc"}, args...), " ")

	// Execute the `go doc` command, handling potential module initialization.
	stdout, stderr, err := executeGoDoc(args)

	// Wrap the output in XML-like tags.
	fmt.Printf("<exec-output cmd=%q>\n", cmdStr)
	if stdout != "" {
		fmt.Printf("<stdout>\n%s</stdout>\n", stdout)
	}
	if stderr != "" {
		fmt.Printf("<stderr>\n%s</stderr>\n", stderr)
	}
	if err != nil {
		fmt.Printf("<e>%s</e>\n", err)
	}
	fmt.Println("</exec-output>")

	return err
}

// isInGoModule returns true if the current directory is inside a Go module.
func isInGoModule() bool {
	cmd := exec.Command("go", "list", "-m")
	err := cmd.Run()
	return err == nil
}

// parsePackageAndVersion parses a package argument to extract the package path and version.
// Returns the package path, version (or empty string if no version), and whether a version was specified.
func parsePackageAndVersion(pkgArg string) (string, string, bool) {
	parts := strings.SplitN(pkgArg, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return pkgArg, "", false
}

// isPackageInModule checks if the given package is a dependency in the current module.
func isPackageInModule(pkgPath string) bool {
	cmd := exec.Command("go", "list", "-m", "all")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	// Create a regexp to match the package path.
	// Go list -m all outputs in the format: module/path vX.Y.Z [replacement]
	// We need to match against the module path part
	pkgPathBase := pkgPath
	// If it has a subpackage, we need to match the base module
	if index := strings.Index(pkgPath, "/"); index > 0 {
		parts := strings.SplitN(pkgPath, "/", 4) // Handle github.com/user/repo/subpkg
		if len(parts) >= 3 {
			// For github.com/user/repo/subpkg, we want github.com/user/repo
			pkgPathBase = strings.Join(parts[:3], "/")
		}
	}

	// Check each line for the package path
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 1 && strings.HasPrefix(fields[0], pkgPathBase) {
			return true
		}
	}

	return false
}

// executeGoDoc executes the `go doc` command, handling module initialization if needed.
func executeGoDoc(args []string) (string, string, error) {
	// Extract package path from arguments
	var pkgPath string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			pkgPath = arg
			break
		}
	}

	// Parse package and version
	var versionSpecified bool
	var version string
	if pkgPath != "" {
		pkgPath, version, versionSpecified = parsePackageAndVersion(pkgPath)
	}

	// Build new args with possibly modified package path
	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == pkgPath+"@"+version {
			newArgs = append(newArgs, pkgPath) // Use cleaned package path
		} else if !strings.HasPrefix(arg, "-") && arg == pkgPath {
			newArgs = append(newArgs, arg) // Keep original arg if it's the package without version
		} else {
			newArgs = append(newArgs, arg) // Keep all other args as is
		}
	}

	// If we're in a Go module and no version is specified, try to use local package
	if isInGoModule() && !versionSpecified && pkgPath != "" {
		// Check if the package is a dependency in the current module
		if isPackageInModule(pkgPath) {
			debugf("Package %s is a dependency in the current module, using local version", pkgPath)
			stdout, stderr, err := runGoDoc(newArgs)
			if err == nil {
				return stdout, stderr, nil
			}
			debugf("Failed to get doc from local module: %v", err)
		}
	}

	// If a version is specified or we need to fetch the package
	if versionSpecified || (pkgPath != "" && !isPackageInModule(pkgPath)) {
		return fetchAndDocWithVersion(newArgs, pkgPath, version)
	}

	// Try direct go doc command as fallback
	return runGoDoc(newArgs)
}

// fetchAndDocWithVersion fetches a specific version of a package and gets its documentation.
func fetchAndDocWithVersion(args []string, pkgPath, version string) (string, string, error) {
	// Create cache directory if it doesn't exist
	cacheDir := filepath.Join(os.TempDir(), "ctx-go-doc-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create cache dir: %v", err)
	}

	// Create a deterministic subdir for this package version
	var pkgDir string
	if version != "" {
		pkgDir = filepath.Join(cacheDir, fmt.Sprintf("%s@%s", sanitizePath(pkgPath), version))
	} else {
		pkgDir = filepath.Join(cacheDir, sanitizePath(pkgPath))
	}

	// Check if we already have a cache for this package/version
	if _, err := os.Stat(filepath.Join(pkgDir, "go.mod")); err != nil {
		// Need to create or recreate the package directory
		if err := os.RemoveAll(pkgDir); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("failed to clean package dir: %v", err)
		}
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create package dir: %v", err)
		}

		debugf("created package dir in %s", pkgDir)

		// Initialize a new Go module
		initCmd := exec.Command("go", "mod", "init", "tmp")
		initCmd.Dir = pkgDir
		out, err := initCmd.CombinedOutput()
		if err != nil {
			return "", string(out), fmt.Errorf("failed to initialize module: %v\n%s", err, out)
		}

		// Fetch the package with version if specified
		var getArgs []string
		if version != "" {
			getArgs = []string{"get", pkgPath + "@" + version}
			debugf("fetching package %s@%s", pkgPath, version)
		} else {
			getArgs = []string{"get", pkgPath}
			debugf("fetching latest package %s", pkgPath)
		}

		getCmd := exec.Command("go", getArgs...)
		getCmd.Dir = pkgDir
		out, err = getCmd.CombinedOutput()
		if err != nil {
			return "", string(out), fmt.Errorf("failed to get package: %v\n%s", err, out)
		}
	} else {
		debugf("using cached package at %s", pkgDir)
	}

	// Run go doc in the package directory
	cmdArgs := append([]string{"doc"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = pkgDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// sanitizePath creates a safe directory name from a package path.
func sanitizePath(path string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
	return re.ReplaceAllString(path, "_")
}

// runGoDoc executes the `go doc` command with the given arguments.
func runGoDoc(args []string) (string, string, error) {
	cmdArgs := append([]string{"doc"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// debugf prints a debug message to stderr if the CTX_GO_DOC_DEBUG environment variable is set to "true".
func debugf(format string, args ...interface{}) {
	if os.Getenv("CTX_GO_DOC_DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "ctx-go-doc: "+format+"\n", args...)
	}
}