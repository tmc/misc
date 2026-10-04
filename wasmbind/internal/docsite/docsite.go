// Package docsite renders navigable references and checks supplied TypeScript consumers.
package docsite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

// Write creates an HTML reference from a generated bundle's canonical schema.
// Output must be a directory owned by the caller.
func Write(bundle, out string) error {
	raw, err := os.ReadFile(filepath.Join(bundle, "bindings.json"))
	if err != nil {
		return err
	}
	var m schema.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if m.Generated != schema.Marker || m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported documentation schema")
	}
	if !validManifest(&m) {
		return fmt.Errorf("incomplete documentation schema")
	}
	api := generate.APIReference(&schema.API{Manifest: m})
	var body, nav bytes.Buffer
	lines := strings.Split(string(api), "\n")
	inCode := false
	id := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "<!--") {
			continue
		}
		if strings.HasPrefix(line, "```") {
			if inCode {
				body.WriteString("</code></pre>\n")
			} else {
				body.WriteString("<pre><code>")
			}
			inCode = !inCode
			continue
		}
		if inCode {
			body.WriteString(html.EscapeString(line) + "\n")
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level > 0 && level <= 3 && len(line) > level && line[level] == ' ' {
			id++
			title := html.EscapeString(strings.TrimSpace(line[level:]))
			fmt.Fprintf(&body, "<h%d id=\"section-%d\">%s</h%d>\n", level, id, title, level)
			fmt.Fprintf(&nav, "<li><a href=\"#section-%d\">%s</a></li>\n", id, title)
			continue
		}
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(&body, "<p>%s</p>\n", html.EscapeString(line))
		}
	}
	if inCode {
		return fmt.Errorf("unterminated reference code block")
	}
	page := fmt.Sprintf("<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>JavaScript API</title><style>body{font:16px system-ui;max-width:80rem;margin:auto;padding:2rem}nav{max-width:25rem}pre{overflow:auto;background:#eee;padding:1rem}code{font-size:.9rem}h2,h3{scroll-margin:1rem}</style><nav aria-label=\"API sections\"><ul>%s</ul></nav><main>%s</main></html>\n", nav.String(), body.String())
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "index.html"), []byte(page), 0644)
}

// CheckExamples runs an explicitly supplied TypeScript compiler with strict,
// emission-free consumer checks. It never downloads tools or modifies examples.
func CheckExamples(ctx context.Context, tsc, lib string, files []string) error {
	if tsc == "" || len(files) == 0 {
		return fmt.Errorf("check-examples requires -tsc and TypeScript files")
	}
	// Explicit files must not inherit a caller's tsconfig. TypeScript 6 rejects
	// that combination; an empty working directory also works with older tsc.
	if strings.ContainsRune(tsc, filepath.Separator) {
		var err error
		tsc, err = filepath.Abs(tsc)
		if err != nil {
			return fmt.Errorf("resolve compiler: %w", err)
		}
	}
	paths := make([]string, len(files))
	for i, file := range files {
		var err error
		paths[i], err = filepath.Abs(file)
		if err != nil {
			return fmt.Errorf("resolve example: %w", err)
		}
	}
	dir, err := os.MkdirTemp("", "wasmbind-examples-")
	if err != nil {
		return fmt.Errorf("create compiler directory: %w", err)
	}
	defer os.RemoveAll(dir)
	args := []string{"--strict", "--noEmit", "--target", "ES2022", "--module", "ESNext", "--moduleResolution", "Bundler", "--lib", lib}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, tsc, args...)
	cmd.Dir = dir
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("type-check examples: %w\n%s", err, data)
	}
	return nil
}

func validManifest(m *schema.Manifest) bool {
	var validType func(*schema.Type) bool
	validType = func(t *schema.Type) bool {
		if t == nil {
			return false
		}
		if t.Kind == "pointer" || t.Kind == "slice" || t.Kind == "array" {
			return validType(t.Elem)
		}
		return t.Kind != ""
	}
	validCall := func(c *schema.Call) bool {
		if c == nil {
			return false
		}
		for _, p := range c.Params {
			if !validType(p.Type) {
				return false
			}
		}
		return c.Result == nil || validType(c.Result)
	}
	for _, d := range m.Declarations {
		if d == nil {
			return false
		}
		if (d.Kind == "enum" || d.Kind == "scalar") && !validType(d.Value) {
			return false
		}
		for _, f := range d.Fields {
			if !validType(f.Type) {
				return false
			}
		}
		for _, c := range d.ImportMethods {
			if !validCall(c) {
				return false
			}
		}
		if c := d.Methods["next"]; c != nil && (!validCall(c) || c.Result == nil) {
			return false
		}
	}
	for _, c := range m.Calls {
		if !validCall(c) {
			return false
		}
	}
	return true
}
