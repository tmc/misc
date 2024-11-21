package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func usage() {
	// Extract the content of the /* ... */ comment in doc.go.
	_, after, _ := strings.Cut(doc, "/*")
	doc, _, _ := strings.Cut(after, "*/")
	io.WriteString(flag.CommandLine.Output(), doc)
	flag.PrintDefaults()

	os.Exit(2)
}

//go:embed doc.go
var doc string

var (
	verbose bool
	stream  bool
	pattern string

	flagDebug bool
)

func main() {
	log.SetPrefix("scripttest: ")
	log.SetFlags(0)

	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&stream, "stream", false, "stream CGPT output")
	flag.BoolVar(&flagDebug, "debug", false, "debug CGPT output")
	flag.StringVar(&pattern, "p", "testdata/*.txt", "test file pattern")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
	}

	cmd := flag.Arg(0)
	var err error

	switch cmd {
	case "test", "run":
		// If pattern provided as argument, override flag
		if flag.NArg() > 1 {
			pattern = flag.Arg(1)
		}
		err = runTest(pattern)

	case "scaffold":
		dir := "."
		if flag.NArg() > 1 {
			dir = flag.Arg(1)
		}
		err = scaffold(dir)

	case "infer":
		dir := "."
		if flag.NArg() > 1 {
			dir = flag.Arg(1)
		}
		err = infer(dir)

	default:
		log.Fatalf("unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func scaffold(dir string) error {
	if verbose {
		log.Printf("scaffolding in directory: %s", dir)
	}

	info, err := loadOrInferCommandInfo(dir)
	if err != nil {
		return fmt.Errorf("failed to load or infer command info: %v", err)
	}

	if verbose {
		log.Printf("command info: %s", info)
	}

	prompt, err := generateScaffoldPrompt(info)
	if err != nil {
		return fmt.Errorf("failed to load prompt cgpt: %v", err)
	}
	resp, err := queryCgpt(prompt, filepath.Join(dir, ".scripttest_history"), "")
	if err != nil {
		return fmt.Errorf("failed to query cgpt: %v", err)
	}

	if verbose {
		log.Printf("cgpt response: %s", resp)
	}

	return applyScaffold(dir, resp)
}

func infer(dir string) error {
	if verbose {
		log.Printf("inferring command info in directory: %s", dir)
	}

	info, err := inferCommandInfo(dir)
	if err != nil {
		return fmt.Errorf("failed to infer command info: %v", err)
	}

	file := filepath.Join(dir, ".scripttest_info")
	if err := os.WriteFile(file, []byte(info), 0644); err != nil {
		return fmt.Errorf("failed to write command info: %v", err)
	}

	if verbose {
		log.Printf("command info written to: %s", file)
	}

	return nil
}

func loadOrInferCommandInfo(dir string) (string, error) {
	file := filepath.Join(dir, ".scripttest_info")
	info, err := os.ReadFile(file)
	if err == nil {
		if verbose {
			log.Printf("loaded existing command info from: %s", file)
		}
		return string(info), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read command info: %v", err)
	}
	if verbose {
		log.Printf("inferring command info")
	}
	return inferCommandInfo(dir)
}

func inferCommandInfo(dir string) (string, error) {
	content, err := getCodebaseContent(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get codebase content: %v", err)
	}

	prompt := fmt.Sprintf("Analyze this codebase and identify key binary entrypoints and commnds:\n\n%s\n\n", content)
	prompt += `output a json representation matching this datatype:
type Commands = CommandInfo[];
type CommandInfo = {
  name: string;    // command name
  summary: string; // usage summary
  args: string;    // argument pattern
}`
	res, err := queryCgpt(prompt, filepath.Join(dir, ".scripttest_history_infer"), "```json\n[")
	if err != nil {
		return "", fmt.Errorf("failed to run cgpt: %w", err)
	}
	return res, nil
}

func getCodebaseContent(dir string) (string, error) {
	// Check if code-to-gpt.sh exists in PATH
	scriptPath, err := exec.LookPath("code-to-gpt.sh")
	if err != nil {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = filepath.Join(os.Getenv("HOME"), "go")
			if runtime.GOOS == "windows" {
				gopath = filepath.Join(os.Getenv("USERPROFILE"), "go")
			}
		}

		binDir := filepath.Join(gopath, "bin")
		scriptPath = filepath.Join(binDir, "code-to-gpt.sh")

		if err := downloadScript(scriptPath); err != nil {
			return "", fmt.Errorf("failed to download code-to-gpt.sh: %v", err)
		}

		// Make executable
		if err := os.Chmod(scriptPath, 0755); err != nil {
			return "", fmt.Errorf("failed to make script executable: %v", err)
		}
	}

	args := []string{"--", ":!.scripttest_history*"}
	cmd := exec.Command(scriptPath, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run code-to-gpt: %v", err)
	}
	return string(output), nil
}

