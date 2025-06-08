package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

func main() {
	output := flag.String("o", "", "Output file path (default: stdout)")
	allDecl := flag.Bool("all", false, "Include all declarations for main packages")
	toc := flag.Bool("toc", false, "Generate table of contents")
	badge := flag.Bool("badge", true, "Add pkg.go.dev badge for library packages")
	install := flag.Bool("add-install-section", true, "Add installation instructions section")
	shields := flag.String("shields", "", "Add shields: all, version, license, build, report (comma-separated)")
	flag.StringVar(output, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(allDecl, "a", false, "Include all declarations for main packages")
	flag.Parse()

	pkgPath := "."
	if flag.NArg() > 0 {
		pkgPath = flag.Arg(0)
	}

	md, err := generateMarkdown(pkgPath, *allDecl, *toc, *badge, *install, *shields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Documentation written to %s\n", *output)
	} else {
		fmt.Print(md)
	}
}

func generateMarkdown(pkgPath string, showAll, genTOC, genBadge, genInstall bool, shieldsStr string) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse package: %w", err)
	}

	// Find first non-test package
	var pkg *ast.Package
	for name, p := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			pkg = p
			break
		}
	}
	if pkg == nil {
		return "", fmt.Errorf("no non-test packages found in %s", pkgPath)
	}

	docPkg := doc.New(pkg, pkgPath, doc.Mode(0))

	var out strings.Builder
	isMain := docPkg.Name == "main"

	// Title
	title := docPkg.Name
	if isMain {
		if abs, err := filepath.Abs(pkgPath); err == nil {
			title = filepath.Base(abs)
		}
	}
	fmt.Fprintf(&out, "# %s\n\n", title)

	// Determine import path for badges
	importPath := pkgPath
	if strings.HasPrefix(pkgPath, ".") || strings.HasPrefix(pkgPath, "/") {
		if modPath := getModulePath(pkgPath); modPath != "" {
			importPath = modPath
		}
	}
	isValidImportPath := strings.Contains(importPath, "/") && !strings.HasPrefix(importPath, ".") && !strings.HasPrefix(importPath, "/")

	// Add badges
	if isValidImportPath {
		var badges []string

		// pkg.go.dev badge
		if genBadge {
			badges = append(badges, fmt.Sprintf("[![Go Reference](https://pkg.go.dev/badge/%s.svg)](https://pkg.go.dev/%s)", importPath, importPath))
		}

		// Additional shields
		if shieldsStr != "" {
			// Extract GitHub path if applicable
			if strings.HasPrefix(importPath, "github.com/") {
				parts := strings.Split(importPath, "/")
				if len(parts) >= 3 {
					owner := parts[1]
					repo := parts[2]

					// Parse shields options
					shieldsList := strings.Split(shieldsStr, ",")
					shieldsMap := make(map[string]bool)
					for _, s := range shieldsList {
						shieldsMap[strings.TrimSpace(s)] = true
					}

					// Check if "all" is specified
					includeAll := shieldsMap["all"]

					// Go Version
					if includeAll || shieldsMap["version"] {
						badges = append(badges, fmt.Sprintf("[![Go Version](https://img.shields.io/github/go-mod/go-version/%s/%s)](go.mod)", owner, repo))
					}

					// License
					if includeAll || shieldsMap["license"] {
						badges = append(badges, fmt.Sprintf("[![License](https://img.shields.io/github/license/%s/%s)](LICENSE)", owner, repo))
					}

					// Build Status (GitHub Actions)
					if includeAll || shieldsMap["build"] {
						badges = append(badges, fmt.Sprintf("[![Build Status](https://img.shields.io/github/actions/workflow/status/%s/%s/test.yml?branch=main)](https://github.com/%s/%s/actions)", owner, repo, owner, repo))
					}

					// Go Report Card
					if includeAll || shieldsMap["report"] {
						badges = append(badges, fmt.Sprintf("[![Go Report Card](https://goreportcard.com/badge/%s)](https://goreportcard.com/report/%s)", importPath, importPath))
					}
				}
			}
		}

		if len(badges) > 0 {
			out.WriteString(strings.Join(badges, " "))
			out.WriteString("\n\n")
		}
	}

	// Collect headers for TOC
	var headers []header
	var content strings.Builder
	var packageDoc strings.Builder
	var installSection strings.Builder

	// Package doc (first)
	var beforeHeaders, afterHeaders string
	if docPkg.Doc != "" {
		beforeHeaders, afterHeaders = formatDocWithHeaders(docPkg.Doc, &headers)
		if beforeHeaders != "" {
			packageDoc.WriteString(beforeHeaders)
			packageDoc.WriteString("\n")
		}
	}

	// Installation section (collect separately)
	if genInstall {
		importPath := pkgPath
		if strings.HasPrefix(pkgPath, ".") || strings.HasPrefix(pkgPath, "/") {
			if modPath := getModulePath(pkgPath); modPath != "" {
				importPath = modPath
			}
		}

		if strings.Contains(importPath, "/") && !strings.HasPrefix(importPath, ".") && !strings.HasPrefix(importPath, "/") {
			// Insert Installation at the beginning of headers
			headers = append([]header{{level: 2, text: "Installation", id: "installation"}}, headers...)
			installSection.WriteString("## Installation\n\n")

			if isMain {
				// Command installation
				// Go installation prerequisites
				goVersion := getGoVersion(pkgPath)
				installSection.WriteString("<details>\n")
				installSection.WriteString("<summary><b>Prerequisites: Go Installation</b></summary>\n\n")
				installSection.WriteString(fmt.Sprintf("You'll need Go %s or later. [Install Go](https://go.dev/doc/install) if you haven't already.\n\n", goVersion))
				installSection.WriteString("<details>\n")
				installSection.WriteString("<summary><b>Setting up your PATH</b></summary>\n\n")
				installSection.WriteString("After installing Go, ensure that `$HOME/go/bin` is in your PATH:\n\n")
				installSection.WriteString("<details>\n")
				installSection.WriteString("<summary><b>For bash users</b></summary>\n\n")
				installSection.WriteString("Add to `~/.bashrc` or `~/.bash_profile`:\n")
				installSection.WriteString("```bash\n")
				installSection.WriteString("export PATH=\"$PATH:$HOME/go/bin\"\n")
				installSection.WriteString("```\n\n")
				installSection.WriteString("Then reload your configuration:\n")
				installSection.WriteString("```bash\n")
				installSection.WriteString("source ~/.bashrc\n")
				installSection.WriteString("```\n\n")
				installSection.WriteString("</details>\n\n")
				installSection.WriteString("<details>\n")
				installSection.WriteString("<summary><b>For zsh users</b></summary>\n\n")
				installSection.WriteString("Add to `~/.zshrc`:\n")
				installSection.WriteString("```bash\n")
				installSection.WriteString("export PATH=\"$PATH:$HOME/go/bin\"\n")
				installSection.WriteString("```\n\n")
				installSection.WriteString("Then reload your configuration:\n")
				installSection.WriteString("```bash\n")
				installSection.WriteString("source ~/.zshrc\n")
				installSection.WriteString("```\n\n")
				installSection.WriteString("</details>\n\n")
				installSection.WriteString("</details>\n\n")
				installSection.WriteString("</details>\n\n")
				installSection.WriteString("### Install\n\n")
				installSection.WriteString("```console\n")
				installSection.WriteString(fmt.Sprintf("go install %s@latest\n", importPath))
				installSection.WriteString("```\n\n")
				installSection.WriteString("### Run directly\n\n")
				installSection.WriteString("```console\n")
				installSection.WriteString(fmt.Sprintf("go run %s@latest [arguments]\n", importPath))
				installSection.WriteString("```\n\n")
			} else {
				// Library installation
				goVersion := getGoVersion(pkgPath)
				installSection.WriteString(fmt.Sprintf("To use this package in your Go project, you'll need [Go](https://go.dev/doc/install) %s or later installed on your system.\n\n", goVersion))
				installSection.WriteString("```console\n")
				installSection.WriteString(fmt.Sprintf("go get %s\n", importPath))
				installSection.WriteString("```\n\n")
				installSection.WriteString("Then import it in your code:\n\n")
				installSection.WriteString("```go\n")
				installSection.WriteString(fmt.Sprintf("import \"%s\"\n", importPath))
				installSection.WriteString("```\n\n")
			}
		}
	}

	// Skip declarations for main packages unless -all
	if !isMain || showAll {
		// Declarations
		if len(docPkg.Consts) > 0 {
			headers = append(headers, header{level: 2, text: "Constants", id: "constants"})
			content.WriteString("## Constants\n\n")
			writeSectionContent(&content, docPkg.Consts, fset)
		}
		if len(docPkg.Vars) > 0 {
			headers = append(headers, header{level: 2, text: "Variables", id: "variables"})
			content.WriteString("## Variables\n\n")
			writeSectionContent(&content, docPkg.Vars, fset)
		}
		if len(docPkg.Funcs) > 0 {
			headers = append(headers, header{level: 2, text: "Functions", id: "functions"})
			content.WriteString("## Functions\n\n")
			writeFuncsContent(&content, docPkg.Funcs, fset, &headers)
		}
		if len(docPkg.Types) > 0 {
			headers = append(headers, header{level: 2, text: "Types", id: "types"})
			content.WriteString("## Types\n\n")
			writeTypesContent(&content, docPkg.Types, fset, &headers)
		}
	}

	// Add package doc intro
	out.WriteString(packageDoc.String())

	// Generate TOC if we have headers and it's enabled
	if genTOC && len(headers) > 0 {
		out.WriteString("## Table of Contents\n\n")
		writeTOC(&out, headers)
		out.WriteString("\n")
	}

	// Add Installation section first
	out.WriteString(installSection.String())

	// Add the rest of the package doc (with headers)
	if afterHeaders != "" {
		out.WriteString(afterHeaders)
		out.WriteString("\n")
	}

	// Add the rest of the content
	out.WriteString(content.String())

	return out.String(), nil
}

