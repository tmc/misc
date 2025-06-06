package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestGoCarrierServer(t *testing.T) {
	engine := &script.Engine{
		Cmds: scripttest.DefaultCmds(),
		Conds: scripttest.DefaultConds(),
	}
	
	// Add our custom commands
	engine.Cmds["wait-port"] = cmdWaitPort()
	engine.Cmds["http-get"] = cmdHttpGet()
	engine.Cmds["save-file"] = cmdSaveFile()
	engine.Cmds["unzip"] = cmdUnzip()
	engine.Cmds["go-cov-server"] = cmdGoServer()

	scripttest.Test(t, context.Background(), engine, nil, "testdata/*.txt")
}

func serverMain() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleModule)
	
	fmt.Printf("Starting go-cov service on port %s\n", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		return 1
	}
	return 0
}

func cmdWaitPort() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "wait for port to be listening",
			Args:    "host:port",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 1 {
				return nil, script.ErrUsage
			}
			
			addr := args[0]
			timeout := 10 * time.Second
			deadline := time.Now().Add(timeout)
			
			return func(s *script.State) (stdout, stderr string, err error) {
				for time.Now().Before(deadline) {
					conn, err := net.DialTimeout("tcp", addr, time.Second)
					if err == nil {
						conn.Close()
						return "", "", nil
					}
					time.Sleep(100 * time.Millisecond)
				}
				return "", "", fmt.Errorf("timeout waiting for port %s to be listening", addr)
			}, nil
		},
	)
}

func cmdHttpGet() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "make HTTP GET request",
			Args:    "url",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 1 {
				return nil, script.ErrUsage
			}
			
			url := args[0]
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "http://" + url
			}
			
			return func(s *script.State) (stdout, stderr string, err error) {
				resp, err := http.Get(url)
				if err != nil {
					return "", "", err
				}
				defer resp.Body.Close()
				
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return "", "", err
				}
				
				s.Setenv("HTTPBODY", string(body))
				return string(body), "", nil
			}, nil
		},
	)
}

func cmdSaveFile() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "save HTTP response body to file",
			Args:    "filename",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 1 {
				return nil, script.ErrUsage
			}
			
			filename := args[0]
			
			return func(s *script.State) (stdout, stderr string, err error) {
				body, ok := s.LookupEnv("HTTPBODY")
				if !ok || body == "" {
					return "", "", fmt.Errorf("no HTTP response body to save")
				}
				
				if err := os.WriteFile(s.Path(filename), []byte(body), 0644); err != nil {
					return "", "", err
				}
				return "", "", nil
			}, nil
		},
	)
}

func cmdGoServer() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "start go-cov server",
			Args:    "",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 0 {
				return nil, script.ErrUsage
			}
			
			return func(s *script.State) (stdout, stderr string, err error) {
				// Start the server in background mode
				go func() {
					serverMain()
				}()
				return "", "", nil
			}, nil
		},
	)
}

func cmdUnzip() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "unzip file",
			Args:    "filename",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) != 1 {
				return nil, script.ErrUsage
			}
			
			filename := args[0]
			
			return func(s *script.State) (stdout, stderr string, err error) {
				cmd := exec.Command("unzip", "-q", s.Path(filename))
				cmd.Dir = s.Getwd()
				
				if err := cmd.Run(); err != nil {
					return "", "", err
				}
				return "", "", nil
			}, nil
		},
	)
}

// Include all the server handler functions here
var versionPattern = regexp.MustCompile(`^go1\.\d+(?:\.\d+)?(?:rc\d+|beta\d+)?$`)

func handleModule(w http.ResponseWriter, r *http.Request) {
	modulePath := strings.TrimPrefix(r.URL.Path, "/")
	
	if r.URL.Query().Get("go-get") == "1" {
		handleGoGet(w, r, modulePath)
		return
	}
	
	if strings.Contains(r.URL.Path, "/@v/") {
		handleModuleProxy(w, r, modulePath)
		return
	}
	
	handleModuleInfo(w, r, modulePath)
}