func downloadScript(destPath string) error {
	// Create bin directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Download script from a trusted source
	resp, err := http.Get("https://raw.githubusercontent.com/tmc/misc/master/code-to-gpt/code-to-gpt.sh")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func queryCgpt(prompt, historyFile string, prefill string) (string, error) {
	args := []string{
		"-I", historyFile,
		"-O", historyFile,
	}

	if flagDebug {
		args = append(args, "--debug")
	}

	if prefill != "" {
		args = append(args, "--prefill", prefill)
	} else {
		args = append(args, "--prefill", "```json\n{")
	}

	cmd := exec.Command("cgpt", args...)

	var output strings.Builder
	var stderr io.Writer = os.Stderr
	cmd.Stdin = strings.NewReader(prompt)

	cmd.Stdout = &output
	if stream {
		stderr = os.Stderr
		cmd.Stdout = io.MultiWriter(&output, stderr)
	}
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cgpt query failed: %v", err)
	}
	return extractJSON(output.String()), nil
}

func runTest(pattern string) error {
	if verbose {
		log.Printf("running tests matching pattern: %s", pattern)
	}

	// Get clean work directory
	dir, err := getWorkDir()
	if err != nil {
		return fmt.Errorf("failed to get work directory: %v", err)
	}

	if verbose {
		log.Printf("using work directory: %s", dir)
	}

	// Create testdata directory
	testdata := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(testdata, 0755); err != nil {
		return fmt.Errorf("failed to create testdata directory: %v", err)
	}

	// Set up test files in work directory
	if err := setupTestDir(dir); err != nil {
		return fmt.Errorf("failed to setup test directory: %v", err)
	}

	// Find matching test files
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %v", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no files match pattern: %s", pattern)
	}

	// Create symlinks in testdata directory
	for _, file := range matches {
		abs, err := filepath.Abs(file)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %v", file, err)
		}
		dst := filepath.Join(testdata, filepath.Base(file))
		if err := os.Symlink(abs, dst); err != nil {
			return fmt.Errorf("failed to link test file %s: %v", file, err)
		}
	}

	// link in .scripttest_info if it exists:
	scriptTestInfo := ".scripttest_info"
	if _, err := os.Stat(scriptTestInfo); err == nil {
		abs, err := filepath.Abs(scriptTestInfo)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for .scripttest_info: %v", err)
		}
		dst := filepath.Join(dir, ".scripttest_info")
		if err := os.Symlink(abs, dst); err != nil {
			return fmt.Errorf("failed to link .scripttest_info: %v", err)
		}
	}

	// Initialize go modules
	if err := initModules(dir); err != nil {
		return fmt.Errorf("failed to initialize modules: %v", err)
	}

	buildID := getBuildID()
	if verbose {
		log.Printf("build ID: %s", buildID)
	}

	// Run go test in the directory
	args := []string{"test"}
	if verbose {
		args = append(args, "-v")
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %v", err)
	}

	return nil
}

func applyScaffold(dir string, resp string) error {
	var files map[string]string
	if err := json.Unmarshal([]byte(resp), &files); err != nil {
		// TODO
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %v", path, err)
		}
		log.Printf("created %s", path)
	}
	return nil
}

func extractJSON(output string) string {
	// Try to find JSON between markdown code fences
	prefix := "```json"
	suffix := "```"
	start := strings.Index(output, prefix)
	if start == -1 {
		// Try alternate code fence
		prefix = "~~~json"
		start = strings.Index(output, prefix)
	}
	if start != -1 {
		start += len(prefix)
		// Find closing fence after the start position
		end := strings.Index(output[start:], suffix)
		if end != -1 {
			// Trim whitespace and validate JSON
			jsonStr := strings.TrimSpace(output[start : start+end])
			if json.Valid([]byte(jsonStr)) {
				return jsonStr
			}
		}
	}
	// If no valid JSON found between fences, try to find and validate any JSON in the string
	if json.Valid([]byte(output)) {
		return output
	}
	return "" // Return empty if no valid JSON found
}

// getCacheDir returns the scripttest cache directory, creating it if needed
func getCacheDir() (string, error) {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %v", err)
		}
		cacheDir = filepath.Join(home, ".cache")
	}

	scripttestCache := filepath.Join(cacheDir, "scripttest")
	if err := os.MkdirAll(scripttestCache, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %v", err)
	}

	workDir := filepath.Join(scripttestCache, "workdir")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %v", err)
	}

	return workDir, nil
}

// getWorkDir returns a clean working directory for the test run
func getWorkDir() (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}

	// Clean any existing content
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to read cache directory: %v", err)
	}
	for _, entry := range entries {
		path := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return "", fmt.Errorf("failed to clean cache entry %s: %v", entry.Name(), err)
		}
	}

	// Create fresh workdir
	workDir := filepath.Join(cacheDir, "current")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %v", err)
	}

	return workDir, nil
}

func initModules(dir string) error {
	// Run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = os.Environ() // Ensure we pass through GO111MODULE, GOPATH etc.

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %v\n%s", err, stderr.String())
	}

	return nil
}