// formatDoc converts doc comments to markdown using go/doc/comment
func formatDoc(doc string) string {
	// Pre-process to handle literal shell-like blocks
	doc = preprocessConsoleBlocks(doc)

	p := &comment.Parser{}
	parsed := p.Parse(doc)

	pr := &comment.Printer{
		HeadingLevel: 2, // Use ## instead of ###
		HeadingID: func(h *comment.Heading) string {
			// Return empty string to disable anchor links
			return ""
		},
	}
	markdown := string(pr.Markdown(parsed))

	// Post-process to restore console blocks and add language hints
	return postprocessConsoleBlocks(markdown)
}

// preprocessConsoleBlocks converts shell-like blocks to placeholders before parsing
func preprocessConsoleBlocks(doc string) string {
	// Support multiple shell-like language types
	shellLanguages := []string{"console", "sh-session", "bash", "shell", "ShellSession"}

	counter := 0
	result := doc

	for _, lang := range shellLanguages {
		re := regexp.MustCompile(fmt.Sprintf("```%s\n((?:.*\n)*?)```", regexp.QuoteMeta(lang)))
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			counter++
			// Convert to indented code block with special marker
			lines := strings.Split(match, "\n")
			var output strings.Builder
			output.WriteString(fmt.Sprintf("SHELLMARKERSTART%dLANG%s\n", counter, lang))
			for i, line := range lines[1 : len(lines)-1] { // Skip ```lang and ```
				if i == 0 || line != "" {
					output.WriteString("\t" + line + "\n")
				}
			}
			output.WriteString(fmt.Sprintf("SHELLMARKEREND%dLANG%s", counter, lang))
			return output.String()
		})
	}

	return result
}

