package discovery

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// BrowserCandidate represents a discovered browser installation
type BrowserCandidate struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Priority int       `json:"priority"` // lower is better
	LastUsed time.Time `json:"lastUsed"`
	Version  string    `json:"version,omitempty"`
}

// BrowserType represents different browser types we can discover
type BrowserType int

const (
	Chrome BrowserType = iota
	Chromium
	Brave
	Edge
	Opera
	Vivaldi
	Arc
)

var browserTypeNames = map[BrowserType]string{
	Chrome:   "Google Chrome",
	Chromium: "Chromium",
	Brave:    "Brave Browser",
	Edge:     "Microsoft Edge",
	Opera:    "Opera",
	Vivaldi:  "Vivaldi",
	Arc:      "Arc Browser",
}

// String returns the human-readable name for the browser type
func (bt BrowserType) String() string {
	if name, ok := browserTypeNames[bt]; ok {
		return name
	}
	return "Unknown Browser"
}

// BrowserInfo holds metadata about a browser installation
type BrowserInfo struct {
	Type     BrowserType
	Name     string
	Paths    []string
	Priority int
}

// GetSupportedBrowsers returns information about browsers we can discover
func GetSupportedBrowsers() []BrowserInfo {
	browsers := []BrowserInfo{
		{
			Type:     Brave,
			Name:     "Brave Browser",
			Priority: 1, // Highest priority
		},
		{
			Type:     Chrome,
			Name:     "Google Chrome",
			Priority: 2,
		},
		{
			Type:     Chromium,
			Name:     "Chromium",
			Priority: 3,
		},
		{
			Type:     Edge,
			Name:     "Microsoft Edge",
			Priority: 4,
		},
		{
			Type:     Arc,
			Name:     "Arc Browser",
			Priority: 5,
		},
		{
			Type:     Vivaldi,
			Name:     "Vivaldi",
			Priority: 6,
		},
		{
			Type:     Opera,
			Name:     "Opera",
			Priority: 7,
		},
	}

	// Add platform-specific paths
	for i := range browsers {
		browsers[i].Paths = getPlatformPaths(browsers[i].Type)
	}

	return browsers
}

// getPlatformPaths returns platform-specific installation paths for a browser type
func getPlatformPaths(browserType BrowserType) []string {
	switch runtime.GOOS {
	case "darwin":
		return getDarwinPaths(browserType)
	case "linux":
		return getLinuxPaths(browserType)
	case "windows":
		return getWindowsPaths(browserType)
	default:
		return []string{}
	}
}

func getDarwinPaths(browserType BrowserType) []string {
	switch browserType {
	case Chrome:
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
	case Chromium:
		return []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case Brave:
		return []string{
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	case Edge:
		return []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case Arc:
		return []string{
			"/Applications/Arc.app/Contents/MacOS/Arc",
		}
	case Vivaldi:
		return []string{
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
		}
	case Opera:
		return []string{
			"/Applications/Opera.app/Contents/MacOS/Opera",
		}
	default:
		return []string{}
	}
}

func getLinuxPaths(browserType BrowserType) []string {
	switch browserType {
	case Chrome:
		return []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-beta",
			"/usr/bin/google-chrome-unstable",
		}
	case Chromium:
		return []string{
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/snap/bin/chromium",
		}
	case Brave:
		return []string{
			"/usr/bin/brave-browser",
			"/usr/bin/brave",
			"/snap/bin/brave",
		}
	case Edge:
		return []string{
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
		}
	case Opera:
		return []string{
			"/usr/bin/opera",
			"/usr/bin/opera-stable",
		}
	case Vivaldi:
		return []string{
			"/usr/bin/vivaldi",
			"/usr/bin/vivaldi-stable",
		}
	default:
		return []string{}
	}
}

