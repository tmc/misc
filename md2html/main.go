package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/fsnotify/fsnotify"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	admonitions "github.com/stefanfritsch/goldmark-admonitions"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
)

//go:embed templates
var templates embed.FS

var (
	flagInput             = flag.String("input", "", "input file (default: directory listing)")
	flagHTTP              = flag.String("http", ":8080", "HTTP server bind address")
	flagOpen              = flag.Bool("open", false, "automatically open browser")
	flagVerbose           = flag.Bool("v", false, "verbose logging")
	flagTitle             = flag.String("title", "Markdown Preview", "HTML title")
	flagCSS               = flag.String("css", "", "path to custom CSS file")
	flagDepth             = flag.Int("depth", 2, "directory traversal depth for listings (minimum 2)")
	flagTOC               = flag.Bool("toc", false, "generate table of contents")
	flagAllowUnsafe       = flag.Bool("allow-unsafe", false, "allow unsafe HTML in markdown (use with caution)")
	flagTemplateDir       = flag.String("templates", "", "path to custom template directory (overrides embedded templates)")
	flagDataJSON          = flag.String("data-json", "", "path to JSON file to load as template data (available as .Data)")
	flagRenderFrontmatter = flag.Bool("render-frontmatter", false, "render YAML frontmatter as part of the document content")
	flagIndex             = flag.String("index", "", "default file to serve for root path (e.g., README.md, index.md)")
)

type server struct {
	mu         sync.RWMutex
	content    string
	clients    map[chan string]bool
	clientsMu  sync.RWMutex
	cssContent string
	inputPath  string
	jsonData   interface{} // Generic JSON data for templates
	shutdownCh chan struct{}

	// Batched reload management
	reloadPending bool
	reloadTimer   *time.Timer
	reloadTimerMu sync.Mutex
}

type DocumentData struct {
	Content     string
	Frontmatter map[string]interface{}
}

func loadJSONFile(filename string) (interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, err
	}

	return jsonData, nil
}

func parseFrontmatter(content string) (DocumentData, error) {
	// Use goldmark to parse frontmatter
	md := goldmark.New(goldmark.WithExtensions(meta.Meta))
	context := parser.NewContext()

	// Parse to extract metadata
	md.Parser().Parse(text.NewReader([]byte(content)), parser.WithContext(context))

	// Get metadata
	metaData := meta.Get(context)
	if metaData == nil {
		metaData = make(map[string]interface{})
	}

	// Strip frontmatter from content manually (goldmark-meta doesn't do this for us)
	strippedContent := stripFrontmatter(content)

	return DocumentData{
		Content:     strippedContent,
		Frontmatter: metaData,
	}, nil
}

func stripFrontmatter(content string) string {
	// Check for YAML frontmatter
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	// Find the closing delimiter
	lines := strings.Split(content, "\n")
	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		// No closing delimiter found, treat as regular content
		return content
	}

	// Extract content after frontmatter
	if frontmatterEnd+1 < len(lines) {
		return strings.Join(lines[frontmatterEnd+1:], "\n")
	}
	return ""
}

func main() {
	flag.Parse()
	runServer()
}

