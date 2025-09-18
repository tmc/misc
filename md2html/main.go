package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	admonitions "github.com/stefanfritsch/goldmark-admonitions"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/alecthomas/chroma/v2/styles"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"go.abhg.dev/goldmark/toc"
)

//go:embed templates
var templates embed.FS

var (
	flagInput   = flag.String("input", "", "input file (default: directory listing)")
	flagHTTP    = flag.String("http", ":8080", "HTTP server bind address")
	flagOpen    = flag.Bool("open", false, "automatically open browser")
	flagVerbose = flag.Bool("v", false, "verbose logging")
	flagTitle   = flag.String("title", "Markdown Preview", "HTML title")
	flagCSS     = flag.String("css", "", "path to custom CSS file")
	flagDepth   = flag.Int("depth", 2, "directory traversal depth for listings (minimum 2)")
	flagTOC     = flag.Bool("toc", true, "generate table of contents")
)


type server struct {
	mu          sync.RWMutex
	content     string
	clients     map[chan string]bool
	clientsMu   sync.RWMutex
	cssContent  string
	inputPath   string
}

func main() {
	flag.Parse()
	runServer()
}


func runServer() {
	s := &server{
		clients:   make(map[chan string]bool),
		inputPath: *flagInput,
	}

	// Load initial content
	if *flagInput != "" {
		content, err := os.ReadFile(*flagInput)
		if err != nil {
			log.Printf("Error reading initial file: %v", err)
		} else {
			s.mu.Lock()
			s.content = string(content)
			s.mu.Unlock()
		}
	}

	// Load CSS if provided
	if *flagCSS != "" {
		css, err := os.ReadFile(*flagCSS)
		if err != nil {
			log.Printf("Error reading CSS file: %v", err)
		} else {
			s.cssContent = string(css)
		}
	}

	// Set up file watching
	if *flagInput != "" {
		go s.watchFiles()
	} else {
		// Watch directory for changes when showing directory listing
		go s.watchDirectory()
	}

	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/events", s.handleSSE)
	http.HandleFunc("/raw", s.handleRaw)

	addr := *flagHTTP
	// Format URL for display and browser opening
	displayURL := formatServerURL(addr)
	log.Printf("Server starting on %s", displayURL)

	// Open browser if requested
	if *flagOpen {
		go func() {
			if !openBrowser(displayURL) {
				log.Printf("Failed to open browser. Please visit %s", displayURL)
			}
		}()
	}

	log.Fatal(http.ListenAndServe(addr, nil))
}

