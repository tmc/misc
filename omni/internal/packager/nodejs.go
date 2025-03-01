package packager

import (
	"fmt"
	"os"
	"path/filepath"
)

// NodeJSPackageOptions contains options specific to Node.js packages
type NodeJSPackageOptions struct {
	Name        string
	Version     string
	Description string
	Author      string
	Email       string
	URL         string
	Platforms   []string
}

// BuildNodeJSPackage builds a Node.js package for the specified options
func BuildNodeJSPackage(opts NodeJSPackageOptions, outputDir string) (string, error) {
	// Create package directory structure
	pkgDir := filepath.Join(outputDir, "nodejs", opts.Name)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create package directory: %w", err)
	}

	// Create package.json
	if err := createPackageJSON(pkgDir, opts); err != nil {
		return "", fmt.Errorf("failed to create package.json: %w", err)
	}

	// Create index.js
	if err := createIndexJS(pkgDir, opts); err != nil {
		return "", fmt.Errorf("failed to create index.js: %w", err)
	}

	// Create bin directory for the binary
	binDir := filepath.Join(pkgDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	// TODO: Copy actual binaries for each platform

	// TODO: Create the actual npm package (npm pack)
	tarballPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.tgz", opts.Name, opts.Version))

	return tarballPath, nil
}

// Templates are now in the internal/template directory

// Helper functions to create package files

func createPackageJSON(pkgDir string, opts NodeJSPackageOptions) error {
	// Read template from file
	tmplPath := filepath.Join("internal", "template", "nodejs", "package.json.tmpl")
	return createFileFromTemplateFile(filepath.Join(pkgDir, "package.json"), tmplPath, opts)
}

func createIndexJS(pkgDir string, opts NodeJSPackageOptions) error {
	// Read template from file
	tmplPath := filepath.Join("internal", "template", "nodejs", "index.js.tmpl")
	return createFileFromTemplateFile(filepath.Join(pkgDir, "index.js"), tmplPath, opts)
}