func runServer() {
	s := &server{
		clients:    make(map[chan string]bool),
		inputPath:  *flagInput,
		shutdownCh: make(chan struct{}),
	}

	// Load JSON data if provided
	if *flagDataJSON != "" {
		jsonData, err := loadJSONFile(*flagDataJSON)
		if err != nil {
			log.Printf("Error loading JSON data file: %v", err)
		} else {
			s.jsonData = jsonData
			if *flagVerbose {
				log.Printf("Loaded JSON data from: %s", *flagDataJSON)
			}
		}
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

	// Setup HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/events", s.handleSSE)
	mux.HandleFunc("/raw", s.handleRaw)

	addr := *flagHTTP
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

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

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")

		// Close shutdown channel to notify all goroutines
		close(s.shutdownCh)

		// Send shutdown signal to all clients
		s.clientsMu.Lock()
		for client := range s.clients {
			select {
			case client <- "shutdown":
			default:
				// Channel is full or closed, skip
			}
		}
		// Clear the clients map
		s.clients = make(map[chan string]bool)
		s.clientsMu.Unlock()

		// Shutdown server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		os.Exit(0)
	}()

	// Start server
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
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

	// Check if root path and index file is specified
	if r.URL.Path == "/" && *flagIndex != "" {
		if fileContent, err := os.ReadFile(*flagIndex); err == nil {
			doc, err := parseFrontmatter(string(fileContent))
			if err != nil {
				log.Printf("Error parsing frontmatter in %s: %v", *flagIndex, err)
				doc = DocumentData{Content: string(fileContent), Frontmatter: make(map[string]interface{})}
			}
			html := renderDocument(doc, *flagIndex, css, *flagIndex)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		} else if *flagVerbose {
			log.Printf("Index file %s not found, falling back to directory listing", *flagIndex)
		}
	}

	// Check if a specific file is requested via clean URL path
	if r.URL.Path != "/" {
		filePath := strings.TrimPrefix(r.URL.Path, "/")

		// Try the path as-is if it ends with .md
		var candidates []string
		if strings.HasSuffix(filePath, ".md") || strings.HasSuffix(filePath, ".markdown") {
			candidates = append(candidates, filePath)
		} else {
			// Try adding .md extension
			candidates = append(candidates, filePath+".md")
			candidates = append(candidates, filePath+".markdown")
		}

		for _, candidate := range candidates {
			if fileContent, err := os.ReadFile(candidate); err == nil {
				doc, err := parseFrontmatter(string(fileContent))
				if err != nil {
					log.Printf("Error parsing frontmatter in %s: %v", candidate, err)
					doc = DocumentData{Content: string(fileContent), Frontmatter: make(map[string]interface{})}
				}
				html := renderDocument(doc, candidate, css, candidate)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(html))
				return
			}
		}

		// File not found
		http.Error(w, fmt.Sprintf("File not found: %s", filePath), http.StatusNotFound)
		return
	}

	// Check if a specific file is requested via query parameter (for backward compatibility)
	if file := r.URL.Query().Get("file"); file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading file %s: %v", file, err), http.StatusNotFound)
			return
		}

		doc, err := parseFrontmatter(string(content))
		if err != nil {
			log.Printf("Error parsing frontmatter in %s: %v", file, err)
			doc = DocumentData{Content: string(content), Frontmatter: make(map[string]interface{})}
		}
		html := renderDocument(doc, file, css, file)
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

		doc := DocumentData{Content: listing, Frontmatter: make(map[string]interface{})}
		html := renderDocument(doc, "Directory Listing", css, wd)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	doc, err := parseFrontmatter(content)
	if err != nil {
		log.Printf("Error parsing frontmatter: %v", err)
		doc = DocumentData{Content: content, Frontmatter: make(map[string]interface{})}
	}
	html := renderDocument(doc, *flagTitle, css, "")
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
	}()

	// Keep connection alive
	fmt.Fprintf(w, "data: connected\n\n")
	w.(http.Flusher).Flush()

	// Create a local copy of shutdown channel to avoid panic
	shutdownCh := s.shutdownCh

	// Listen for updates
	for {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				// Channel closed, server shutting down
				fmt.Fprintf(w, "data: shutdown\n\n")
				w.(http.Flusher).Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		case <-shutdownCh:
			// Server is shutting down
			fmt.Fprintf(w, "data: shutdown\n\n")
			w.(http.Flusher).Flush()
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
	select {
	case <-time.After(50 * time.Millisecond):
	default:
		return // debounce
	}

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
	// Get light theme (github)
	lightStyle := styles.Get("github")
	if lightStyle == nil {
		lightStyle = styles.Fallback
	}

	// Get dark theme (github-dark)
	darkStyle := styles.Get("github-dark")
	if darkStyle == nil {
		darkStyle = styles.Get("dracula") // fallback dark theme
		if darkStyle == nil {
			darkStyle = lightStyle
		}
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithLineNumbers(false),
	)

	var buf bytes.Buffer

	// Generate light theme CSS
	if err := formatter.WriteCSS(&buf, lightStyle); err != nil {
		return ""
	}

	// Add dark theme CSS with media query
	buf.WriteString("\n@media (prefers-color-scheme: dark) {\n")
	if err := formatter.WriteCSS(&buf, darkStyle); err != nil {
		return buf.String()
	}
	buf.WriteString("\n}\n")

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

func preprocessHTMLBlocks(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result []string
	inHTMLBlock := false
	blockDepth := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for opening HTML tags
		if strings.HasPrefix(trimmedLine, "<") && !strings.HasPrefix(trimmedLine, "</") &&
			!strings.HasSuffix(trimmedLine, "/>") {
			// Simple HTML block detection (could be improved)
			if strings.Contains(trimmedLine, "<div") || strings.Contains(trimmedLine, "<dl") ||
				strings.Contains(trimmedLine, "<table") || strings.Contains(trimmedLine, "<section") {
				inHTMLBlock = true
				blockDepth++
			}
		}

		// Check for closing HTML tags
		if strings.HasPrefix(trimmedLine, "</") {
			if strings.Contains(trimmedLine, "</div>") || strings.Contains(trimmedLine, "</dl>") ||
				strings.Contains(trimmedLine, "</table>") || strings.Contains(trimmedLine, "</section>") {
				blockDepth--
				if blockDepth <= 0 {
					inHTMLBlock = false
					blockDepth = 0
				}
			}
		}

		// Remove indentation inside HTML blocks
		if inHTMLBlock {
			result = append(result, trimmedLine)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func markdownToHTML(markdown string) string {
	return markdownToHTMLWithContext(markdown, "")
}

func markdownToHTMLWithContext(markdown, filePath string) string {
	// Convert GitHub alerts to admonitions format
	markdown = convertGitHubAlerts(markdown)

	// Preprocess HTML blocks when unsafe HTML is allowed
	if *flagAllowUnsafe {
		markdown = preprocessHTMLBlocks(markdown)
	}

	// Core secure extensions
	extensions := []goldmark.Extender{
		extension.GFM, // GitHub Flavored Markdown
		extension.Footnote,
		meta.Meta, // YAML frontmatter support
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(false),
				chromahtml.WithClasses(true),
			),
		),
		&admonitions.Extender{},
	}

	// Add TOC if requested
	if *flagTOC {
		extensions = append(extensions, &toc.Extender{
			MinDepth: 1,
			MaxDepth: 6,
			Compact:  false,
		})
	}

	// Build markdown processor
	md := goldmark.New(
		goldmark.WithExtensions(extensions...),
		func() goldmark.Option {
			if *flagAllowUnsafe {
				return goldmark.WithParserOptions(
					parser.WithAutoHeadingID(),
					parser.WithAttribute(),
				)
			}
			return goldmark.WithParserOptions(parser.WithAutoHeadingID())
		}(),
		func() goldmark.Option {
			if *flagAllowUnsafe {
				return goldmark.WithRendererOptions(
					html.WithXHTML(),
					html.WithUnsafe(),
					html.WithHardWraps(),
				)
			}
			return goldmark.WithRendererOptions(html.WithXHTML())
		}(),
	)

	source := []byte(markdown)

	// If we have a file path, parse and modify links
	if filePath != "" {
		doc := md.Parser().Parse(text.NewReader(source))

		// Walk through AST and modify relative links
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}

			if link, ok := n.(*ast.Link); ok {
				href := string(link.Destination)

				// Skip external links and anchors
				if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
					strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") ||
					strings.HasPrefix(href, "tel:") {
					return ast.WalkContinue, nil
				}

				if strings.HasPrefix(href, "/") {
					// Absolute path - keep as is
					link.Destination = []byte(href)
				} else if strings.HasSuffix(href, ".md") || strings.HasSuffix(href, ".markdown") {
					// Relative markdown path - convert to clean URL
					filename := filepath.Base(href)
					urlPath := "/" + strings.TrimSuffix(strings.TrimSuffix(filename, ".md"), ".markdown")
					link.Destination = []byte(urlPath)
				}
			}
			return ast.WalkContinue, nil
		})

		// Render the modified AST
		var buf bytes.Buffer
		if err := md.Renderer().Render(&buf, source, doc); err != nil {
			return fmt.Sprintf("<p>Error rendering markdown: %v</p>", err)
		}
		return buf.String()
	}

	// Simple conversion without link modification
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return fmt.Sprintf("<p>Error rendering markdown: %v</p>", err)
	}
	return buf.String()
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

	// Check for index.md and include its content if present
	indexFile := filepath.Join(dir, "index.md")
	if _, err := os.Stat(indexFile); err == nil {
		content, err := os.ReadFile(indexFile)
		if err == nil {
			buf.WriteString(string(content))
			buf.WriteString("\n\n---\n\n")
		}
	}

	buf.WriteString(fmt.Sprintf("# Directory Listing: %s\n\n", dir))

	if len(files) == 0 {
		buf.WriteString("*No markdown files found in this directory.*\n")
		return buf.String(), nil
	}

	for _, file := range files {
		// Convert file path to clean URL
		cleanURL := "/" + file.RelPath
		if strings.HasSuffix(cleanURL, ".md") {
			cleanURL = strings.TrimSuffix(cleanURL, ".md")
		}
		if strings.HasSuffix(cleanURL, ".markdown") {
			cleanURL = strings.TrimSuffix(cleanURL, ".markdown")
		}

		buf.WriteString(fmt.Sprintf("- [%s](%s) (%d bytes, modified %s)\n",
			file.RelPath, cleanURL, file.Size, file.ModTime.Format("2006-01-02 15:04:05")))
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


func renderDocument(doc DocumentData, title, customCSS, filePath string) string {
	content := doc.Content
	if *flagRenderFrontmatter && len(doc.Frontmatter) > 0 {
		if frontmatterYAML, err := yaml.Marshal(doc.Frontmatter); err == nil {
			content = "```yaml\n" + string(frontmatterYAML) + "```\n\n" + content
		}
	}

	var html string
	if filePath != "" {
		html = markdownToHTMLWithContext(content, filePath)
	} else {
		html = markdownToHTML(content)
	}

	return renderTemplate(html, title, customCSS, true, doc.Frontmatter)
}

func loadAllTemplates() (*template.Template, error) {
	// Create template with helper functions
	// Start with embedded templates using ParseFS
	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil {
				return defaultValue
			}
			if s, ok := value.(string); ok && s == "" {
				return defaultValue
			}
			return value
		},
		"loadJSON": func(filename string) interface{} {
			data, err := loadJSONFile(filename)
			if err != nil {
				log.Printf("Error loading JSON file %s: %v", filename, err)
				return nil
			}
			return data
		},
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
	}).ParseFS(templates, "templates/*.html", "templates/*/*.html")

	if err != nil {
		log.Printf("Error parsing embedded templates: %v", err)
		tmpl = template.New("root") // fallback to empty template
	}

	// Load custom templates (override embedded ones)
	if *flagTemplateDir != "" {
		customPattern1 := filepath.Join(*flagTemplateDir, "*.html")
		customPattern2 := filepath.Join(*flagTemplateDir, "*", "*.html")
		if customTmpl, err := tmpl.ParseGlob(customPattern1); err == nil {
			tmpl = customTmpl
		}
		if customTmpl, err := tmpl.ParseGlob(customPattern2); err == nil {
			tmpl = customTmpl
		}
	}

	return tmpl, nil
}

