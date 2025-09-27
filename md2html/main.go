package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	flags = flag.NewFlagSet("md2html", flag.ExitOnError)

	flagHTTP              = flags.String("http", "", "HTTP server bind address")
	flagHTML              = flags.String("html", "", "output directory for static HTML generation (disables server mode)")
	flagOpen              = flags.Bool("open", false, "automatically open browser")
	flagVerbose           = flags.Bool("v", false, "verbose logging")
	flagTitle             = flags.String("title", "Markdown Preview", "HTML title")
	flagCSS               = flags.String("css", "", "path to custom CSS file")
	flagDepth             = flags.Int("depth", 2, "directory traversal depth for listings (minimum 2)")
	flagTOC               = flags.Bool("toc", false, "generate table of contents")
	flagAllowUnsafe       = flags.Bool("allow-unsafe", false, "allow unsafe HTML in markdown (use with caution)")
	flagTemplateDir       = flags.String("templates", "", "path to custom template directory (overrides embedded templates)")
	flagDataJSON          = flags.String("data-json", "", "path to JSON file to load as template data (available as .Data)")
	flagRenderFrontmatter = flags.Bool("render-frontmatter", false, "render YAML frontmatter as part of the document content")
	flagIndex             = flags.String("index", "", "default file to serve for root path (e.g., README.md, index.md)")
	flagHTMLExt           = flags.String("html-ext", "", "file extension for generated HTML files (e.g., 'html' for .html, empty for no extension except index.html)")
)

type Config struct {
	Source            string // file, directory, or "-" for stdin
	HTTP              string
	HTML              string
	Open              bool
	Verbose           bool
	Title             string
	CSS               string
	Depth             int
	TOC               bool
	AllowUnsafe       bool
	TemplateDir       string
	DataJSON          string
	RenderFrontmatter bool
	Index             string
	HTMLExt           string
}

// configFromFlags creates a Config from current global flag values
func configFromFlags(fs *flag.FlagSet) Config {
	return Config{
		HTTP:              fs.Lookup("http").Value.String(),
		HTML:              fs.Lookup("html").Value.String(),
		Open:              fs.Lookup("open").Value.String() == "true",
		Verbose:           fs.Lookup("v").Value.String() == "true",
		Title:             fs.Lookup("title").Value.String(),
		CSS:               fs.Lookup("css").Value.String(),
		Depth:             int(fs.Lookup("depth").Value.(flag.Getter).Get().(int)),
		TOC:               fs.Lookup("toc").Value.String() == "true",
		AllowUnsafe:       fs.Lookup("allow-unsafe").Value.String() == "true",
		TemplateDir:       fs.Lookup("templates").Value.String(),
		DataJSON:          fs.Lookup("data-json").Value.String(),
		RenderFrontmatter: fs.Lookup("render-frontmatter").Value.String() == "true",
		Index:             fs.Lookup("index").Value.String(),
		HTMLExt:           fs.Lookup("html-ext").Value.String(),
	}
}

