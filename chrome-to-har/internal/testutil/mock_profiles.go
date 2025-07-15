package testutil

import (
	"fmt"
	"log"
)

// MockProfileManager provides a test implementation of profile management
type MockProfileManager struct {
	Profiles    []string
	Verbose     bool
	WorkDirPath string // Export this field for test access
}

// NewMockProfileManager creates a new mock profile manager
func NewMockProfileManager() *MockProfileManager {
	return &MockProfileManager{
		Profiles:    []string{"Test Profile 1", "Test Profile 2"},
		WorkDirPath: "/tmp/mock-chrome-profile",
	}
}

// ListProfiles returns the mock list of profiles
func (m *MockProfileManager) ListProfiles() ([]string, error) {
	if m.Verbose {
		for _, p := range m.Profiles {
			log.Printf("Found valid profile: %s", p)
		}
	}
	return m.Profiles, nil
}

// SetupWorkdir simulates setting up a working directory
func (m *MockProfileManager) SetupWorkdir() error {
	if m.Verbose {
		log.Printf("Setting up mock working directory: %s", m.WorkDirPath)
	}
	return nil
}

// Cleanup simulates cleanup operations
func (m *MockProfileManager) Cleanup() error {
	if m.Verbose {
		log.Printf("Cleaning up mock working directory: %s", m.WorkDirPath)
	}
	return nil
}

// CopyProfile simulates copying a profile
func (m *MockProfileManager) CopyProfile(name string, cookieDomains []string) error {
	found := false
	for _, p := range m.Profiles {
		if p == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("profile not found: %s", name)
	}
	if m.Verbose {
		log.Printf("Copying mock profile %s with domains: %v", name, cookieDomains)
	}
	return nil
}

// WorkDir returns the mock working directory
func (m *MockProfileManager) WorkDir() string {
	return m.WorkDirPath
}
