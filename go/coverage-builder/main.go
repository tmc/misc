// coverage-builder is a web service that generates Go version wrappers
// with coverage instrumentation enabled for any Go version.
//
// It dynamically discovers available Go versions from go.dev/dl
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*
var templates embed.FS

var (
	wrapperTemplate *template.Template
	versionRegex    = regexp.MustCompile(`^go(\d+)\.(\d+)(?:\.(\d+))?(?:-(alpha|beta|rc)(\d+))?$`)

	// Version cache
	versionCache     map[string]VersionInfo
	versionCacheMu   sync.RWMutex
	lastVersionFetch time.Time
)

type VersionInfo struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []File `json:"files"`
}

type File struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

type TemplateData struct {
	Version     string
	VersionInfo VersionInfo
}

func init() {
	var err error
	wrapperTemplate, err = template.ParseFS(templates, "templates/wrapper.go.tmpl")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	versionCache = make(map[string]VersionInfo)
}

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/versions", handleVersionsAPI)
	http.HandleFunc("/generate/", handleGenerateWrapper)
	http.HandleFunc("/refresh", handleRefreshVersions)
	http.HandleFunc("/health", handleHealth)

	// Fetch versions on startup
	go refreshVersionCache()

	port := "8080"
	log.Printf("Starting coverage builder server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func refreshVersionCache() {
	log.Println("Refreshing Go version cache...")

	resp, err := http.Get("https://go.dev/dl/?mode=json")
	if err != nil {
		log.Printf("Failed to fetch versions: %v", err)
		return
	}
	defer resp.Body.Close()

	var versions []VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		log.Printf("Failed to decode versions: %v", err)
		return
	}

	versionCacheMu.Lock()
	defer versionCacheMu.Unlock()

	// Clear and rebuild cache
	versionCache = make(map[string]VersionInfo)

	for _, v := range versions {
		// Store by version name
		versionCache[v.Version] = v

		// Also store stable versions without patch number
		if v.Stable && strings.Count(v.Version, ".") == 2 {
			// e.g., go1.21.5 -> go1.21
			parts := strings.Split(v.Version, ".")
			majorMinor := parts[0] + "." + parts[1]

			// Only store if this is the latest patch version
			if existing, ok := versionCache[majorMinor]; !ok || v.Version > existing.Version {
				versionCache[majorMinor] = v
			}
		}
	}

	lastVersionFetch = time.Now()
	log.Printf("Version cache refreshed with %d versions", len(versionCache))
}

func getVersionInfo(version string) (VersionInfo, bool) {
	versionCacheMu.RLock()
	defer versionCacheMu.RUnlock()

	// Check if cache is stale (older than 1 hour)
	if time.Since(lastVersionFetch) > time.Hour {
		go refreshVersionCache()
	}

	info, ok := versionCache[version]
	return info, ok
}