func main() {
	flags.Parse(os.Args[1:])
	ctx := context.Background()
	logger := slog.Default()
	cfg := configFromFlags(flags)
	if err := run(ctx, cfg, logger, os.Stdout, flags.Args()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, cfg Config, logger *slog.Logger, out io.Writer, args []string) error {
	// Set up signal handling
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Handle positional arguments
	if len(args) > 0 {
		cfg.Source = args[0]
	}
	if len(args) > 1 {
		return fmt.Errorf("too many positional arguments")
	}

	// Configure logger level based on verbose flag
	if cfg.Verbose {
		// Create a new logger with debug level when verbose is enabled
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
		handler := slog.NewTextHandler(os.Stderr, opts)
		logger = slog.New(handler)
	}

	// TODO: clean up handling stdin and choosing between modes

	// If -html flag is provided, generate static HTML
	if cfg.HTML != "" {
		return generateStaticHTML(ctx, cfg, logger)
	}

	// If -http flag is provided, run server
	if cfg.HTTP != "" {
		logger.Info("Starting server", "address", cfg.HTTP)
		err := runServer(ctx, cfg, logger)
		// Don't treat context cancellation as an error (graceful shutdown)
		if err == context.Canceled {
			return nil
		}
		return err
	}

	// If source is provided but no mode specified, convert to HTML and output to stdout
	if cfg.Source != "" && cfg.Source != "." {
		// Read the markdown file
		content, err := os.ReadFile(cfg.Source)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		// Convert to HTML
		doc, err := parseFrontmatter(string(content))
		if err != nil {
			logger.Error("Error parsing frontmatter", "error", err)
			doc = DocumentData{Content: string(content), Frontmatter: make(map[string]interface{})}
		}

		html := markdownToHTMLWithContext(cfg, doc.Content, cfg.Source)
		fmt.Fprint(out, html)
		return nil
	}

	// Neither -html nor -http provided and no source, show usage
	flag.Usage()
	return flag.ErrHelp
}

func runServer(ctx context.Context, cfg Config, logger *slog.Logger) error {
	s := newServer(cfg, logger)
	return s.Run(ctx)
}

func formatServerURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	if !strings.Contains(addr, "://") {
		return "http://" + addr
	}
	return addr
}

func generateDirectoryListing(cfg Config, dir string) (string, error) {
	files, err := findMarkdownFiles(dir, cfg.Depth)
	if err != nil {
		return "", err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})

	var buf strings.Builder

	if content, err := os.ReadFile(filepath.Join(dir, "index.md")); err == nil {
		buf.WriteString(string(content) + "\n\n---\n\n")
	}

	buf.WriteString(fmt.Sprintf("# Directory Listing: %s\n\n", dir))

	if len(files) == 0 {
		buf.WriteString("*No markdown files found.*\n")
		return buf.String(), nil
	}

	for _, f := range files {
		url := "/" + f.RelPath
		url = strings.TrimSuffix(url, ".md")
		url = strings.TrimSuffix(url, ".markdown")
		if cfg.HTMLExt != "" {
			url += "." + cfg.HTMLExt
		}
		buf.WriteString(fmt.Sprintf("- [%s](%s) (%d bytes, %s)\n",
			f.RelPath, url, f.Size, f.ModTime.Format("2006-01-02 15:04")))
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
	// Skip browser opening if environment variable is set (useful for tests)
	if os.Getenv("MD2HTML_NO_BROWSER") != "" {
		return true
	}

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

func renderDocument(cfg Config, doc DocumentData, title, customCSS, filePath string) string {
	content := doc.Content
	if cfg.RenderFrontmatter && len(doc.Frontmatter) > 0 {
		if frontmatterYAML, err := yaml.Marshal(doc.Frontmatter); err == nil {
			content = "```yaml\n" + string(frontmatterYAML) + "```\n\n" + content
		}
	}

	html := markdownToHTMLWithContext(cfg, content, filePath)
	return renderTemplate(cfg, html, title, customCSS, true, doc.Frontmatter)
}

func loadAllTemplates(cfg Config) (*template.Template, error) {
	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"default": func(def, val interface{}) interface{} {
			if val == nil {
				return def
			}
			if s, ok := val.(string); ok && s == "" {
				return def
			}
			return val
		},
		"loadJSON": func(filename string) interface{} {
			data, err := loadJSONFile(filename)
			if err != nil {
				log.Printf("Error loading JSON %s: %v", filename, err)
				return nil
			}
			return data
		},
		"replace": strings.ReplaceAll,
	}).ParseFS(templates, "templates/*.html", "templates/*/*.html")

	if err != nil {
		log.Printf("Error parsing embedded templates: %v", err)
		tmpl = template.New("root")
	}

	if cfg.TemplateDir != "" {
		for _, pattern := range []string{"*.html", "*/*.html"} {
			if t, err := tmpl.ParseGlob(filepath.Join(cfg.TemplateDir, pattern)); err == nil {
				tmpl = t
			}
		}
	}

	return tmpl, nil
}

