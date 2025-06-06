package main

import (
	"archive/tar"
	"compress/gzip"
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

const (
	goDownloadURL = "https://go.dev/dl/"
	userAgent     = "go-cov/1.0"
)

func main() {
	if len(os.Args) < 1 {
		log.Fatal("Usage: go install go.tmc.dev/go-cov@go1.24.3")
	}

	// Extract version from module path (similar to golang.org/dl pattern)
	version := extractVersionFromPath()
	if version == "" {
		log.Fatal("No Go version specified. Use: go install go.tmc.dev/go-cov@go1.24.3")
	}

	fmt.Printf("Installing coverage-enabled Go %s...\n", version)

	// Download and install Go
	if err := downloadAndInstallGo(version); err != nil {
		log.Fatalf("Failed to install Go %s: %v", version, err)
	}

	// Build coverage-enabled version
	if err := buildCoverageEnabledGo(version); err != nil {
		log.Fatalf("Failed to build coverage-enabled Go %s: %v", version, err)
	}

	fmt.Printf("Coverage-enabled Go %s installed successfully!\n", version)
	fmt.Printf("Use: export PATH=$HOME/sdk/%s-cov/bin:$PATH\n", version)
}

func extractVersionFromPath() string {
	// In a real installation, this would be passed via build flags
	// For now, we'll try to detect from environment or use a default
	if version := os.Getenv("GO_COV_VERSION"); version != "" {
		return version
	}
	
	// Default for demonstration
	return "go1.24.3"
}

func downloadAndInstallGo(version string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	
	filename := fmt.Sprintf("%s.%s-%s.tar.gz", version, goos, goarch)
	url := fmt.Sprintf("%s%s", goDownloadURL, filename)
	
	fmt.Printf("Downloading %s...\n", url)
	
	// Create SDK directory
	sdkDir := filepath.Join(os.Getenv("HOME"), "sdk")
	if err := os.MkdirAll(sdkDir, 0755); err != nil {
		return fmt.Errorf("failed to create SDK directory: %w", err)
	}
	
	// Download Go tarball
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download Go: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Go: HTTP %d", resp.StatusCode)
	}
	
	// Extract to temporary directory first
	tempDir := filepath.Join(sdkDir, version+"-temp")
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to remove temp directory: %w", err)
	}
	
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	// Extract tarball
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()
	
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}
		
		// Remove "go/" prefix from paths
		path := strings.TrimPrefix(header.Name, "go/")
		if path == header.Name {
			continue // Skip if no "go/" prefix
		}
		
		target := filepath.Join(tempDir, path)
		
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			f.Close()
		}
	}
	
	// Move to final location
	finalDir := filepath.Join(sdkDir, version)
	if err := os.RemoveAll(finalDir); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}
	
	if err := os.Rename(tempDir, finalDir); err != nil {
		return fmt.Errorf("failed to move to final location: %w", err)
	}
	
	fmt.Printf("Go %s extracted to %s\n", version, finalDir)
	return nil
}

func buildCoverageEnabledGo(version string) error {
	sdkDir := filepath.Join(os.Getenv("HOME"), "sdk")
	goDir := filepath.Join(sdkDir, version)
	covDir := filepath.Join(sdkDir, version+"-cov")
	
	fmt.Printf("Building coverage-enabled Go in %s...\n", covDir)
	
	// Copy Go installation to coverage directory
	if err := copyDir(goDir, covDir); err != nil {
		return fmt.Errorf("failed to copy Go installation: %w", err)
	}
	
	// Build Go toolchain with coverage support
	srcDir := filepath.Join(covDir, "src")
	
	// Set environment for building
	env := []string{
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
		"GOROOT=" + covDir,
		"PATH=" + filepath.Join(covDir, "bin") + ":" + os.Getenv("PATH"),
	}
	
	// Add coverage build flags
	buildScript := `#!/bin/bash
set -e
cd ` + srcDir + `
export GOROOT_BOOTSTRAP=` + goDir + `
export GOROOT=` + covDir + `
export GOEXPERIMENT=coverageredesign
./make.bash
`
	
	scriptPath := filepath.Join(covDir, "build-coverage.sh")
	if err := os.WriteFile(scriptPath, []byte(buildScript), 0755); err != nil {
		return fmt.Errorf("failed to write build script: %w", err)
	}
	
	// Execute build script
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build coverage-enabled Go: %w", err)
	}
	
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		
		dstPath := filepath.Join(dst, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		
		return copyFile(path, dstPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	_, err = io.Copy(dstFile, srcFile)
	return err
}