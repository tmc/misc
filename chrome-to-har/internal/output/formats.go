// Package output handles different output formats for Chrome data.
package output

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// Format defines supported output formats
type Format string

const (
	// FormatHTML outputs raw HTML
	FormatHTML Format = "html"

	// FormatText outputs plain text extracted from HTML
	FormatText Format = "text"

	// FormatJSON outputs JSON representation of the page
	FormatJSON Format = "json"

	// FormatHAR outputs HTTP Archive (HAR) format
	FormatHAR Format = "har"
)

// PageData represents captured data from a page
type PageData struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// WriteHTML writes HTML content to the writer
func WriteHTML(w io.Writer, content string) error {
	_, err := io.WriteString(w, content)
	return err
}

// WriteText extracts and writes text content from HTML
func WriteText(w io.Writer, html string) error {
	// Simple text extraction
	// This is a naive implementation. A real one would use a proper HTML parser
	text := strings.ReplaceAll(html, "\n", " ")
	text = strings.ReplaceAll(text, "<script", "\n<script")
	text = strings.ReplaceAll(text, "</script>", "</script>\n")
	text = strings.ReplaceAll(text, "<style", "\n<style")
	text = strings.ReplaceAll(text, "</style>", "</style>\n")
	text = strings.ReplaceAll(text, "<", "\n<")

	// Extract text nodes
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "<") {
			content := strings.TrimSpace(line)
			if content != "" {
				sb.WriteString(content)
				sb.WriteString("\n")
			}
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// WriteMarkdown converts HTML to Markdown and writes it
func WriteMarkdown(w io.Writer, html string) error {
	// This is a placeholder. A real implementation would use a proper HTML to MD converter
	// For now, we'll just use a very basic extraction
	return WriteText(w, html)
}

// WriteJSON serializes the page data as JSON and writes it
func WriteJSON(w io.Writer, data PageData) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Write outputs content in the specified format
func Write(w io.Writer, format Format, data PageData) error {
	switch format {
	case FormatHTML:
		return WriteHTML(w, data.Content)
	case FormatText:
		return WriteText(w, data.Content)
	case FormatJSON:
		return WriteJSON(w, data)
	default:
		return errors.Errorf("unsupported output format: %s", format)
	}
}