func renderTemplate(cfg Config, htmlContent, title, customCSS string, liveReload bool, frontmatter map[string]interface{}) string {
	tmpl, err := loadAllTemplates(cfg)
	if err != nil {
		log.Printf("Error loading templates: %v", err)
		return fmt.Sprintf("<p>Template loading error: %v</p>", err)
	}

	name := "layout"
	if tmpl.Lookup(name) == nil {
		for _, n := range []string{"docs.html", "page.html", "live-reload.html", "base"} {
			if tmpl.Lookup(n) != nil {
				name = n
				break
			}
		}
	}

	if tmpl.Lookup(name) == nil {
		return "<p>No template found. Expected 'layout' template"
	}

	var buf bytes.Buffer
	// Prepare extension with dot prefix for template use
	htmlExt := ""
	if cfg.HTMLExt != "" {
		htmlExt = "." + cfg.HTMLExt
	}

	data := struct {
		Title       string
		Content     template.HTML
		CustomCSS   template.CSS
		ChromaCSS   template.CSS
		Verbose     bool
		LiveReload  bool
		HTMLExt     string
		Frontmatter map[string]interface{}
	}{
		Title:       title,
		Content:     template.HTML(htmlContent),
		CustomCSS:   template.CSS(customCSS),
		ChromaCSS:   template.CSS(generateChromaCSS()),
		Verbose:     cfg.Verbose,
		LiveReload:  liveReload,
		HTMLExt:     htmlExt,
		Frontmatter: frontmatter,
	}

	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("Error executing template: %v", err)
		return fmt.Sprintf("<p>Template execution error: %v</p>", err)
	}
	return buf.String()
}

func generateStaticHTML(ctx context.Context, cfg Config, logger *slog.Logger) error {
	sourceDir := cfg.Source
	if sourceDir == "" {
		sourceDir = "."
	}

	outputDir := cfg.HTML

	logger.Info("Generating static HTML", "source", sourceDir, "output", outputDir)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Load JSON data if provided
	if cfg.DataJSON != "" {
		_, err := loadJSONFile(cfg.DataJSON)
		if err != nil {
			logger.Error("Error loading JSON data file", "error", err, "file", cfg.DataJSON)
		} else {
			logger.Debug("Loaded JSON data", "file", cfg.DataJSON)
		}
	}

	// Load CSS if provided
	var cssContent string
	if cfg.CSS != "" {
		css, err := os.ReadFile(cfg.CSS)
		if err != nil {
			logger.Error("Error reading CSS file", "error", err, "file", cfg.CSS)
		} else {
			cssContent = string(css)
			logger.Debug("Loaded CSS content", "file", cfg.CSS)
		}
	}

	// Find all markdown files
	files, err := findMarkdownFiles(sourceDir, 100) // Use high depth for static generation
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %v", err)
	}

	logger.Info("Found markdown files to process", "count", len(files))

	// Process each markdown file
	for _, file := range files {
		if err := processMarkdownFile(file, sourceDir, outputDir, cssContent, cfg); err != nil {
			logger.Error("Error processing file", "error", err, "file", file.RelPath)
			continue
		}
		logger.Debug("Generated file", "file", file.RelPath)
	}

	// Handle index file if specified
	if cfg.Index != "" {
		indexFile := filepath.Join(sourceDir, cfg.Index)
		if _, err := os.Stat(indexFile); err == nil {
			if err := processIndexFile(indexFile, outputDir, cssContent, cfg); err != nil {
				logger.Error("Error processing index file", "error", err, "file", indexFile)
			} else {
				logger.Debug("Processed index file", "file", indexFile)
			}
		}
	} else {
		// Generate table of contents as index.html
		if err := generateTOCIndex(sourceDir, outputDir, files, cssContent, cfg); err != nil {
			logger.Error("Error generating TOC index", "error", err)
		} else {
			logger.Debug("Generated TOC index")
		}
	}

	logger.Info("Static HTML generation completed")

	return nil
}

