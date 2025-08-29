package chromeprofiles

import (
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	chromeErrors "github.com/tmc/misc/chrome-to-har/internal/errors"
	"github.com/tmc/misc/chrome-to-har/internal/secureio"
	"github.com/tmc/misc/chrome-to-har/internal/validation"
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
	dir, err := secureio.CreateSecureTempDir("chrome-to-har-")
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.FilePermissionError, "failed to create secure temporary directory")
	}
	pm.workDir = dir
	pm.logf("Created secure temporary working directory: %s", dir)
	return nil
}

func (pm *profileManager) Cleanup() error {
	if pm.workDir != "" {
		pm.logf("Cleaning up working directory: %s", pm.workDir)
		return secureio.SecureRemoveAll(pm.workDir)
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
		return nil, chromeErrors.WithContext(
			chromeErrors.FileError("read", pm.baseDir, err),
			"operation", "list_profiles",
		)
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
		return chromeErrors.New(chromeErrors.ProfileSetupError, "working directory not set up")
	}

	// Validate profile name for security
	if err := validation.ValidateProfileName(name); err != nil {
		return chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ProfileNotFoundError, "invalid profile name"),
			"profile", name,
		)
	}

	srcDir := filepath.Join(pm.baseDir, name)
	if !isValidProfile(srcDir) {
		return chromeErrors.WithContext(
			chromeErrors.New(chromeErrors.ProfileNotFoundError, "invalid profile: profile directory does not contain expected files"),
			"profile", name,
		)
	}

	dstDir := filepath.Join(pm.workDir, "Default")
	if err := os.MkdirAll(dstDir, secureio.SecureDirPerms); err != nil {
		return chromeErrors.WithContext(
			chromeErrors.FileError("create", dstDir, err),
			"profile", name,
		)
	}

	pm.logf("Copying profile from %s to %s", srcDir, dstDir)

	// Handle cookies with domain filtering
	if len(cookieDomains) > 0 {
		pm.logf("Filtering cookies for domains: %v", cookieDomains)
		if err := pm.CopyCookiesWithDomains(srcDir, dstDir, cookieDomains); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to copy cookies with domain filtering"),
				"profile", name,
			)
		}
	} else {
		if err := copyFile(filepath.Join(srcDir, "Cookies"), filepath.Join(dstDir, "Cookies")); err != nil {
			if !os.IsNotExist(err) {
				return chromeErrors.WithContext(
					chromeErrors.FileError("copy", filepath.Join(srcDir, "Cookies"), err),
					"profile", name,
				)
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
		return chromeErrors.WithContext(
			chromeErrors.FileError("write", filepath.Join(pm.workDir, "Local State"), err),
			"profile", name,
		)
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
		return chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to open source cookies database"),
			"database", srcDB,
		)
	}
	defer src.Close()

	// Create destination database
	if err := copyFile(srcDB, dstDB); err != nil {
		return chromeErrors.WithContext(
			chromeErrors.FileError("copy", srcDB, err),
			"operation", "copy_cookies_database",
		)
	}

	dst, err := sql.Open("sqlite", dstDB)
	if err != nil {
		return chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to open destination cookies database"),
			"database", dstDB,
		)
	}
	defer dst.Close()

	// Begin transaction
	tx, err := dst.Begin()
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to begin database transaction")
	}
	defer tx.Rollback()

	// Delete cookies that don't match domains
	var whereClause strings.Builder
	whereClause.WriteString("host_key NOT LIKE '%")
	whereClause.WriteString(strings.Join(domains, "%' AND host_key NOT LIKE '%"))
	whereClause.WriteString("%'")

	_, err = tx.Exec("DELETE FROM cookies WHERE " + whereClause.String())
	if err != nil {
		return chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to filter cookies by domain"),
			"domains", domains,
		)
	}

	if err := tx.Commit(); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to commit database changes")
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
		return "", chromeErrors.WithContext(
			chromeErrors.New(chromeErrors.ConfigurationError, "unsupported operating system"),
			"os", runtime.GOOS,
		)
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