// postprocessConsoleBlocks restores shell blocks from placeholders
func postprocessConsoleBlocks(markdown string) string {
	// More flexible regex to handle various shell language types
	re := regexp.MustCompile(`SHELLMARKERSTART(\d+)LANG([^_\s]+)\s*((?:.*\n)*?)\s*SHELLMARKEREND\d+LANG([^_\s]+)`)
	markdown = re.ReplaceAllStringFunc(markdown, func(match string) string {
		// Extract the language and content
		lines := strings.Split(match, "\n")
		var result strings.Builder

		// Find start marker to extract language
		var lang string
		startIdx := -1
		endIdx := -1

		for i, line := range lines {
			if strings.Contains(line, "SHELLMARKERSTART") {
				startIdx = i
				// Extract language from marker: SHELLMARKERSTART1LANGconsole -> console
				parts := strings.Split(line, "LANG")
				if len(parts) >= 2 {
					lang = parts[1]
				}
			} else if strings.Contains(line, "SHELLMARKEREND") {
				endIdx = i
				break
			}
		}

		// Default to console if no language found
		if lang == "" {
			lang = "console"
		}

		result.WriteString("```" + lang + "\n")

		// Extract content between markers
		if startIdx >= 0 && endIdx >= 0 {
			for _, line := range lines[startIdx+1 : endIdx] {
				// Remove leading tab that was added for indentation
				if strings.HasPrefix(line, "\t") {
					result.WriteString(line[1:] + "\n")
				} else if strings.HasPrefix(line, "    ") {
					// Handle 4-space indentation too
					result.WriteString(line[4:] + "\n")
				} else {
					result.WriteString(line + "\n")
				}
			}
		}
		result.WriteString("```")
		return result.String()
	})

	// Then add shell language hints to other code blocks
	return addShellLanguageHints(markdown)
}