func renderTemplate(htmlContent, title, customCSS string, liveReload bool, frontmatter map[string]interface{}) string {
	// Load all templates
	tmpl, err := loadAllTemplates()
	if err != nil {
		log.Printf("Error loading templates: %v", err)
		return fmt.Sprintf("<p>Template loading error: %v</p>", err)
	}

	// Use "layout" as the standard template name
	templateName := "layout"

	// If layout doesn't exist, fall back to other common names
	if tmpl.Lookup(templateName) == nil {
		// Try other standard names
		for _, name := range []string{"docs.html", "page.html", "live-reload.html"} {
			if tmpl.Lookup(name) != nil {
				templateName = name
				break
			}
		}
	}

	// If still no template found, error out
	if tmpl.Lookup(templateName) == nil {
		return "<p>No template found. Expected 'layout' template"
	}

	var buf bytes.Buffer
	data := struct {
		Title       string
		Content     template.HTML
		CustomCSS   template.CSS
		ChromaCSS   template.CSS
		Verbose     bool
		LiveReload  bool
		Frontmatter map[string]interface{}
	}{
		Title:       title,
		Content:     template.HTML(htmlContent),
		CustomCSS:   template.CSS(customCSS),
		ChromaCSS:   template.CSS(generateChromaCSS()),
		Verbose:     *flagVerbose,
		LiveReload:  liveReload,
		Frontmatter: frontmatter,
	}

	// Try to execute the selected template, fall back to base if it exists
	err = tmpl.ExecuteTemplate(&buf, templateName, data)
	if err != nil {
		// Try base template as fallback
		if tmpl.Lookup("base") != nil {
			err = tmpl.ExecuteTemplate(&buf, "base", data)
		}
		if err != nil {
			log.Printf("Error executing template: %v", err)
			return fmt.Sprintf("<p>Template execution error: %v</p>", err)
		}
	}
	return buf.String()
}

