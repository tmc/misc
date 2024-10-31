package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

var (
	verbose bool
	stream  bool
)

func main() {
	log.SetPrefix("scripttestctl: ")
	log.SetFlags(0)

	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&stream, "stream", false, "stream CGPT output")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
	}

	cmd := flag.Arg(0)
	dir := "."
	if flag.NArg() > 1 {
		dir = flag.Arg(1)
	}

	var err error
	switch cmd {
	case "scaffold":
		err = scaffold(dir)
	case "infer":
		err = infer(dir)
	default:
		log.Fatalf("unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: scripttestctl [-v] <command> [dir]\n")
	fmt.Fprintf(os.Stderr, "commands:\n")
	fmt.Fprintf(os.Stderr, "  scaffold    create scripttest scaffold\n")
	fmt.Fprintf(os.Stderr, "  infer       infer command info\n")
	os.Exit(2)
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

	prompt := generateScaffoldPrompt(info)
	resp, err := queryCGPT(prompt, filepath.Join(dir, ".scripttestctl_history"))
	if err != nil {
		return fmt.Errorf("failed to query CGPT: %v", err)
	}

	if verbose {
		log.Printf("CGPT response: %s", resp)
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

	file := filepath.Join(dir, ".scripttestctl_info")
	if err := ioutil.WriteFile(file, []byte(info), 0644); err != nil {
		return fmt.Errorf("failed to write command info: %v", err)
	}

	if verbose {
		log.Printf("command info written to: %s", file)
	}

	return nil
}

func loadOrInferCommandInfo(dir string) (string, error) {
	file := filepath.Join(dir, ".scripttestctl_info")
	info, err := ioutil.ReadFile(file)
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

	prompt := fmt.Sprintf("Analyze this codebase and identify key commands, binaries, and functions:\n\n%s", content)
	return queryCGPT(prompt, filepath.Join(dir, ".scripttestctl_history"))
}

func getCodebaseContent(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "ls-files")
	files, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list git files: %v", err)
	}

	var content strings.Builder
	for _, file := range strings.Fields(string(files)) {
		data, err := ioutil.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %v", file, err)
		}
		fmt.Fprintf(&content, "--- %s ---\n%s\n\n", file, string(data))
	}
	return content.String(), nil
}

func generateScaffoldPrompt(info string) string {
	return fmt.Sprintf("Generate a scripttest scaffold for this command info:\n\n%s", info)
}

func queryCGPT(prompt, historyFile string) (string, error) {
	args := []string{
		"-i", prompt,
		"-I", historyFile,
		"-O", historyFile,
		"--prefill", "```json\n{",
	}
	cmd := exec.Command("cgpt", args...)

	var output strings.Builder
	var stderr io.Writer = ioutil.Discard
	if stream {
		stderr = os.Stderr
	}

	cmd.Stdout = io.MultiWriter(&output, stderr)
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cgpt query failed: %v", err)
	}

	return extractJSON(output.String()), nil
}

func applyScaffold(dir string, resp string) error {
	var files map[string]string
	if err := json.Unmarshal([]byte(resp), &files); err != nil {
		// If not valid JSON, treat as a single file content
		files = map[string]string{"test_cowsay.py": resp}
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %v", path, err)
		}
		log.Printf("created %s", path)
	}
	return nil
}

func streamOutput(r io.Reader, w io.Writer, c *color.Color) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if c != nil {
			c.Fprintln(w, line)
		} else {
			fmt.Fprintln(w, line)
		}
	}
}

func extractJSON(output string) string {
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}\n```")
	if start != -1 && end != -1 && end > start {
		return output[start : end+1]
	}
	return output // Return the whole output if we can't find the JSON boundaries
}