func (s *server) watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch the directory containing the markdown file (handles atomic replacements)
	dir := filepath.Dir(*flagInput)
	if dir == "" {
		dir = "."
	}
	err = watcher.Add(dir)
	if err != nil {
		log.Printf("Error watching directory: %v", err)
		return
	}

	// Watch CSS file directory if provided
	if *flagCSS != "" {
		cssDir := filepath.Dir(*flagCSS)
		if cssDir == "" {
			cssDir = "."
		}
		if cssDir != dir {
			err = watcher.Add(cssDir)
			if err != nil {
				log.Printf("Error watching CSS directory: %v", err)
			}
		}
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Debounce rapid changes
				time.Sleep(100 * time.Millisecond)

				eventFile := filepath.Base(event.Name)
				inputFile := filepath.Base(*flagInput)

				if eventFile == inputFile {
					if *flagVerbose {
						log.Printf("Detected change to %s, updating content", inputFile)
					}
					content, err := os.ReadFile(*flagInput)
					if err != nil {
						log.Printf("Error reading file: %v", err)
						continue
					}
					s.mu.Lock()
					s.content = string(content)
					s.mu.Unlock()
					s.notifyClients()
				} else if *flagCSS != "" && eventFile == filepath.Base(*flagCSS) {
					css, err := os.ReadFile(*flagCSS)
					if err != nil {
						log.Printf("Error reading CSS file: %v", err)
						continue
					}
					s.mu.Lock()
					s.cssContent = string(css)
					s.mu.Unlock()
					s.notifyClients()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	content := s.content
	css := s.cssContent
	s.mu.RUnlock()

	// Check if a specific file is requested via query parameter
	if file := r.URL.Query().Get("file"); file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading file %s: %v", file, err), http.StatusNotFound)
			return
		}
		html := renderHTMLWithLiveReload(string(content), file, css)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	// If no input file specified and content is empty, show directory listing
	if *flagInput == "" && content == "" {
		wd, err := os.Getwd()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting working directory: %v", err), http.StatusInternalServerError)
			return
		}

		listing, err := generateDirectoryListing(wd)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error generating directory listing: %v", err), http.StatusInternalServerError)
			return
		}

		html := renderHTMLWithLiveReload(listing, "Directory Listing", css)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	html := renderHTMLWithLiveReload(content, *flagTitle, css)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	content := s.content
	s.mu.RUnlock()

	html := markdownToHTML(content)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan string, 10)

	s.clientsMu.Lock()
	s.clients[clientChan] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientChan)
		s.clientsMu.Unlock()
		close(clientChan)
	}()

	// Keep connection alive
	fmt.Fprintf(w, "data: connected\n\n")
	w.(http.Flusher).Flush()

	// Listen for updates
	for {
		select {
		case msg := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *server) watchDirectory() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating directory watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch current directory and all subdirectories recursively
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				log.Printf("Error watching directory %s: %v", path, err)
			} else if *flagVerbose {
				log.Printf("Watching directory: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error setting up recursive directory watching: %v", err)
		return
	}

	if *flagVerbose {
		log.Printf("Watching current directory for .md file changes")
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if *flagVerbose {
				log.Printf("Directory event: %s %s (isMarkdown: %v)",
					event.Op, event.Name,
					strings.HasSuffix(strings.ToLower(event.Name), ".md") || strings.HasSuffix(strings.ToLower(event.Name), ".markdown"))
			}

			// Check if it's a markdown file change
			if strings.HasSuffix(strings.ToLower(event.Name), ".md") ||
			   strings.HasSuffix(strings.ToLower(event.Name), ".markdown") {
				if event.Op&fsnotify.Write == fsnotify.Write ||
				   event.Op&fsnotify.Create == fsnotify.Create ||
				   event.Op&fsnotify.Remove == fsnotify.Remove ||
				   event.Op&fsnotify.Chmod == fsnotify.Chmod {
					// Debounce rapid changes
					time.Sleep(100 * time.Millisecond)

					if *flagVerbose {
						log.Printf("Markdown file changed: %s %s - notifying clients", event.Op, event.Name)
					}
					s.notifyClients()
				} else if *flagVerbose {
					log.Printf("Ignoring event %s for %s (not a watched operation)", event.Op, event.Name)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Directory watcher error: %v", err)
		}
	}
}

func (s *server) notifyClients() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	if *flagVerbose {
		log.Printf("Notifying %d SSE clients", len(s.clients))
	}

	for clientChan := range s.clients {
		select {
		case clientChan <- "reload":
			if *flagVerbose {
				log.Printf("Sent reload signal to client")
			}
		default:
			if *flagVerbose {
				log.Printf("Client channel full, skipping")
			}
		}
	}
}

func generateChromaCSS() string {
	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithLineNumbers(false),
	)

	var buf bytes.Buffer
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return ""
	}

	return buf.String()
}

