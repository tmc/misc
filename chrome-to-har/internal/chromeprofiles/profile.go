package chromeprofiles

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

type profileManager struct {
	baseDir string
	workDir string
	verbose bool
}

// NewProfileManager creates a new profile manager with the given options
func NewProfileManager(opts ...Option) (*profileManager, error) {
	baseDir, err := getChromeProfileDir()
	if err != nil {
		return nil, err
	}
	pm := &profileManager{
		baseDir: baseDir,
	}
	for _, opt := range opts {
		opt(pm)
	}
	return pm, nil
}

func (pm *profileManager) logf(format string, args ...interface{}) {
	if pm.verbose {
		log.Printf(format, args...)
	}
}

func (pm *profileManager) SetupWorkdir() error {
	dir, err := os.MkdirTemp("", "chrome-to-har-*")
	if err != nil {
		return errors.Wrap(err, "creating temp directory")
	}
	pm.workDir = dir
	pm.logf("Created temporary working directory: %s", dir)
	return nil
}

func (pm *profileManager) Cleanup() error {
	if pm.workDir != "" {
		pm.logf("Cleaning up working directory: %s", pm.workDir)
		return os.RemoveAll(pm.workDir)
	}
	return nil
}

// WorkDir returns the current working directory
func (pm *profileManager) WorkDir() string {
	return pm.workDir
}

func (pm *profileManager) ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		return nil, errors.Wrap(err, "reading profile directory")
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			profilePath := filepath.Join(pm.baseDir, entry.Name())
			if isValidProfile(profilePath) {
				profiles = append(profiles, entry.Name())
				pm.logf("Found valid profile: %s", entry.Name())
			}
		}
	}
	return profiles, nil
}

func (pm *profileManager) CopyProfile(name string, cookieDomains []string) error {
	if pm.workDir == "" {
		return fmt.Errorf("working directory not set up")
	}

	srcDir := filepath.Join(pm.baseDir, name)
	if !isValidProfile(srcDir) {
		return fmt.Errorf("invalid profile: %s", name)
	}

	dstDir := filepath.Join(pm.workDir, "Default")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return errors.Wrap(err, "creating profile directory")
	}

	pm.logf("Copying profile from %s to %s", srcDir, dstDir)

	// Handle cookies with domain filtering
	if len(cookieDomains) > 0 {
		pm.logf("Filtering cookies for domains: %v", cookieDomains)
		if err := pm.CopyCookiesWithDomains(srcDir, dstDir, cookieDomains); err != nil {
			return errors.Wrap(err, "copying cookies")
		}
	} else {
		if err := copyFile(filepath.Join(srcDir, "Cookies"), filepath.Join(dstDir, "Cookies")); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrap(err, "copying cookies")
			}
		}
	}

	// Essential profile components
	essentials := map[string]bool{
		"Login Data":               false,
		"Web Data":                 false,
		"Preferences":              false,
		"Bookmarks":                false,
		"History":                  false,
		"Favicons":                 false,
		"Network Action Predictor": false,
		"Network Persistent State": false,
		"Extension Cookies":        false,
		"Local Storage":            true,
		"IndexedDB":                true,
		"Session Storage":          true,
	}

	for name, isDir := range essentials {
		src := filepath.Join(srcDir, name)
		dst := filepath.Join(dstDir, name)

		if isDir {
			if err := copyDir(src, dst); err != nil {
				if !os.IsNotExist(err) {
					pm.logf("Warning: error copying directory %s: %v", name, err)
				}
			} else {
				pm.logf("Copied directory: %s", name)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				if !os.IsNotExist(err) {
					pm.logf("Warning: error copying file %s: %v", name, err)
				}
			} else {
				pm.logf("Copied file: %s", name)
			}
		}
	}

	// Create minimal Local State file
	localState := `{"os_crypt":{"encrypted_key":""}}`
	if err := os.WriteFile(filepath.Join(pm.workDir, "Local State"), []byte(localState), 0644); err != nil {
		return errors.Wrap(err, "writing local state")
	}
	pm.logf("Created Local State file")

	return nil
}

func (pm *profileManager) CopyCookiesWithDomains(srcDir, dstDir string, domains []string) error {
	srcDB := filepath.Join(srcDir, "Cookies")
	dstDB := filepath.Join(dstDir, "Cookies")

	// Open source database
	src, err := sql.Open("sqlite", srcDB+"?mode=ro")
	if err != nil {
		return errors.Wrap(err, "opening source cookies database")
	}
	defer src.Close()

	// Create destination database
	if err := copyFile(srcDB, dstDB); err != nil {
		return errors.Wrap(err, "creating initial cookies database")
	}

	dst, err := sql.Open("sqlite", dstDB)
	if err != nil {
		return errors.Wrap(err, "opening destination cookies database")
	}
	defer dst.Close()

	// Begin transaction
	tx, err := dst.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning transaction")
	}
	defer tx.Rollback()

	// Delete cookies that don't match domains
	var whereClause strings.Builder
	whereClause.WriteString("host_key NOT LIKE '%")
	whereClause.WriteString(strings.Join(domains, "%' AND host_key NOT LIKE '%"))
	whereClause.WriteString("%'")

	_, err = tx.Exec("DELETE FROM cookies WHERE " + whereClause.String())
	if err != nil {
		return errors.Wrap(err, "filtering cookies")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "committing changes")
	}

	pm.logf("Copied and filtered cookies for domains: %v", domains)
	return nil
}

func getChromeProfileDir() (string, error) {
	var baseDir string
	switch runtime.GOOS {
	case "windows":
		baseDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data")
	case "darwin":
		baseDir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Google", "Chrome")
	case "linux":
		baseDir = filepath.Join(os.Getenv("HOME"), ".config", "google-chrome")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	return baseDir, nil
}

func isValidProfile(dir string) bool {
	indicators := []string{"Preferences", "History", "Cookies"}
	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(dir, indicator)); err == nil {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	info, err := source.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, info.Mode())
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
