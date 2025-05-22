package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"sync/atomic"
)

var (
	addr           = flag.String("addr", ":8080", "Address to listen on")
	cacheDir       = flag.String("cache-dir", "/tmp/ctx-src-cache", "Directory for caching repositories")
	gcsBucket      = flag.String("gcs-bucket", "", "GCS bucket name for gcsfuse cache (if empty, local cache will be used)")
	gcsMountPoint  = flag.String("gcs-mount", "/mnt/ctx-src-cache", "Mount point for gcsfuse bucket")
	cloneTimeout   = flag.Duration("clone-timeout", 5*time.Minute, "Timeout for cloning repositories")
	maxConcurrent  = flag.Int("max-concurrent", 5, "Maximum number of concurrent git operations")
	verbose        = flag.Bool("verbose", false, "Enable verbose logging")
	ctxSrcPath     = flag.String("ctx-src-path", "", "Path to ctx-src binary (if empty, assumed to be in PATH)")
	defaultBranch  = flag.String("default-branch", "main", "Default branch to use if none specified")
)

// Simple cache to track in-progress clones to prevent duplicate operations
var (
	inProgressMutex sync.Mutex
	inProgressRepos = make(map[string]chan struct{})
	semaphore       = make(chan struct{}, 5) // Default to 5 concurrent operations

	// Metrics
	requestCount       atomic.Int64
	successCount       atomic.Int64
	errorCount         atomic.Int64
	totalProcessingTime atomic.Int64 // in milliseconds
	serverStartTime    = time.Now()
	cacheHits          atomic.Int64
	cacheMisses        atomic.Int64
	totalBytesServed   atomic.Int64
)