// convertGitHubAlerts converts GitHub alert syntax to goldmark-admonitions syntax
func convertGitHubAlerts(markdown string) string {
	// Map GitHub alert types to admonition types
	alertMap := map[string]string{
		"[!NOTE]":      "note",
		"[!TIP]":       "tip",
		"[!IMPORTANT]": "important",
		"[!WARNING]":   "warning",
		"[!CAUTION]":   "danger", // goldmark-admonitions uses "danger" instead of "caution"
	}

	lines := strings.Split(markdown, "\n")
	var result []string
	inAlert := false
	currentAlertType := ""
	alertContent := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this starts a GitHub alert
		if strings.HasPrefix(trimmed, "> ") {
			alertLine := strings.TrimPrefix(trimmed, "> ")
			for githubType, admonitionType := range alertMap {
				if strings.HasPrefix(alertLine, githubType) {
					// Start collecting alert content
					inAlert = true
					currentAlertType = admonitionType
					// Get the title (text after the alert type on the same line)
					title := strings.TrimSpace(strings.TrimPrefix(alertLine, githubType))
					if title == "" {
						title = strings.Title(admonitionType)
					}
					alertContent = []string{"!!!" + currentAlertType + " " + title}
					break
				}
			}
			if inAlert && !strings.Contains(trimmed, "[!") {
				// This is content within the alert
				content := strings.TrimPrefix(trimmed, "> ")
				alertContent = append(alertContent, content)
			} else if !inAlert {
				// Regular blockquote, not an alert
				result = append(result, line)
			}
		} else if inAlert {
			// End of alert block
			alertContent = append(alertContent, "!!!")
			result = append(result, alertContent...)
			result = append(result, line)
			inAlert = false
			alertContent = []string{}
		} else {
			// Regular line
			result = append(result, line)
		}
	}

	// Handle case where file ends while in alert
	if inAlert {
		alertContent = append(alertContent, "!!!")
		result = append(result, alertContent...)
	}

	return strings.Join(result, "\n")
}

func markdownToHTML(markdown string) string {
	// Convert GitHub alerts to admonitions format
	markdown = convertGitHubAlerts(markdown)

	// Core secure extensions
	extensions := []goldmark.Extender{
		extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, linkify, task lists)
		extension.Footnote,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"), // Secure, well-tested GitHub style
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(false), // Disabled for cleaner output
				chromahtml.WithClasses(true),      // Use CSS classes instead of inline styles
			),
		),
		&admonitions.Extender{},
	}

	// Build markdown processor
	var tocRenderer *toc.Extender
	if *flagTOC {
		tocRenderer = &toc.Extender{
			MinDepth: 1, // Include h1 like GitHub
			MaxDepth: 6, // Include all heading levels like GitHub
			Compact:  false, // Show full hierarchy like GitHub
		}
		extensions = append(extensions, tocRenderer)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Required for TOC functionality
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(), // More secure than HTML5 mode
			// Removed html.WithUnsafe() for better security
		),
	)

	// Render markdown content (TOC extension handles TOC automatically)
	source := []byte(markdown)
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return fmt.Sprintf("<p>Error rendering markdown: %v</p>", err)
	}

	return buf.String()
}

func hasHTML(content string) bool {
	return strings.Contains(content, "<") && strings.Contains(content, ">")
}

func processWithHTML2MD(content string) string {
	// Try to use html2md if it exists
	cmd := exec.Command("html2md")
	cmd.Stdin = strings.NewReader(content)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err == nil {
		return out.String()
	}
	return ""
}

func formatServerURL(addr string) string {
	// If address starts with ":", prepend localhost
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	// If address doesn't have a scheme, add http://
	if !strings.Contains(addr, "://") {
		return "http://" + addr
	}
	return addr
}

func generateDirectoryListing(dir string) (string, error) {
	files, err := findMarkdownFiles(dir, *flagDepth)
	if err != nil {
		return "", err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Directory Listing: %s\n\n", dir))

	if len(files) == 0 {
		buf.WriteString("*No markdown files found in this directory.*\n")
		return buf.String(), nil
	}

	for _, file := range files {
		buf.WriteString(fmt.Sprintf("- [%s](?file=%s) (%d bytes, modified %s)\n",
			file.RelPath, file.RelPath, file.Size, file.ModTime.Format("2006-01-02 15:04:05")))
	}

	return buf.String(), nil
}

type markdownFile struct {
	RelPath string
	Size    int64
	ModTime time.Time
}

func findMarkdownFiles(rootDir string, maxDepth int) ([]markdownFile, error) {
	var files []markdownFile

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path and depth
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Skip if we've exceeded max depth
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth >= maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a markdown file
		if !info.IsDir() {
			name := strings.ToLower(info.Name())
			if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") {
				files = append(files, markdownFile{
					RelPath: relPath,
					Size:    info.Size(),
					ModTime: info.ModTime(),
				})
			}
		}

		return nil
	})

	return files, err
}

