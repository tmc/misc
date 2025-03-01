package publisher

import (
	"fmt"
)

// Publisher defines the interface for package publishers
type Publisher interface {
	// Publish publishes a package
	Publish(packagePath string) error
	// Verify verifies that a package was published successfully
	Verify(packageName, version string) error
}

// PublishOptions contains options for publishing packages
type PublishOptions struct {
	Version       string
	Packages      []string
	UseTestPyPI   bool
	GitHubOwner   string
	GitHubRepo    string
	SkipPyPI      bool
	SkipNPM       bool
	SkipGitHub    bool
	SkipVerify    bool
	ReleaseNotes  string
}

// PublishPackages publishes packages to their respective repositories
func PublishPackages(opts PublishOptions) error {
	// Publish to PyPI
	if !opts.SkipPyPI {
		if err := publishToPyPI(opts); err != nil {
			return fmt.Errorf("PyPI publishing failed: %w", err)
		}
	}

	// Publish to npm
	if !opts.SkipNPM {
		if err := publishToNPM(opts); err != nil {
			return fmt.Errorf("npm publishing failed: %w", err)
		}
	}

	// Publish to GitHub
	if !opts.SkipGitHub {
		if err := publishToGitHub(opts); err != nil {
			return fmt.Errorf("GitHub publishing failed: %w", err)
		}
	}

	return nil
}

// Helper functions for publishing to each platform

func publishToPyPI(opts PublishOptions) error {
	// Find Python wheel package
	var pythonPackage string
	for _, pkg := range opts.Packages {
		if hasExtension(pkg, ".whl") {
			pythonPackage = pkg
			break
		}
	}

	if pythonPackage == "" {
		return fmt.Errorf("no Python wheel package found")
	}

	// Create PyPI publisher
	pypi, err := NewPyPIPublisher(opts.UseTestPyPI)
	if err != nil {
		return err
	}

	// Publish
	if err := pypi.Publish(pythonPackage); err != nil {
		return err
	}

	// Verify if not skipped
	if !opts.SkipVerify {
		if err := pypi.Verify("omni", opts.Version); err != nil {
			return fmt.Errorf("PyPI verification failed: %w", err)
		}
	}

	return nil
}

func publishToNPM(opts PublishOptions) error {
	// Find npm package
	var npmPackage string
	for _, pkg := range opts.Packages {
		if hasExtension(pkg, ".tgz") {
			npmPackage = pkg
			break
		}
	}

	if npmPackage == "" {
		return fmt.Errorf("no npm package found")
	}

	// Create NPM publisher
	npm, err := NewNPMPublisher()
	if err != nil {
		return err
	}

	// Publish
	if err := npm.Publish(npmPackage); err != nil {
		return err
	}

	// Verify if not skipped
	if !opts.SkipVerify {
		if err := npm.Verify("omni", opts.Version); err != nil {
			return fmt.Errorf("npm verification failed: %w", err)
		}
	}

	return nil
}

func publishToGitHub(opts PublishOptions) error {
	// Create GitHub publisher
	github, err := NewGitHubPublisher(opts.GitHubOwner, opts.GitHubRepo)
	if err != nil {
		return err
	}

	// Create release
	uploadURL, err := github.CreateRelease(opts.Version, opts.ReleaseNotes)
	if err != nil {
		return fmt.Errorf("failed to create GitHub release: %w", err)
	}

	// Upload all packages as assets
	for _, pkg := range opts.Packages {
		if err := github.UploadAsset(uploadURL, pkg); err != nil {
			return fmt.Errorf("failed to upload asset %s: %w", pkg, err)
		}
	}

	// Upload checksums file if exists
	// TODO: Implement checksum file upload

	// Verify if not skipped
	if !opts.SkipVerify {
		if err := github.Verify(opts.Version); err != nil {
			return fmt.Errorf("GitHub verification failed: %w", err)
		}
	}

	return nil
}

// hasExtension checks if a filename has the given extension
func hasExtension(filename, ext string) bool {
	lenName := len(filename)
	lenExt := len(ext)
	
	if lenName <= lenExt {
		return false
	}
	
	return filename[lenName-lenExt:] == ext
}