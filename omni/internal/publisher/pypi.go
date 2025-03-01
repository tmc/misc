package publisher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PyPIPublisher is responsible for publishing packages to PyPI
type PyPIPublisher struct {
	Token       string
	UseTestPyPI bool
}

// NewPyPIPublisher creates a new PyPI publisher
func NewPyPIPublisher(useTestPyPI bool) (*PyPIPublisher, error) {
	token := os.Getenv("OMNI_PYPI_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("OMNI_PYPI_TOKEN environment variable not set")
	}

	return &PyPIPublisher{
		Token:       token,
		UseTestPyPI: useTestPyPI,
	}, nil
}

// Publish publishes a Python wheel package to PyPI
func (p *PyPIPublisher) Publish(wheelPath string) error {
	// Create temporary pypirc file with credentials
	pypirc, err := p.createPyPIRC()
	if err != nil {
		return fmt.Errorf("failed to create PyPI credentials: %w", err)
	}
	defer os.Remove(pypirc)

	// Determine repository URL
	repoURL := "https://upload.pypi.org/legacy/"
	if p.UseTestPyPI {
		repoURL = "https://test.pypi.org/legacy/"
	}

	// Upload using twine
	cmd := exec.Command(
		"twine", "upload",
		"--repository-url", repoURL,
		"--config-file", pypirc,
		wheelPath,
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to upload to PyPI: %w, output: %s", err, output)
	}

	return nil
}

// createPyPIRC creates a temporary .pypirc file with authentication info
func (p *PyPIPublisher) createPyPIRC() (string, error) {
	tempDir := os.TempDir()
	pypircPath := filepath.Join(tempDir, fmt.Sprintf("pypirc-%d", os.Getpid()))

	content := fmt.Sprintf(`[distutils]
index-servers =
    pypi

[pypi]
repository = %s
username = __token__
password = %s
`,
		func() string {
			if p.UseTestPyPI {
				return "https://test.pypi.org/legacy/"
			}
			return "https://upload.pypi.org/legacy/"
		}(),
		p.Token,
	)

	if err := os.WriteFile(pypircPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write .pypirc: %w", err)
	}

	return pypircPath, nil
}

// Verify verifies that a package was successfully published
func (p *PyPIPublisher) Verify(packageName, version string) error {
	// Use pip search or PyPI API to verify package is available
	baseURL := "https://pypi.org/pypi"
	if p.UseTestPyPI {
		baseURL = "https://test.pypi.org/pypi"
	}

	// For simplicity, just check if the package can be found via pip
	cmd := exec.Command("pip", "install", "--dry-run", "--index-url", baseURL, 
		fmt.Sprintf("%s==%s", packageName, version))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package verification failed: %w, output: %s", err, output)
	}

	return nil
}