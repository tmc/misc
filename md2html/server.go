package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

func newServer(cfg Config, logger *slog.Logger) *server {
	s := &server{
		config:     cfg,
		logger:     logger,
		clients:    make(map[chan string]bool),
		inputPath:  cfg.Source,
		shutdownCh: make(chan struct{}),
	}

	// Load JSON data if provided
	if cfg.DataJSON != "" {
		jsonData, err := loadJSONFile(cfg.DataJSON)
		if err != nil {
			logger.Error("Error loading JSON data file", "error", err, "file", cfg.DataJSON)
		} else {
			s.jsonData = jsonData
			logger.Debug("Loaded JSON data", "file", cfg.DataJSON)
		}
	}

	// Load initial content
	if cfg.Source != "" && cfg.Source != "-" {
		content, err := os.ReadFile(cfg.Source)
		if err != nil {
			logger.Error("Error reading initial file", "error", err, "file", cfg.Source)
		} else {
			s.mu.Lock()
			s.content = string(content)
			s.mu.Unlock()
			logger.Debug("Loaded initial content", "file", cfg.Source)
		}
	}

	// Load CSS if provided
	if cfg.CSS != "" {
		css, err := os.ReadFile(cfg.CSS)
		if err != nil {
			logger.Error("Error reading CSS file", "error", err, "file", cfg.CSS)
		} else {
			s.cssContent = string(css)
			logger.Debug("Loaded CSS content", "file", cfg.CSS)
		}
	}

	return s
}

type server struct {
	config     Config
	logger     *slog.Logger
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

func (s *server) watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch the directory containing the markdown file (handles atomic replacements)
	dir := filepath.Dir(s.inputPath)
	if dir == "" {
		dir = "."
	}
	err = watcher.Add(dir)
	if err != nil {
		log.Printf("Error watching directory: %v", err)
		return
	}

	// Watch CSS file directory if provided
	if s.config.CSS != "" {
		cssDir := filepath.Dir(s.config.CSS)
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
				inputFile := filepath.Base(s.inputPath)

				if eventFile == inputFile {
					if s.config.Verbose {
						log.Printf("Detected change to %s, updating content", inputFile)
					}
					content, err := os.ReadFile(s.inputPath)
					if err != nil {
						log.Printf("Error reading file: %v", err)
						continue
					}
					s.mu.Lock()
					s.content = string(content)
					s.mu.Unlock()
					s.notifyClients()
				} else if s.config.CSS != "" && eventFile == filepath.Base(s.config.CSS) {
					css, err := os.ReadFile(s.config.CSS)
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
	if r.URL.Path == "/" && s.config.Index != "" {
		if fileContent, err := os.ReadFile(s.config.Index); err == nil {
			doc, err := parseFrontmatter(string(fileContent))
			if err != nil {
				log.Printf("Error parsing frontmatter in %s: %v", s.config.Index, err)
				doc = DocumentData{Content: string(fileContent), Frontmatter: make(map[string]interface{})}
			}
			html := renderDocument(s.config, doc, s.config.Index, css, s.config.Index)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		} else if s.config.Verbose {
			log.Printf("Index file %s not found, falling back to directory listing", s.config.Index)
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
				html := renderDocument(s.config, doc, candidate, css, candidate)
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
		html := renderDocument(s.config, doc, file, css, file)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	// If no input file specified and content is empty, show directory listing
	if s.inputPath == "" && content == "" {
		wd, err := os.Getwd()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting working directory: %v", err), http.StatusInternalServerError)
			return
		}

		listing, err := generateDirectoryListing(s.config, wd)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error generating directory listing: %v", err), http.StatusInternalServerError)
			return
		}

		doc := DocumentData{Content: listing, Frontmatter: make(map[string]interface{})}
		html := renderDocument(s.config, doc, "Directory Listing", css, wd)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	doc, err := parseFrontmatter(content)
	if err != nil {
		log.Printf("Error parsing frontmatter: %v", err)
		doc = DocumentData{Content: content, Frontmatter: make(map[string]interface{})}
	}
	html := renderDocument(s.config, doc, s.config.Title, css, "")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	content := s.content
	s.mu.RUnlock()

	html := markdownToHTMLWithContext(s.config, content, "")
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
			} else if s.config.Verbose {
				log.Printf("Watching directory: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error setting up recursive directory watching: %v", err)
		return
	}

	if s.config.Verbose {
		log.Printf("Watching current directory for .md file changes")
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if s.config.Verbose {
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

					if s.config.Verbose {
						log.Printf("Markdown file changed: %s %s - notifying clients", event.Op, event.Name)
					}
					s.notifyClients()
				} else if s.config.Verbose {
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
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	if s.config.Verbose {
		log.Printf("Notifying %d clients", len(s.clients))
	}

	for ch := range s.clients {
		select {
		case ch <- "reload":
		default:
		}
	}
}

// Run starts the server and handles graceful shutdown
func (s *server) Run(ctx context.Context) error {
	// Set up file watching
	if s.config.Source != "" && s.config.Source != "-" {
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

	srv := &http.Server{
		Addr:    s.config.HTTP,
		Handler: mux,
	}

	// Format URL for display and browser opening
	displayURL := formatServerURL(s.config.HTTP)

	// Open browser if requested
	if s.config.Open {
		go func() {
			if !openBrowser(displayURL) {
				s.logger.Warn("Failed to open browser", "url", displayURL)
			} else {
				s.logger.Debug("Opened browser", "url", displayURL)
			}
		}()
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()
		// Close shutdown channel to notify all goroutines
		close(s.shutdownCh)
		// Send shutdown signal to all clients
		s.clientsMu.Lock()
		for client := range s.clients {
			select {
			case client <- "shutdown":
			default:
			}
		}
		s.clients = make(map[chan string]bool)
		s.clientsMu.Unlock()

		// Shutdown server with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("Server shutdown error", "error", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		s.logger.Error("Server error", "error", err)
		return fmt.Errorf("server error: %v", err)
	}

	<-idleConnsClosed
	return nil
}