func handleGoGet(w http.ResponseWriter, r *http.Request, modulePath string) {
	version := extractGoVersionFromModule(modulePath)
	if version == "" {
		http.Error(w, "Invalid Go version", http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta name="go-import" content="go.tmc.dev/go-cov git https://github.com/tmc/misc">
<meta name="go-source" content="go.tmc.dev/go-cov https://github.com/tmc/misc https://github.com/tmc/misc/tree/master{/dir} https://github.com/tmc/misc/blob/master{/dir}/{file}#L{line}">
</head>
<body>
<p>Coverage-enabled Go installer for %s</p>
<p>Usage: <code>go run go.tmc.dev/go-cov%s@latest</code></p>
</body>
</html>`, version, strings.TrimPrefix(version, "go"))
}

func handleModuleProxy(w http.ResponseWriter, r *http.Request, modulePath string) {
	parts := strings.Split(r.URL.Path, "/@v/")
	if len(parts) != 2 {
		http.Error(w, "Invalid module proxy request", http.StatusBadRequest)
		return
	}
	
	version := strings.TrimSuffix(parts[1], ".info")
	version = strings.TrimSuffix(version, ".mod")
	version = strings.TrimSuffix(version, ".zip")
	
	goVersion := extractGoVersionFromModule(modulePath)
	if goVersion == "" {
		http.Error(w, "Invalid Go version in module path", http.StatusBadRequest)
		return
	}
	
	if strings.HasSuffix(r.URL.Path, ".info") {
		handleVersionInfo(w, r, version)
	} else if strings.HasSuffix(r.URL.Path, ".mod") {
		handleGoMod(w, r, goVersion)
	} else if strings.HasSuffix(r.URL.Path, ".zip") {
		handleModuleZip(w, r, goVersion)
	} else if r.URL.Path == "/@v/list" {
		handleVersionList(w, r)
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func handleVersionInfo(w http.ResponseWriter, r *http.Request, version string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "Version": "%s",
  "Time": "2024-12-10T10:00:00Z"
}`, version)
}

func handleGoMod(w http.ResponseWriter, r *http.Request, goVersion string) {
	moduleName := extractModuleNameFromPath(r.URL.Path)
	
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, `module %s

go 1.21

require (
	golang.org/x/mod v0.14.0
	golang.org/x/sys v0.15.0
)
`, moduleName)
}

func handleModuleZip(w http.ResponseWriter, r *http.Request, goVersion string) {
	w.Header().Set("Content-Type", "application/zip")
	
	moduleName := extractModuleNameFromPath(r.URL.Path)
	mainGo, err := generateMainGo(goVersion)
	if err != nil {
		http.Error(w, "Failed to generate main.go: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	goMod := generateGoMod(moduleName)
	
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	
	modFile, err := zipWriter.Create("go.mod")
	if err != nil {
		http.Error(w, "Failed to create go.mod in zip", http.StatusInternalServerError)
		return
	}
	if _, err := modFile.Write([]byte(goMod)); err != nil {
		http.Error(w, "Failed to write go.mod", http.StatusInternalServerError)
		return
	}
	
	mainFile, err := zipWriter.Create("main.go")
	if err != nil {
		http.Error(w, "Failed to create main.go in zip", http.StatusInternalServerError)
		return
	}
	if _, err := mainFile.Write([]byte(mainGo)); err != nil {
		http.Error(w, "Failed to write main.go", http.StatusInternalServerError)
		return
	}
	
	if err := zipWriter.Close(); err != nil {
		http.Error(w, "Failed to close zip", http.StatusInternalServerError)
		return
	}
	
	w.Write(buf.Bytes())
}

func handleVersionList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	versions := []string{
		"latest",
	}
	
	for _, v := range versions {
		fmt.Fprintln(w, v)
	}
}

func handleModuleInfo(w http.ResponseWriter, r *http.Request, modulePath string) {
	version := extractGoVersionFromModule(modulePath)
	if version == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>go-cov - Coverage-enabled Go installer</title></head>
<body>
<h1>go-cov - Coverage-enabled Go installer</h1>
<p>Install coverage-enabled Go versions:</p>
<pre>go run go.tmc.dev/go-cov1.24.3@latest</pre>
<p>Available versions: go1.21.0 through go1.24.3</p>
</body>
</html>`)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Coverage-enabled Go %s</title></head>
<body>
<h1>Coverage-enabled Go %s</h1>
<p>Install with:</p>
<pre>go run go.tmc.dev/go-cov%s@latest</pre>
</body>
</html>`, version, version, strings.TrimPrefix(version, "go"))
}