func getWindowsPaths(browserType BrowserType) []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("PROGRAMFILES")
	programFilesx86 := os.Getenv("PROGRAMFILES(X86)")

	switch browserType {
	case Chrome:
		return []string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesx86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
		}
	case Chromium:
		return []string{
			filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
			filepath.Join(programFilesx86, "Chromium", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Chromium", "Application", "chrome.exe"),
		}
	case Brave:
		return []string{
			filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(programFilesx86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
	case Edge:
		return []string{
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesx86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
		}
	case Opera:
		return []string{
			filepath.Join(programFiles, "Opera", "opera.exe"),
			filepath.Join(programFilesx86, "Opera", "opera.exe"),
			filepath.Join(localAppData, "Programs", "Opera", "opera.exe"),
		}
	case Vivaldi:
		return []string{
			filepath.Join(programFiles, "Vivaldi", "Application", "vivaldi.exe"),
			filepath.Join(programFilesx86, "Vivaldi", "Application", "vivaldi.exe"),
			filepath.Join(localAppData, "Vivaldi", "Application", "vivaldi.exe"),
		}
	default:
		return []string{}
	}
}

// DiscoverBrowsers finds all available Chromium-based browsers on the system
func DiscoverBrowsers() []BrowserCandidate {
	var candidates []BrowserCandidate

	browsers := GetSupportedBrowsers()

	for _, browser := range browsers {
		for _, path := range browser.Paths {
			if isExecutable(path) {
				candidate := BrowserCandidate{
					Path:     path,
					Name:     browser.Name,
					Priority: browser.Priority,
					LastUsed: getLastUsedTime(path, browser.Name),
				}

				// Try to get version
				if version := getBrowserVersion(path); version != "" {
					candidate.Version = version
				}

				candidates = append(candidates, candidate)
			}
		}
	}

	// Also check PATH for browsers
	pathCandidates := discoverFromPATH()
	candidates = append(candidates, pathCandidates...)

	// Sort by priority (lower is better), then by last used time (newer is better)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].LastUsed.After(candidates[j].LastUsed)
		}
		return candidates[i].Priority < candidates[j].Priority
	})

	// Remove duplicates (keep the highest priority one)
	seen := make(map[string]bool)
	var unique []BrowserCandidate
	for _, candidate := range candidates {
		if !seen[candidate.Path] {
			seen[candidate.Path] = true
			unique = append(unique, candidate)
		}
	}

	return unique
}

// FindBestBrowser returns the path to the best available browser, or empty string if none found
func FindBestBrowser() string {
	// Check environment variable first
	if envPath := os.Getenv("CHROME_EXECUTABLE_PATH"); envPath != "" {
		if isExecutable(envPath) {
			return envPath
		}
	}

	candidates := DiscoverBrowsers()
	if len(candidates) > 0 {
		return candidates[0].Path
	}

	return ""
}

// isExecutable checks if a file exists and is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Windows, check if it's a file
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}

	// On Unix-like systems, check execute permission
	return info.Mode()&0111 != 0
}

// discoverFromPATH searches for browsers in the system PATH
func discoverFromPATH() []BrowserCandidate {
	var candidates []BrowserCandidate

	commands := map[string]BrowserInfo{
		"brave-browser":         {Type: Brave, Name: "Brave Browser", Priority: 1},
		"brave":                 {Type: Brave, Name: "Brave Browser", Priority: 1},
		"google-chrome":         {Type: Chrome, Name: "Google Chrome", Priority: 2},
		"google-chrome-stable":  {Type: Chrome, Name: "Google Chrome", Priority: 2},
		"chromium-browser":      {Type: Chromium, Name: "Chromium", Priority: 3},
		"chromium":              {Type: Chromium, Name: "Chromium", Priority: 3},
		"microsoft-edge":        {Type: Edge, Name: "Microsoft Edge", Priority: 4},
		"msedge":                {Type: Edge, Name: "Microsoft Edge", Priority: 4},
		"vivaldi":               {Type: Vivaldi, Name: "Vivaldi", Priority: 6},
		"opera":                 {Type: Opera, Name: "Opera", Priority: 7},
	}

	for cmd, info := range commands {
		if path, err := exec.LookPath(cmd); err == nil {
			candidate := BrowserCandidate{
				Path:     path,
				Name:     info.Name,
				Priority: info.Priority,
				LastUsed: time.Time{}, // Unknown for PATH commands
			}
			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

// getLastUsedTime attempts to determine when a browser was last used
func getLastUsedTime(browserPath, browserName string) time.Time {
	// For macOS, check profile directories
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return time.Time{}
		}

		profilePaths := map[string][]string{
			"Google Chrome":  {filepath.Join(home, "Library", "Application Support", "Google", "Chrome")},
			"Brave Browser":  {filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")},
			"Microsoft Edge": {filepath.Join(home, "Library", "Application Support", "Microsoft Edge")},
			"Chromium":       {filepath.Join(home, "Library", "Application Support", "Chromium")},
			"Vivaldi":        {filepath.Join(home, "Library", "Application Support", "Vivaldi")},
			"Opera":          {filepath.Join(home, "Library", "Application Support", "com.operasoftware.Opera")},
		}

		if paths, ok := profilePaths[browserName]; ok {
			for _, profilePath := range paths {
				if info, err := os.Stat(filepath.Join(profilePath, "Default", "Preferences")); err == nil {
					return info.ModTime()
				}
				if info, err := os.Stat(filepath.Join(profilePath, "Preferences")); err == nil {
					return info.ModTime()
				}
			}
		}
	}

	// Fallback to executable modification time
	if info, err := os.Stat(browserPath); err == nil {
		return info.ModTime()
	}

	return time.Time{}
}

// getBrowserVersion attempts to get the version of a browser
func getBrowserVersion(browserPath string) string {
	// Try running with --version flag
	cmd := exec.Command(browserPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(output))
	// Extract just the version number
	parts := strings.Fields(version)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return version
}