type RepoRequest struct {
	Owner    string   `json:"owner"`
	Repo     string   `json:"repo"`
	Ref      string   `json:"ref,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	NoXML    bool     `json:"no_xml,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MetricsResponse struct {
	Uptime            string  `json:"uptime"`
	RequestCount      int64   `json:"request_count"`
	SuccessCount      int64   `json:"success_count"`
	ErrorCount        int64   `json:"error_count"`
	SuccessRate       float64 `json:"success_rate"`
	AvgProcessingTime float64 `json:"avg_processing_time_ms"`
	CacheHits         int64   `json:"cache_hits"`
	CacheMisses       int64   `json:"cache_misses"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
	TotalBytesServed  int64   `json:"total_bytes_served"`
	CurrentGoroutines int     `json:"current_goroutines"`
	NumCPU            int     `json:"num_cpu"`
	GoVersion         string  `json:"go_version"`
	GitOperations     int     `json:"concurrent_git_operations"`
}

func main() {
	flag.Parse()

	// Set up semaphore with configured value
	semaphore = make(chan struct{}, *maxConcurrent)

	// Set up cache directory
	if *gcsBucket != "" {
		// Set up gcsfuse if GCS bucket is provided
		if err := setupGCSFuse(); err != nil {
			log.Fatalf("Failed to set up gcsfuse: %v", err)
		}
		*cacheDir = *gcsMountPoint
	}

	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
	}

	// Find ctx-src binary
	if *ctxSrcPath == "" {
		path, err := exec.LookPath("ctx-src")
		if err != nil {
			log.Fatal("ctx-src binary not found in PATH. Please specify with -ctx-src-path flag.")
		}
		*ctxSrcPath = path
	}

	// Register handlers
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/metrics", handleMetricsRequest)
	http.HandleFunc("/src", handleSourceRequest)

	log.Printf("Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func setupGCSFuse() error {
	// Check if gcsfuse is installed
	if _, err := exec.LookPath("gcsfuse"); err != nil {
		return fmt.Errorf("gcsfuse not found: %v", err)
	}

	// Create mount point if it doesn't exist
	if err := os.MkdirAll(*gcsMountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %v", err)
	}

	// Check if already mounted
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check mount status: %v", err)
	}

	if strings.Contains(string(output), *gcsMountPoint) {
		log.Printf("GCS bucket already mounted at %s", *gcsMountPoint)
		return nil
	}

	// Mount the GCS bucket
	cmd = exec.Command("gcsfuse", "--foreground=false", *gcsBucket, *gcsMountPoint)
	if *verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount GCS bucket: %v", err)
	}
	
	log.Printf("Mounted GCS bucket %s at %s", *gcsBucket, *gcsMountPoint)
	return nil
}

func handleSourceRequest(w http.ResponseWriter, r *http.Request) {
	// Increment request counter
	requestCount.Add(1)
	
	// Track processing time
	startTime := time.Now()
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		errorCount.Add(1)
		return
	}

	// Parse request
	var req RepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, fmt.Sprintf("Invalid request: %v", err))
		errorCount.Add(1)
		return
	}

	// Validate request
	if req.Owner == "" || req.Repo == "" {
		respondWithError(w, "Owner and repo are required")
		errorCount.Add(1)
		return
	}

	// Set default branch if not specified
	if req.Ref == "" {
		req.Ref = *defaultBranch
	}

	// Process repository and get source code
	src, err := processRepo(req)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to process repository: %v", err))
		errorCount.Add(1)
		return
	}

	// Increment success counter
	successCount.Add(1)
	
	// Update processing time
	processingTime := time.Since(startTime).Milliseconds()
	totalProcessingTime.Add(processingTime)
	
	// Update bytes served
	totalBytesServed.Add(int64(len(src)))

	// Return the source code
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(src))
}

func respondWithError(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(ErrorResponse{Error: errMsg})
}

func processRepo(req RepoRequest) (string, error) {
	repoPath := filepath.Join(*cacheDir, req.Owner, req.Repo)
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", req.Owner, req.Repo)
	
	// Check if repo already exists locally (cache hit)
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		cacheHits.Add(1)
	} else {
		cacheMisses.Add(1)
	}
	
	// Acquire semaphore to limit concurrent git operations
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	// Check if the repository is already being processed
	inProgressMutex.Lock()
	ch, exists := inProgressRepos[repoURL]
	if exists {
		// Wait for the existing clone to complete
		inProgressMutex.Unlock()
		select {
		case <-ch:
			// Clone completed
		case <-time.After(*cloneTimeout):
			return "", fmt.Errorf("timeout waiting for repository clone to complete")
		}
	} else {
		// Create a new channel to signal when clone is complete
		ch = make(chan struct{})
		inProgressRepos[repoURL] = ch
		inProgressMutex.Unlock()
		
		// Clean up when done
		defer func() {
			inProgressMutex.Lock()
			delete(inProgressRepos, repoURL)
			close(ch)
			inProgressMutex.Unlock()
		}()
		
		// Clone or update the repository
		if err := cloneOrUpdateRepo(repoURL, repoPath, req.Ref); err != nil {
			return "", err
		}
	}

	// Run ctx-src to get the source code
	return getSourceCode(repoPath, req)
}

func cloneOrUpdateRepo(repoURL, repoPath, ref string) error {
	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		// Clone the repository
		log.Printf("Cloning repository %s to %s", repoURL, repoPath)
		
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
		
		cmd := exec.Command("git", "clone", repoURL, repoPath)
		if *verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %v", err)
		}
	} else {
		// Update the repository
		log.Printf("Updating repository %s", repoPath)
		
		// Reset any local changes and update
		resetCmd := exec.Command("git", "-C", repoPath, "reset", "--hard", "HEAD")
		if *verbose {
			resetCmd.Stdout = os.Stdout
			resetCmd.Stderr = os.Stderr
		}
		if err := resetCmd.Run(); err != nil {
			return fmt.Errorf("git reset failed: %v", err)
		}
		
		fetchCmd := exec.Command("git", "-C", repoPath, "fetch", "origin")
		if *verbose {
			fetchCmd.Stdout = os.Stdout
			fetchCmd.Stderr = os.Stderr
		}
		if err := fetchCmd.Run(); err != nil {
			return fmt.Errorf("git fetch failed: %v", err)
		}
	}

	// Checkout the specified reference
	checkoutCmd := exec.Command("git", "-C", repoPath, "checkout", ref)
	if *verbose {
		checkoutCmd.Stdout = os.Stdout
		checkoutCmd.Stderr = os.Stderr
	}
	if err := checkoutCmd.Run(); err != nil {
		// Try to checkout as a branch from origin
		checkoutCmd = exec.Command("git", "-C", repoPath, "checkout", "-b", ref, "origin/"+ref)
		if err := checkoutCmd.Run(); err != nil {
			return fmt.Errorf("git checkout failed: %v", err)
		}
	}

	pullCmd := exec.Command("git", "-C", repoPath, "pull", "origin", ref)
	if *verbose {
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
	}
	if err := pullCmd.Run(); err != nil {
		// Ignore pull errors as we might be in a detached HEAD state
		log.Printf("Warning: git pull failed (might be detached HEAD): %v", err)
	}

	return nil
}

func getSourceCode(repoPath string, req RepoRequest) (string, error) {
	args := []string{}
	
	// Set verbose flag if needed
	if *verbose {
		args = append(args, "--verbose")
	}
	
	// Set no-xml-tags flag if requested
	if req.NoXML {
		args = append(args, "--no-xml-tags")
	}
	
	// Add repo path
	args = append(args, repoPath)
	
	// Add paths to include
	for _, path := range req.Paths {
		args = append(args, path)
	}
	
	// Add paths to exclude with gitignore syntax
	for _, exclude := range req.Excludes {
		args = append(args, "!"+exclude)
	}
	
	// Run ctx-src command
	cmd := exec.Command(*ctxSrcPath, args...)
	if *verbose {
		cmd.Stderr = os.Stderr
		log.Printf("Running command: %s %v", *ctxSrcPath, args)
	}
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ctx-src failed: %v", err)
	}
	
	return string(output), nil
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	// Calculate metrics
	uptime := time.Since(serverStartTime).String()
	reqCount := requestCount.Load()
	succCount := successCount.Load()
	errCount := errorCount.Load()
	
	// Calculate success rate
	var successRate float64
	if reqCount > 0 {
		successRate = float64(succCount) / float64(reqCount) * 100
	}
	
	// Calculate average processing time
	var avgProcTime float64
	if reqCount > 0 {
		avgProcTime = float64(totalProcessingTime.Load()) / float64(reqCount)
	}
	
	// Calculate cache hit rate
	cHits := cacheHits.Load()
	cMisses := cacheMisses.Load()
	var cacheHitRate float64
	if cHits+cMisses > 0 {
		cacheHitRate = float64(cHits) / float64(cHits+cMisses) * 100
	}
	
	// Count current goroutines
	currentGoroutines := runtime.NumGoroutine()
	
	// Get current concurrent git operations (semaphore buffer usage)
	gitOperations := *maxConcurrent - len(semaphore)
	
	// Prepare metrics response
	metrics := MetricsResponse{
		Uptime:            uptime,
		RequestCount:      reqCount,
		SuccessCount:      succCount,
		ErrorCount:        errCount,
		SuccessRate:       successRate,
		AvgProcessingTime: avgProcTime,
		CacheHits:         cHits,
		CacheMisses:       cMisses,
		CacheHitRate:      cacheHitRate,
		TotalBytesServed:  totalBytesServed.Load(),
		CurrentGoroutines: currentGoroutines,
		NumCPU:            runtime.NumCPU(),
		GoVersion:         runtime.Version(),
		GitOperations:     gitOperations,
	}
	
	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}