// addShellLanguageHints detects shell commands and adds language hints to code blocks
func addShellLanguageHints(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result strings.Builder

	inCodeBlock := false
	codeBlockContent := []string{}
	codeBlockStart := ""

	for _, line := range lines {
		// Check for code block start (with or without language)
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				// Starting a code block
				inCodeBlock = true
				codeBlockStart = line
				codeBlockContent = []string{}
			} else {
				// Ending a code block - analyze content
				if codeBlockStart == "```" && isShellCodeBlock(codeBlockContent) {
					// Only add shell hint to plain code blocks
					result.WriteString("```shell\n")
					for _, codeLine := range codeBlockContent {
						result.WriteString(codeLine + "\n")
					}
					result.WriteString("```\n")
				} else {
					// Keep existing language hint or plain block
					result.WriteString(codeBlockStart + "\n")
					for _, codeLine := range codeBlockContent {
						result.WriteString(codeLine + "\n")
					}
					result.WriteString("```\n")
				}
				inCodeBlock = false
			}
		} else if inCodeBlock {
			codeBlockContent = append(codeBlockContent, line)
		} else {
			result.WriteString(line + "\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}

// isShellCodeBlock detects if a code block contains shell commands
func isShellCodeBlock(lines []string) bool {
	shellIndicators := []string{
		"go install", "go run", "go build", "go get",
		"npm install", "npm run", "yarn",
		"docker", "kubectl", "curl", "wget",
		"mkdir", "cd ", "ls ", "cp ", "mv ", "rm ",
		"export ", "echo ", "./", "bash", "sh ",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		for _, indicator := range shellIndicators {
			if strings.Contains(trimmed, indicator) {
				return true
			}
		}
	}

	return false
}

// funcSig returns a formatted function signature
func funcSig(fn *ast.FuncDecl, fset *token.FileSet) string {
	var buf bytes.Buffer
	buf.WriteString("func ")

	// Receiver
	if fn.Recv != nil {
		buf.WriteString("(")
		if r := fn.Recv.List[0]; len(r.Names) > 0 {
			buf.WriteString(r.Names[0].Name + " ")
		}
		buf.WriteString(nodeString(fset, fn.Recv.List[0].Type))
		buf.WriteString(") ")
	}

	// Name and params
	buf.WriteString(fn.Name.Name)
	buf.WriteString(nodeString(fset, fn.Type.Params))

	// Results
	if fn.Type.Results != nil {
		buf.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			// Single unnamed result
			buf.WriteString(nodeString(fset, fn.Type.Results.List[0].Type))
		} else {
			// Multiple or named results
			buf.WriteString(nodeString(fset, fn.Type.Results))
		}
	}

	return buf.String()
}

// nodeString formats an AST node as a string
func nodeString(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	format.Node(&buf, fset, node)
	return strings.TrimSpace(buf.String())
}

// header represents a markdown header for TOC generation
type header struct {
	level int
	text  string
	id    string
}

// formatDocWithHeaders is like formatDoc but also collects headers and splits content
func formatDocWithHeaders(doc string, headers *[]header) (beforeFirstHeader, afterFirstHeader string) {
	// Pre-process to handle literal shell-like blocks
	doc = preprocessConsoleBlocks(doc)

	p := &comment.Parser{}
	parsed := p.Parse(doc)

	pr := &comment.Printer{
		HeadingLevel: 2, // Use ## instead of ###
		HeadingID: func(h *comment.Heading) string {
			// Return empty string to disable anchor links
			return ""
		},
	}
	markdown := string(pr.Markdown(parsed))
	formatted := postprocessConsoleBlocks(markdown)

	// Parse the formatted markdown to extract headers
	lines := strings.Split(formatted, "\n")
	var before, after strings.Builder
	foundFirstHeader := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for markdown headers (now ## without anchors)
		if strings.HasPrefix(trimmed, "## ") {
			foundFirstHeader = true
			headerText := trimmed[3:]
			id := makeHeaderID(headerText)
			*headers = append(*headers, header{level: 2, text: headerText, id: id})

			after.WriteString(line + "\n")
			continue
		}

		// Normal text
		if foundFirstHeader {
			after.WriteString(line + "\n")
		} else {
			before.WriteString(line + "\n")
		}
	}

	return strings.TrimRight(before.String(), "\n"), strings.TrimRight(after.String(), "\n")
}

// makeHeaderID creates a GitHub-compatible header ID
func makeHeaderID(text string) string {
	// Convert to lowercase
	id := strings.ToLower(text)
	// Replace spaces with hyphens
	id = strings.ReplaceAll(id, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	id = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(id, "")
	// Collapse multiple hyphens
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	// Trim hyphens from ends
	id = strings.Trim(id, "-")
	return id
}

// writeTOC writes the table of contents
func writeTOC(out *strings.Builder, headers []header) {
	for _, h := range headers {
		indent := strings.Repeat("  ", h.level-2)
		fmt.Fprintf(out, "%s- [%s](#%s)\n", indent, h.text, h.id)
	}
}

// writeSectionContent writes content for const/var sections
func writeSectionContent(out *strings.Builder, values []*doc.Value, fset *token.FileSet) {
	for _, v := range values {
		out.WriteString("```go\n")
		out.WriteString(nodeString(fset, v.Decl))
		out.WriteString("\n```\n\n")

		if v.Doc != "" {
			out.WriteString(formatDoc(v.Doc))
			out.WriteString("\n\n")
		}
	}
}

// writeFuncsContent writes function content and collects headers
func writeFuncsContent(out *strings.Builder, funcs []*doc.Func, fset *token.FileSet, headers *[]header) {
	for _, f := range funcs {
		id := makeHeaderID(f.Name)
		*headers = append(*headers, header{level: 3, text: f.Name, id: id})
		fmt.Fprintf(out, "### %s\n\n```go\n%s\n```\n\n", f.Name, funcSig(f.Decl, fset))
		if f.Doc != "" {
			out.WriteString(formatDoc(f.Doc))
			out.WriteString("\n\n")
		}
	}
}

// writeTypesContent writes type content and collects headers
func writeTypesContent(out *strings.Builder, types []*doc.Type, fset *token.FileSet, headers *[]header) {
	for _, t := range types {
		id := makeHeaderID(t.Name)
		*headers = append(*headers, header{level: 3, text: t.Name, id: id})
		fmt.Fprintf(out, "### %s\n\n```go\n%s\n```\n\n", t.Name, nodeString(fset, t.Decl))
		if t.Doc != "" {
			out.WriteString(formatDoc(t.Doc))
			out.WriteString("\n\n")
		}

		// Methods
		if len(t.Methods) > 0 {
			out.WriteString("#### Methods\n\n")
			for _, m := range t.Methods {
				mid := makeHeaderID(t.Name + "." + m.Name)
				*headers = append(*headers, header{level: 4, text: t.Name + "." + m.Name, id: mid})
				fmt.Fprintf(out, "##### %s\n\n```go\n%s\n```\n\n", m.Name, funcSig(m.Decl, fset))
				if m.Doc != "" {
					out.WriteString(formatDoc(m.Doc))
					out.WriteString("\n\n")
				}
			}
		}
	}
}

// getGoVersion tries to determine the Go version from go.mod
func getGoVersion(pkgPath string) string {
	dir := pkgPath
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(modPath)
		if err == nil {
			modFile, err := modfile.Parse(modPath, data, nil)
			if err == nil && modFile.Go != nil {
				version := modFile.Go.Version
				// Trim to major.minor format (e.g., "1.24.3" -> "1.24")
				if major, rest, found := strings.Cut(version, "."); found {
					if minor, _, _ := strings.Cut(rest, "."); minor != "" {
						return major + "." + minor
					}
				}
				return version
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "1.21" // fallback
}

// getModulePath tries to determine the module path from go.mod
func getModulePath(pkgPath string) string {
	dir := pkgPath
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(modPath)
		if err == nil {
			modFile, err := modfile.Parse(modPath, data, nil)
			if err == nil && modFile.Module != nil {
				// Calculate the full import path
				relPath, _ := filepath.Rel(dir, pkgPath)
				if relPath == "." {
					return modFile.Module.Mod.Path
				}
				return filepath.Join(modFile.Module.Mod.Path, relPath)
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
