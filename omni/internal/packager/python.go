package packager

import (
	"fmt"
	"os"
	"path/filepath"
)

// PythonPackageOptions contains options specific to Python packages
type PythonPackageOptions struct {
	Name        string
	Version     string
	Description string
	Author      string
	AuthorEmail string
	URL         string
	Platforms   []string
}

// BuildPythonPackage builds a Python package for the specified options
func BuildPythonPackage(opts PythonPackageOptions, outputDir string) (string, error) {
	// Create package directory structure
	pkgDir := filepath.Join(outputDir, "python", opts.Name)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create package directory: %w", err)
	}

	// Create setup.py
	if err := createSetupPy(pkgDir, opts); err != nil {
		return "", fmt.Errorf("failed to create setup.py: %w", err)
	}

	// Create pyproject.toml
	if err := createPyprojectToml(pkgDir, opts); err != nil {
		return "", fmt.Errorf("failed to create pyproject.toml: %w", err)
	}

	// Create __init__.py
	if err := createInitPy(pkgDir, opts); err != nil {
		return "", fmt.Errorf("failed to create __init__.py: %w", err)
	}

	// TODO: Build and package the actual wheel
	wheelPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s-py3-none-any.whl", opts.Name, opts.Version))

	return wheelPath, nil
}

// Templates are now in the internal/template directory

// Helper functions to create package files

func createSetupPy(pkgDir string, opts PythonPackageOptions) error {
	// Read template from file
	tmplPath := filepath.Join("internal", "template", "python", "setup.py.tmpl")
	return createFileFromTemplateFile(filepath.Join(pkgDir, "setup.py"), tmplPath, opts)
}

func createPyprojectToml(pkgDir string, opts PythonPackageOptions) error {
	// Read template from file
	tmplPath := filepath.Join("internal", "template", "python", "pyproject.toml.tmpl")
	return createFileFromTemplateFile(filepath.Join(pkgDir, "pyproject.toml"), tmplPath, opts)
}

func createInitPy(pkgDir string, opts PythonPackageOptions) error {
	pkgCodeDir := filepath.Join(pkgDir, opts.Name)
	if err := os.MkdirAll(pkgCodeDir, 0755); err != nil {
		return err
	}
	
	// Read template from file
	tmplPath := filepath.Join("internal", "template", "python", "__init__.py.tmpl")
	return createFileFromTemplateFile(filepath.Join(pkgCodeDir, "__init__.py"), tmplPath, opts)
}