package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	admonitions "github.com/stefanfritsch/goldmark-admonitions"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"
)

//go:embed templates
var templates embed.FS

func generateChromaCSS() string {
	lightStyle := styles.Get("github")
	if lightStyle == nil {
		lightStyle = styles.Fallback
	}

	darkStyle := styles.Get("github-dark")
	if darkStyle == nil {
		if darkStyle = styles.Get("dracula"); darkStyle == nil {
			darkStyle = lightStyle
		}
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithLineNumbers(false),
	)

	var buf bytes.Buffer

	// Write dark theme CSS with media query
	buf.WriteString("@media (prefers-color-scheme: dark) {\n")
	if err := formatter.WriteCSS(&buf, darkStyle); err != nil {
		return ""
	}
	buf.WriteString("\n}\n")

	// Write light theme CSS with media query (and as default)
	buf.WriteString("\n@media (prefers-color-scheme: light), (prefers-color-scheme: no-preference) {\n")
	if err := formatter.WriteCSS(&buf, lightStyle); err != nil {
		return ""
	}
	buf.WriteString("\n}\n")

	return buf.String()
}

func convertGitHubAlerts(markdown string) string {
	alerts := map[string]string{
		"[!NOTE]":      "note",
		"[!TIP]":       "tip",
		"[!IMPORTANT]": "important",
		"[!WARNING]":   "warning",
		"[!CAUTION]":   "danger",
	}

	lines := strings.Split(markdown, "\n")
	var result []string
	var alert []string
	var alertType string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "> ") {
			if alert != nil {
				alert = append(alert, "!!!")
				result = append(result, alert...)
				alert = nil
			}
			result = append(result, line)
			continue
		}

		content := strings.TrimPrefix(trimmed, "> ")
		if alert == nil {
			for gh, adm := range alerts {
				if strings.HasPrefix(content, gh) {
					alertType = adm
					title := strings.TrimSpace(strings.TrimPrefix(content, gh))
					if title == "" {
						title = strings.Title(alertType)
					}
					alert = []string{"!!!" + alertType + " " + title}
					break
				}
			}
			if alert == nil {
				result = append(result, line)
			}
		} else {
			alert = append(alert, content)
		}
	}

	if alert != nil {
		alert = append(alert, "!!!")
		result = append(result, alert...)
	}

	return strings.Join(result, "\n")
}

func preprocessHTMLBlocks(markdown string) string {
	blockTags := []string{"<div", "<dl", "<table", "<section"}
	lines := strings.Split(markdown, "\n")
	var result []string
	depth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, tag := range blockTags {
			if strings.Contains(trimmed, tag) {
				depth++
			}
			if strings.Contains(trimmed, "</"+tag[1:]+">") {
				depth--
			}
		}
		if depth > 0 {
			result = append(result, trimmed)
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func markdownToHTMLWithContext(cfg Config, markdown, filePath string) string {
	markdown = convertGitHubAlerts(markdown)
	if cfg.AllowUnsafe {
		markdown = preprocessHTMLBlocks(markdown)
	}

	extensions := []goldmark.Extender{
		extension.GFM,
		extension.Footnote,
		meta.Meta,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(false),
				chromahtml.WithClasses(true),
			),
		),
		&admonitions.Extender{},
	}
	if cfg.TOC {
		extensions = append(extensions, &toc.Extender{
			MinDepth: 1,
			MaxDepth: 6,
		})
	}

	parserOpts := []parser.Option{parser.WithAutoHeadingID()}
	if cfg.AllowUnsafe {
		parserOpts = append(parserOpts, parser.WithAttribute())
	}

	rendererOpts := []renderer.Option{html.WithXHTML(), html.WithHardWraps()}
	if cfg.AllowUnsafe {
		rendererOpts = append(rendererOpts, html.WithUnsafe())
	}

	md := goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(parserOpts...),
		goldmark.WithRendererOptions(rendererOpts...),
	)

	source := []byte(markdown)

	if filePath != "" {
		doc := md.Parser().Parse(text.NewReader(source))
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			link, ok := n.(*ast.Link)
			if !ok {
				return ast.WalkContinue, nil
			}
			href := string(link.Destination)
			if strings.Contains(href, "://") || strings.HasPrefix(href, "#") ||
				strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") ||
				strings.HasPrefix(href, "/") {
				return ast.WalkContinue, nil
			}
			if ext := filepath.Ext(href); ext == ".md" || ext == ".markdown" {
				base := strings.TrimSuffix(filepath.Base(href), ext)
				// Add extension based on flag setting
				if cfg.HTMLExt != "" {
					link.Destination = []byte("/" + base + "." + cfg.HTMLExt)
				} else {
					link.Destination = []byte("/" + base)
				}
			}
			return ast.WalkContinue, nil
		})
		var buf bytes.Buffer
		if err := md.Renderer().Render(&buf, source, doc); err != nil {
			return fmt.Sprintf("<p>Error: %v</p>", err)
		}
		return buf.String()
	}

	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return fmt.Sprintf("<p>Error: %v</p>", err)
	}
	return buf.String()
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
