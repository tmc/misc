package packager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BuildTarget represents a target platform for building
type BuildTarget struct {
	OS   string
	Arch string
	Ext  string // Extension for the binary (.exe for Windows)
}

// DefaultBuildTargets returns commonly used build targets
func DefaultBuildTargets() []BuildTarget {
	return []BuildTarget{
		{"linux", "amd64", ""},
		{"linux", "arm64", ""},
		{"darwin", "amd64", ""},
		{"darwin", "arm64", ""},
		{"windows", "amd64", ".exe"},
	}
}

// BuildBinary builds a Go binary for the specified target
func BuildBinary(pkgPath, outputDir, version string, target BuildTarget) (string, error) {
	binaryName := "omni"
	if target.OS == "windows" {
		binaryName += target.Ext
	}
	
	outputPath := filepath.Join(outputDir, "bin", target.OS+"_"+target.Arch, binaryName)
	
	// Create the output directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Set up command with cross-compilation environment variables
	cmd := exec.Command("go", "build", "-o", outputPath, pkgPath)
	cmd.Env = append(os.Environ(),
		"GOOS="+target.OS,
		"GOARCH="+target.Arch,
		"CGO_ENABLED=0",
	)
	
	// Run the build
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build failed: %w, output: %s", err, output)
	}
	
	return outputPath, nil
}

// BuildAllBinaries builds the Go binary for all specified targets
func BuildAllBinaries(pkgPath, outputDir, version string, targets []BuildTarget) (map[BuildTarget]string, error) {
	results := make(map[BuildTarget]string)
	
	for _, target := range targets {
		outputPath, err := BuildBinary(pkgPath, outputDir, version, target)
		if err != nil {
			return nil, fmt.Errorf("failed to build for %s/%s: %w", target.OS, target.Arch, err)
		}
		results[target] = outputPath
	}
	
	return results, nil
}

// GenerateChecksums creates checksums for all built binaries
func GenerateChecksums(binaries map[BuildTarget]string, outputDir string) (string, error) {
	checksumPath := filepath.Join(outputDir, "checksums.txt")
	checksumFile, err := os.Create(checksumPath)
	if err != nil {
		return "", fmt.Errorf("failed to create checksum file: %w", err)
	}
	defer checksumFile.Close()
	
	for target, path := range binaries {
		// Run sha256sum or equivalent
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("certutil", "-hashfile", path, "SHA256")
		} else {
			cmd = exec.Command("shasum", "-a", "256", path)
		}
		
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to generate checksum for %s/%s: %w", target.OS, target.Arch, err)
		}
		
		// Write to checksum file
		_, err = checksumFile.Write(output)
		if err != nil {
			return "", fmt.Errorf("failed to write checksum: %w", err)
		}
	}
	
	return checksumPath, nil
}