func extractGoVersionFromModule(modulePath string) string {
	base := filepath.Base(modulePath)
	if strings.HasPrefix(base, "go-cov") {
		return "go" + strings.TrimPrefix(base, "go-cov")
	}
	return ""
}

func extractModuleNameFromPath(urlPath string) string {
	parts := strings.Split(urlPath, "/@v/")
	if len(parts) > 0 {
		return strings.TrimPrefix(parts[0], "/")
	}
	return "go.tmc.dev/go-cov1.24.3"
}

func generateGoMod(moduleName string) string {
	return fmt.Sprintf(`module %s

go 1.21

require (
	golang.org/x/mod v0.14.0
	golang.org/x/sys v0.15.0
)
`, moduleName)
}

const mainGoTemplate = `package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	version        = "{{.Version}}"
	goSourceURL    = "https://go.dev/dl/"
	userAgent      = "go-cov/1.0"
)

func main() {
	fmt.Printf("Installing coverage-enabled Go %s from source...\n", version)

	if err := downloadGoSource(version); err != nil {
		log.Fatalf("Failed to download Go source %s: %v", version, err)
	}

	if err := buildCoverageEnabledGo(version); err != nil {
		log.Fatalf("Failed to build coverage-enabled Go %s: %v", version, err)
	}

	fmt.Printf("Coverage-enabled Go %s built and installed successfully!\n", version)
	fmt.Printf("Use: export PATH=$HOME/sdk/%s-cov/bin:$PATH\n", version)
}

func downloadGoSource(version string) error {
	// Download Go source code instead of binary
	filename := fmt.Sprintf("%s.src.tar.gz", version)
	url := fmt.Sprintf("%s%s", goSourceURL, filename)
	
	fmt.Printf("Downloading Go source: %s...\n", url)
	
	sdkDir := filepath.Join(os.Getenv("HOME"), "sdk")
	if err := os.MkdirAll(sdkDir, 0755); err != nil {
		return fmt.Errorf("failed to create SDK directory: %%w", err)
	}
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download Go source: %%w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Go source: HTTP %%d", resp.StatusCode)
	}
	
	sourceDir := filepath.Join(sdkDir, version+"-src")
	if err := os.RemoveAll(sourceDir); err != nil {
		return fmt.Errorf("failed to remove existing source directory: %%w", err)
	}
	
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %%w", err)
	}
	
	// Extract source tarball
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %%w", err)
	}
	defer gzr.Close()
	
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %%w", err)
		}
		
		// Remove "go/" prefix from paths
		path := strings.TrimPrefix(header.Name, "go/")
		if path == header.Name {
			continue // Skip if no "go/" prefix
		}
		
		target := filepath.Join(sourceDir, path)
		
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %%w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %%w", err)
			}
			
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %%w", err)
			}
			
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file: %%w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink: %%w", err)
			}
		}
	}
	
	fmt.Printf("Go %s source extracted to %s\n", version, sourceDir)
	return nil
}

func buildCoverageEnabledGo(version string) error {
	sdkDir := filepath.Join(os.Getenv("HOME"), "sdk")
	sourceDir := filepath.Join(sdkDir, version+"-src")
	buildDir := filepath.Join(sdkDir, version+"-cov")
	
	fmt.Printf("Building coverage-enabled Go from source...\n")
	fmt.Printf("Source: %s\n", sourceDir)
	fmt.Printf("Target: %s\n", buildDir)
	
	// Find bootstrap Go compiler
	bootstrapGo, err := findBootstrapGo()
	if err != nil {
		return fmt.Errorf("failed to find bootstrap Go: %%w", err)
	}
	fmt.Printf("Using bootstrap Go: %s\n", bootstrapGo)
	
	// Prepare build environment
	srcDir := filepath.Join(sourceDir, "src")
	
	// Create build script with coverage experiments enabled
	buildScript := fmt.Sprintf(` + "`#!/bin/bash\nset -e\nset -x\n\necho \"Building coverage-enabled Go %s...\"\necho \"Source directory: %s\"\necho \"Target directory: %s\"\necho \"Bootstrap Go: %s\"\n\ncd %s\n\n# Set build environment\nexport GOROOT_BOOTSTRAP=%s\nexport GOROOT=%s\nexport GOOS=%s\nexport GOARCH=%s\n\n# Enable coverage experiments\nexport GOEXPERIMENT=coverageredesign\n\n# Show environment\necho \"Build environment:\"\necho \"GOROOT_BOOTSTRAP=$GOROOT_BOOTSTRAP\"\necho \"GOROOT=$GOROOT\"\necho \"GOOS=$GOOS\"\necho \"GOARCH=$GOARCH\"\necho \"GOEXPERIMENT=$GOEXPERIMENT\"\n\n# Run the build\necho \"Starting Go build...\"\n./make.bash\n\necho \"Build completed successfully!\"\necho \"Go binary location: $GOROOT/bin/go\"\n$GOROOT/bin/go version\n`" + `", 
		version, sourceDir, buildDir, bootstrapGo, srcDir, bootstrapGo, buildDir, runtime.GOOS, runtime.GOARCH)
	
	scriptPath := filepath.Join(sourceDir, "build-coverage.sh")
	if err := os.WriteFile(scriptPath, []byte(buildScript), 0755); err != nil {
		return fmt.Errorf("failed to write build script: %%w", err)
	}
	
	// Copy source to build directory
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to remove existing build directory: %%w", err)
	}
	
	fmt.Printf("Copying source tree to build directory...\n")
	if err := copyDir(sourceDir, buildDir); err != nil {
		return fmt.Errorf("failed to copy source to build directory: %%w", err)
	}
	
	// Execute build script in build directory
	buildScriptPath := filepath.Join(buildDir, "build-coverage.sh")
	fmt.Printf("Executing build script: %s\n", buildScriptPath)
	
	cmd := exec.Command("/bin/bash", buildScriptPath)
	cmd.Dir = buildDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"GOROOT_BOOTSTRAP="+bootstrapGo,
		"GOROOT="+buildDir,
		"GOOS="+runtime.GOOS,
		"GOARCH="+runtime.GOARCH,
		"GOEXPERIMENT=coverageredesign",
	)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build coverage-enabled Go: %%w", err)
	}
	
	// Verify the build
	goBinary := filepath.Join(buildDir, "bin", "go")
	if _, err := os.Stat(goBinary); err != nil {
		return fmt.Errorf("build completed but go binary not found at %s: %%w", goBinary, err)
	}
	
	// Test the built Go
	fmt.Printf("Testing built Go binary...\n")
	testCmd := exec.Command(goBinary, "version")
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("built Go binary failed version test: %%w", err)
	}
	
	return nil
}

func findBootstrapGo() (string, error) {
	// Try to find an existing Go installation to use as bootstrap
	candidates := []string{
		"/usr/local/go",
		"/opt/go",
		"/usr/lib/go",
	}
	
	// First try the current Go installation
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		if _, err := os.Stat(filepath.Join(goroot, "bin", "go")); err == nil {
			return goroot, nil
		}
	}
	
	// Try to find Go in PATH
	if goPath, err := exec.LookPath("go"); err == nil {
		// Get GOROOT from the go command
		cmd := exec.Command(goPath, "env", "GOROOT")
		output, err := cmd.Output()
		if err == nil {
			goroot := strings.TrimSpace(string(output))
			if _, err := os.Stat(filepath.Join(goroot, "bin", "go")); err == nil {
				return goroot, nil
			}
		}
	}
	
	// Try common installation locations
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "bin", "go")); err == nil {
			return candidate, nil
		}
	}
	
	return "", fmt.Errorf("no suitable bootstrap Go compiler found. Please install Go first or set GOROOT_BOOTSTRAP")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		
		dstPath := filepath.Join(dst, relPath)
		
		if info.Mode()&os.ModeSymlink != 0 {
			// Handle symlink
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, dstPath)
		}
		
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		
		return copyFile(path, dstPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	_, err = io.Copy(dstFile, srcFile)
	return err
}
`

func generateMainGo(goVersion string) (string, error) {
	tmpl := template.Must(template.New("main").Parse(mainGoTemplate))
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Version string }{Version: goVersion}); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}