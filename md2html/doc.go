// md2html is a comprehensive markdown-to-HTML converter with live preview capabilities.
//
// It provides a local web server that renders markdown files with GitHub-compatible
// styling and features, including live reload functionality for real-time preview
// of changes.
//
// # Features
//
//   - GitHub-compatible markdown rendering using Goldmark
//   - Live reload via Server-Sent Events (SSE) for instant preview updates
//   - GitHub-style alerts/admonitions (NOTE, TIP, IMPORTANT, WARNING, CAUTION)
//   - Automatic Table of Contents (TOC) generation
//   - Syntax highlighting with GitHub theme via Chroma
//   - MathJax support for mathematical expressions
//   - Mermaid diagram support
//   - Directory browsing with recursive file watching
//   - GitHub Flavored Markdown (GFM) extensions
//   - Task lists, tables, footnotes, and strikethrough support
//
// # Usage
//
// Basic usage with a specific markdown file:
//
//	md2html -input README.md
//
// Start server on a custom port:
//
//	md2html -input doc.md -http :3000
//
// Enable verbose logging:
//
//	md2html -input notes.md -v
//
// Automatically open browser:
//
//	md2html -input README.md -open
//
// Directory browsing mode (no input file specified):
//
//	md2html
//
// # Command Line Options
//
//	-input string
//	      Input markdown file (default: directory listing mode)
//	-http string
//	      HTTP server bind address (default ":8080")
//	-open
//	      Automatically open browser on startup
//	-title string
//	      HTML page title (default "Markdown Preview")
//	-toc
//	      Generate table of contents (default true)
//	-css string
//	      Path to custom CSS file for additional styling
//	-depth int
//	      Directory traversal depth for listings (default 2, minimum 2)
//	-v
//	      Enable verbose logging for debugging
//
// # GitHub Alerts
//
// md2html supports GitHub-style alerts using the following syntax:
//
//	> [!NOTE]
//	> Useful information that users should know.
//
//	> [!TIP]
//	> Helpful advice for doing things better.
//
//	> [!IMPORTANT]
//	> Key information users need to know.
//
//	> [!WARNING]
//	> Urgent info that needs immediate attention.
//
//	> [!CAUTION]
//	> Advises about risks or negative outcomes.
//
// These are automatically rendered with GitHub's color scheme and styling.
//
// # Live Reload
//
// The server watches for changes to markdown files in the current directory
// and all subdirectories. When a markdown file is modified, all connected
// browsers automatically reload to show the updated content.
//
// # Mathematical Expressions
//
// Inline math: $E = mc^2$
//
// Display math:
// $$
// \int_{-\infty}^{\infty} e^{-x^2} dx = \sqrt{\pi}
// $$
//
// # Mermaid Diagrams
//
// Create diagrams using Mermaid syntax:
//
// ```mermaid
// graph TD
//
//	A[Start] --> B{Decision}
//	B -->|Yes| C[Continue]
//	B -->|No| D[Stop]
//
// ```
//
// # Security
//
// md2html uses Goldmark's secure defaults and does not allow unsafe HTML
// by default. All rendering is done server-side with proper HTML escaping.
//
//go:generate gocmddoc -o README.md
package main