func processMarkdownFile(file markdownFile, sourceDir, outputDir, cssContent string, cfg Config) error {
	sourcePath := filepath.Join(sourceDir, file.RelPath)

	// Read and parse the markdown file
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	doc, err := parseFrontmatter(string(content))
	if err != nil {
		log.Printf("Error parsing frontmatter in %s: %v", file.RelPath, err)
		doc = DocumentData{Content: string(content), Frontmatter: make(map[string]interface{})}
	}

	// Generate HTML content
	htmlContent := markdownToHTMLWithContext(cfg, doc.Content, file.RelPath)

	// Determine output file path
	baseName := strings.TrimSuffix(file.RelPath, filepath.Ext(file.RelPath))
	outputPath := baseName
	// When HTMLExt is empty, only index gets .html extension
	// When HTMLExt is set, all files get that extension
	if cfg.HTMLExt != "" {
		outputPath = baseName + "." + cfg.HTMLExt
	}
	outputPath = filepath.Join(outputDir, outputPath)

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	// Render with template
	title := cfg.Title
	if docTitle, ok := doc.Frontmatter["title"].(string); ok && docTitle != "" {
		title = docTitle
	} else {
		title = strings.TrimSuffix(filepath.Base(file.RelPath), filepath.Ext(file.RelPath))
	}

	finalHTML := renderTemplate(cfg, htmlContent, title, cssContent, false, doc.Frontmatter)

	// Write output file
	return os.WriteFile(outputPath, []byte(finalHTML), 0644)
}

func processIndexFile(indexPath, outputDir, cssContent string, cfg Config) error {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	doc, err := parseFrontmatter(string(content))
	if err != nil {
		log.Printf("Error parsing frontmatter in index file: %v", err)
		doc = DocumentData{Content: string(content), Frontmatter: make(map[string]interface{})}
	}

	htmlContent := markdownToHTMLWithContext(cfg, doc.Content, filepath.Base(indexPath))

	title := cfg.Title
	if docTitle, ok := doc.Frontmatter["title"].(string); ok && docTitle != "" {
		title = docTitle
	}

	finalHTML := renderTemplate(cfg, htmlContent, title, cssContent, false, doc.Frontmatter)

	indexOutputPath := filepath.Join(outputDir, "index.html")
	return os.WriteFile(indexOutputPath, []byte(finalHTML), 0644)
}

func generateTOCIndex(sourceDir, outputDir string, files []markdownFile, cssContent string, cfg Config) error {
	// Generate table of contents markdown
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Directory Listing: %s\n\n", sourceDir))

	if len(files) == 0 {
		buf.WriteString("*No markdown files found.*\n")
	} else {
		for _, f := range files {
			// Generate URL with optional extension based on config
			url := "/" + strings.TrimSuffix(f.RelPath, filepath.Ext(f.RelPath))
			if cfg.HTMLExt != "" {
				url += "." + cfg.HTMLExt
			}
			buf.WriteString(fmt.Sprintf("- [%s](%s) (%d bytes, %s)\n",
				f.RelPath, url, f.Size, f.ModTime.Format("2006-01-02 15:04")))
		}
	}

	// Convert to HTML
	htmlContent := markdownToHTMLWithContext(cfg, buf.String(), "")

	// Render with template
	doc := DocumentData{Content: buf.String(), Frontmatter: make(map[string]interface{})}
	finalHTML := renderTemplate(cfg, htmlContent, "Directory Listing", cssContent, false, doc.Frontmatter)

	// Write index.html
	indexOutputPath := filepath.Join(outputDir, "index.html")
	return os.WriteFile(indexOutputPath, []byte(finalHTML), 0644)
}