func openBrowser(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return false
	}

	return cmd.Start() == nil
}

func renderHTML(markdown, title, customCSS string) string {
	html := markdownToHTML(markdown)

	tmplStr := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            color: #333;
            transition: opacity 0.1s ease;
        }
        h1, h2, h3, h4, h5, h6 {
            margin-top: 1.5em;
            margin-bottom: 0.5em;
        }
        code {
            background: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', monospace;
        }
        pre {
            background: #f4f4f4;
            padding: 1em;
            border-radius: 5px;
            overflow-x: auto;
        }
        pre code {
            background: none;
            padding: 0;
        }
        blockquote {
            border-left: 4px solid #ddd;
            padding-left: 1em;
            margin-left: 0;
            color: #666;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 1em 0;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        th {
            background: #f4f4f4;
        }
        img {
            max-width: 100%;
        }
        a {
            color: #0366d6;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        {{.CustomCSS}}
    </style>
</head>
<body>
    {{.Content}}
</body>
</html>`

	tmpl := template.Must(template.New("html").Parse(tmplStr))

	var buf bytes.Buffer
	data := struct {
		Title     string
		Content   template.HTML
		CustomCSS template.CSS
		ChromaCSS template.CSS
		Verbose   bool
	}{
		Title:     title,
		Content:   template.HTML(html),
		CustomCSS: template.CSS(customCSS),
		ChromaCSS: template.CSS(generateChromaCSS()),
		Verbose:   *flagVerbose,
	}

	tmpl.Execute(&buf, data)
	return buf.String()
}

func renderHTMLWithLiveReload(markdown, title, customCSS string) string {
	html := markdownToHTML(markdown)

	tmplStr := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            color: #333;
            transition: opacity 0.1s ease;
        }
        h1, h2, h3, h4, h5, h6 {
            margin-top: 1.5em;
            margin-bottom: 0.5em;
        }
        code {
            background: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', monospace;
        }
        pre {
            background: #f4f4f4;
            padding: 1em;
            border-radius: 5px;
            overflow-x: auto;
        }
        pre code {
            background: none;
            padding: 0;
        }
        blockquote {
            border-left: 4px solid #ddd;
            padding-left: 1em;
            margin-left: 0;
            color: #666;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 1em 0;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        th {
            background: #f4f4f4;
        }
        img {
            max-width: 100%;
        }
        a {
            color: #0366d6;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .connection-status {
            position: fixed;
            top: 10px;
            right: 10px;
            padding: 5px 10px;
            border-radius: 3px;
            font-size: 12px;
            background: #28a745;
            color: white;
        }
        .connection-status.disconnected {
            background: #dc3545;
        }
        /* GitHub-style Table of Contents */
        #table-of-contents {
            background: #f6f8fa;
            border: 1px solid #d0d7de;
            border-radius: 6px;
            padding: 16px;
            margin: 16px 0;
        }
        #table-of-contents h2 {
            margin-top: 0;
            margin-bottom: 12px;
            font-size: 16px;
            font-weight: 600;
            color: #24292f;
        }
        #table-of-contents ul {
            list-style: none;
            padding-left: 0;
            margin: 0;
        }
        #table-of-contents li {
            margin: 4px 0;
        }
        #table-of-contents a {
            color: #0969da;
            text-decoration: none;
            font-size: 14px;
        }
        #table-of-contents a:hover {
            text-decoration: underline;
        }
        #table-of-contents ul ul {
            padding-left: 16px;
        }
        /* GitHub Alerts - exact GitHub styles */
        :root {
            --color-note: #0969da;
            --color-tip: #1a7f37;
            --color-warning: #9a6700;
            --color-severe: #bc4c00;
            --color-caution: #d1242f;
            --color-important: #8250df;
        }

        /* Map goldmark-admonitions classes to GitHub alert styles */
        .admonition {
            padding: 0.5rem 1rem;
            margin-bottom: 16px;
            color: inherit;
            border-left: .25em solid #888;
        }

        .admonition > :first-child {
            margin-top: 0;
            display: flex;
            font-weight: 500;
            align-items: center;
            line-height: 1;
        }

        .admonition > :last-child {
            margin-bottom: 0;
        }

        .adm-note {
            border-left-color: var(--color-note);
        }

        .adm-note > :first-child {
            color: var(--color-note);
        }

        .adm-tip {
            border-left-color: var(--color-tip);
        }

        .adm-tip > :first-child {
            color: var(--color-tip);
        }

        .adm-important {
            border-left-color: var(--color-important);
        }

        .adm-important > :first-child {
            color: var(--color-important);
        }

        .adm-warning {
            border-left-color: var(--color-warning);
        }

        .adm-warning > :first-child {
            color: var(--color-warning);
        }

        .adm-danger {
            border-left-color: var(--color-caution);
        }

        .adm-danger > :first-child {
            color: var(--color-caution);
        }
        {{.CustomCSS}}
        {{.ChromaCSS}}
    </style>
    <script id="MathJax-script" async src="https://cdn.jsdelivr.net/npm/mathjax@3/es5/tex-mml-chtml.js"></script>
    <script>
        window.MathJax = {
            tex: {
                inlineMath: [['$', '$'], ['\\(', '\\)']],
                displayMath: [['$$', '$$'], ['\\[', '\\]']]
            }
        };
    </script>
    <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
    <script>
        document.addEventListener('DOMContentLoaded', function() {
            // Initialize Mermaid
            mermaid.initialize({
                startOnLoad: false,
                theme: 'default',
                securityLevel: 'loose'
            });

            // Convert code blocks with language-mermaid to mermaid diagrams
            document.querySelectorAll('code.language-mermaid').forEach((block, index) => {
                const div = document.createElement('div');
                div.className = 'mermaid';
                div.textContent = block.textContent;
                div.id = 'mermaid-' + index;
                block.parentElement.replaceWith(div);
            });

            // Render all mermaid diagrams
            mermaid.run();
        });
    </script>
</head>
<body>
    <div id="status" class="connection-status">Connected</div>
    {{.Content}}
    <script>
        (function() {
            const status = document.getElementById('status');
            const contentContainer = document.querySelector('body');

            // Create EventSource for SSE
            const eventSource = new EventSource('/events');

            eventSource.onopen = function() {
                {{if .Verbose}}console.log('SSE connected');{{end}}
                status.textContent = 'Connected';
                status.classList.remove('disconnected');
            };

            eventSource.onmessage = function(event) {
                {{if .Verbose}}console.log('SSE message:', event.data);{{end}}
                if (event.data === 'reload') {
                    {{if .Verbose}}console.log('Reloading page...');{{end}}
                    document.body.style.opacity = '0.8';
                    setTimeout(() => {
                        location.reload();
                    }, 100);
                }
            };

            eventSource.onerror = function() {
                {{if .Verbose}}console.log('SSE error, reconnecting...');{{end}}
                status.textContent = 'Reconnecting...';
                status.classList.add('disconnected');
                setTimeout(function() {
                    window.location.reload();
                }, 1000);
            };
        })();
    </script>
</body>
</html>`

	tmpl := template.Must(template.New("html").Parse(tmplStr))

	var buf bytes.Buffer
	data := struct {
		Title     string
		Content   template.HTML
		CustomCSS template.CSS
		ChromaCSS template.CSS
		Verbose   bool
	}{
		Title:     title,
		Content:   template.HTML(html),
		CustomCSS: template.CSS(customCSS),
		ChromaCSS: template.CSS(generateChromaCSS()),
		Verbose:   *flagVerbose,
	}

	tmpl.Execute(&buf, data)
	return buf.String()
}