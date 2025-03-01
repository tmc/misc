package publisher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NPMPublisher is responsible for publishing packages to npm
type NPMPublisher struct {
	Token string
}

// NewNPMPublisher creates a new npm publisher
func NewNPMPublisher() (*NPMPublisher, error) {
	token := os.Getenv("OMNI_NPM_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("OMNI_NPM_TOKEN environment variable not set")
	}

	return &NPMPublisher{
		Token: token,
	}, nil
}

// Publish publishes an npm package to the npm registry
func (p *NPMPublisher) Publish(packagePath string) error {
	// Create temporary .npmrc file with auth token
	npmrc, err := p.createNPMRC()
	if err != nil {
		return fmt.Errorf("failed to create npm credentials: %w", err)
	}
	defer os.Remove(npmrc)

	// Use the npm CLI to publish
	cmd := exec.Command("npm", "publish", packagePath, "--userconfig", npmrc)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to publish to npm: %w, output: %s", err, output)
	}

	return nil
}

// createNPMRC creates a temporary .npmrc file with authentication token
func (p *NPMPublisher) createNPMRC() (string, error) {
	tempDir := os.TempDir()
	npmrcPath := filepath.Join(tempDir, fmt.Sprintf("npmrc-%d", os.Getpid()))

	// Create npmrc content with the token
	content := fmt.Sprintf("//registry.npmjs.org/:_authToken=%s\n", p.Token)

	if err := os.WriteFile(npmrcPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write .npmrc: %w", err)
	}

	return npmrcPath, nil
}

// Verify verifies that a package was successfully published
func (p *NPMPublisher) Verify(packageName, version string) error {
	// Use npm view to check if the package exists with the correct version
	cmd := exec.Command("npm", "view", fmt.Sprintf("%s@%s", packageName, version), "version")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package verification failed: %w, output: %s", err, output)
	}

	// Check if the output matches the expected version
	outputStr := string(output)
	if outputStr != version+"\n" {
		return fmt.Errorf("package version mismatch: expected %s, got %s", version, outputStr)
	}

	return nil
}