func findSourceFile(version VersionInfo) (string, string) {
	for _, file := range version.Files {
		if file.Kind == "source" {
			return fmt.Sprintf("https://go.dev/dl/%s", file.Filename), file.SHA256
		}
	}

	// Fallback to constructing URL
	return fmt.Sprintf("https://go.dev/dl/%s.src.tar.gz", version.Version), ""
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// Extract version from path if present
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" && versionRegex.MatchString(strings.TrimSuffix(path, "-cov")) {
		handleGenerateWrapper(w, r)
		return
	}

	indexHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Go Coverage Builder</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .version { margin: 10px 0; padding: 10px; background: #f4f4f4; border-radius: 5px; }
        .version a { text-decoration: none; color: #007d9c; }
        .stable { background: #e7f3ff; }
        pre { background: #f4f4f4; padding: 10px; overflow-x: auto; }
        .usage { background: #e7f3ff; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .refresh { float: right; padding: 5px 10px; background: #007d9c; color: white; text-decoration: none; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>Go Coverage Builder</h1>
    <a href="/refresh" class="refresh" onclick="this.innerText='Refreshing...'; fetch('/refresh').then(() => location.reload()); return false;">Refresh Versions</a>
    <p>Generate Go version wrappers with coverage instrumentation enabled.</p>
    
    <div class="usage">
        <h3>Quick Start</h3>
        <pre>
# Download wrapper for Go 1.23 with coverage
curl -L https://coverage.example.com/go1.23-cov > go1.23-cov
chmod +x go1.23-cov

# Or as a Go file
curl https://coverage.example.com/go1.23-cov.go > go1.23-cov.go
go build -o go1.23-cov go1.23-cov.go

# Use it
./go1.23-cov download
./go1.23-cov build -cover -o myapp main.go
GOCOVERDIR=/tmp/coverage ./myapp
./go1.23-cov tool covdata percent -i=/tmp/coverage</pre>
    </div>

    <h2>Available Versions</h2>
    <div id="versions">Loading versions...</div>

    <h2>API Endpoints</h2>
    <ul>
        <li><code>GET /api/versions</code> - List all available versions</li>
        <li><code>GET /go{version}-cov</code> - Get shell script wrapper</li>
        <li><code>GET /go{version}-cov.go</code> - Get Go source for wrapper</li>
        <li><code>GET /refresh</code> - Refresh version list from go.dev</li>
    </ul>

    <script>
        fetch('/api/versions')
            .then(r => r.json())
            .then(versions => {
                const container = document.getElementById('versions');
                container.innerHTML = '';
                
                Object.entries(versions)
                    .sort((a, b) => b[0].localeCompare(a[0]))
                    .forEach(([v, info]) => {
                        const div = document.createElement('div');
                        div.className = 'version' + (info.stable ? ' stable' : '');
                        
                        let sourceUrl = '';
                        info.files.forEach(f => {
                            if (f.kind === 'source') {
                                sourceUrl = 'https://go.dev/dl/' + f.filename;
                            }
                        });
                        
                        div.innerHTML = ` + "`" + `
                            <strong>${v}</strong> ${info.stable ? '(stable)' : ''} - 
                            <a href="/${v}-cov">Shell wrapper</a> |
                            <a href="/${v}-cov.go">Go source</a> |
                            <a href="${sourceUrl}" target="_blank">Official source</a>
                        ` + "`" + `;
                        container.appendChild(div);
                    });
            })
            .catch(err => {
                document.getElementById('versions').innerHTML = 'Error loading versions: ' + err;
            });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, indexHTML)
}

func handleVersionsAPI(w http.ResponseWriter, r *http.Request) {
	versionCacheMu.RLock()
	defer versionCacheMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versionCache)
}

func handleRefreshVersions(w http.ResponseWriter, r *http.Request) {
	refreshVersionCache()

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "refreshed",
			"versions": len(versionCache),
		})
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func handleGenerateWrapper(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimPrefix(path, "generate/")

	// Check if it's a .go file request
	isGoFile := strings.HasSuffix(path, ".go")
	if isGoFile {
		path = strings.TrimSuffix(path, ".go")
	}

	// Extract version from path
	version := strings.TrimSuffix(path, "-cov")
	if !versionRegex.MatchString(version) {
		http.Error(w, "Invalid version format", http.StatusBadRequest)
		return
	}

	// Get version info
	vInfo, ok := getVersionInfo(version)
	if !ok {
		// Try to construct info for unknown versions
		vInfo = VersionInfo{
			Version: version,
			Stable:  false,
			Files: []File{{
				Filename: version + ".src.tar.gz",
				Kind:     "source",
			}},
		}
	}

	// Find source file
	sourceURL, checksum := findSourceFile(vInfo)

	data := TemplateData{
		Version:     version,
		VersionInfo: vInfo,
	}

	// Add source info to template data
	if isGoFile {
		// Return Go source
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-cov.go", version))

		// Create a new template with extra data
		tmplData := struct {
			TemplateData
			SourceURL string
			Checksum  string
		}{
			TemplateData: data,
			SourceURL:    sourceURL,
			Checksum:     checksum,
		}

		if err := wrapperTemplate.Execute(w, tmplData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		// Return shell script wrapper
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-cov", version))

		scriptURL := r.Host + "/" + version + "-cov.go"
		if r.TLS == nil {
			scriptURL = "http://" + scriptURL
		} else {
			scriptURL = "https://" + scriptURL
		}

		// Generate shell script that downloads and builds the Go wrapper
		shellScript := fmt.Sprintf(`#!/bin/sh
# Coverage-enabled wrapper for %s
# Dynamically generated by Go Coverage Builder

set -e

# Configuration
VERSION="%s"
WRAPPER_NAME="${VERSION}-cov"
BUILD_DIR="${HOME}/.go-coverage-builds"

# Create build directory
mkdir -p "${BUILD_DIR}"
cd "${BUILD_DIR}"

# Download Go wrapper if needed
if [ ! -f "${WRAPPER_NAME}.go" ]; then
    echo "Downloading ${VERSION} coverage wrapper..."
    curl -sL "%s" > "${WRAPPER_NAME}.go" || {
        echo "Failed to download wrapper"
        exit 1
    }
fi

# Build wrapper if needed
if [ ! -f "${WRAPPER_NAME}" ] || [ "${WRAPPER_NAME}.go" -nt "${WRAPPER_NAME}" ]; then
    echo "Building ${WRAPPER_NAME}..."
    go build -o "${WRAPPER_NAME}" "${WRAPPER_NAME}.go" || {
        echo "Failed to build wrapper" 
        exit 1
    }
fi

# Execute wrapper with all arguments
exec "${BUILD_DIR}/${WRAPPER_NAME}" "$@"
`, version, version, scriptURL)

		fmt.Fprint(w, shellScript)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	versionCacheMu.RLock()
	versions := len(versionCache)
	lastFetch := lastVersionFetch
	versionCacheMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"versions":    versions,
		"last_update": lastFetch,
		"cache_age":   time.Since(lastFetch).String(),
	})
}
