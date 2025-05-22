package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	serverURL    = flag.String("server", "http://localhost:8080", "ctx-src-server URL")
	githubRepo   = flag.String("repo", "", "GitHub repository in owner/repo format")
	ref          = flag.String("ref", "main", "Git reference (branch, tag, or commit)")
	paths        = flag.String("paths", "", "Comma-separated list of paths to include")
	excludes     = flag.String("excludes", "", "Comma-separated list of paths to exclude")
	noXML        = flag.Bool("no-xml", false, "Disable XML tags in output")
	outputFile   = flag.String("output", "", "Output file (default: stdout)")
	timeout      = flag.Duration("timeout", 5*time.Minute, "Request timeout")
)

type RepoRequest struct {
	Owner    string   `json:"owner"`
	Repo     string   `json:"repo"`
	Ref      string   `json:"ref,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	NoXML    bool     `json:"no_xml,omitempty"`
}

func main() {
	flag.Parse()

	if *githubRepo == "" {
		fmt.Fprintln(os.Stderr, "Error: GitHub repository is required")
		flag.Usage()
		os.Exit(1)
	}

	// Parse owner/repo
	parts := strings.Split(*githubRepo, "/")
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Error: Invalid repository format. Expected owner/repo")
		os.Exit(1)
	}
	owner, repo := parts[0], parts[1]

	// Prepare request
	reqBody := RepoRequest{
		Owner:    owner,
		Repo:     repo,
		Ref:      *ref,
		NoXML:    *noXML,
	}

	// Parse paths and excludes
	if *paths != "" {
		reqBody.Paths = strings.Split(*paths, ",")
	}
	if *excludes != "" {
		reqBody.Excludes = strings.Split(*excludes, ",")
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: *timeout,
	}

	// Create request
	url := *serverURL + "/src"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make request
	fmt.Fprintf(os.Stderr, "Fetching source code from %s/%s@%s...\n", owner, repo, *ref)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error making request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error response (status %d): %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	// Output the response
	var output io.Writer = os.Stdout
	if *outputFile != "" {
		file, err := os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		output = file
		fmt.Fprintf(os.Stderr, "Writing output to %s\n", *outputFile)
	}

	_, err = io.Copy(output, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
}