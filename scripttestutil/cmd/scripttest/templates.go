package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templateData holds data for template execution
type templateData struct {
	BuildID string // build identifier
}

func setupTestDir(dir string) error {
	buildID := getBuildID()
	data := templateData{
		BuildID: buildID,
	}

	templates := []struct {
		src  string
		dst  string
		data interface{}
	}{
		{
			src:  "templates/test_main.go.tmpl",
			dst:  "main_test.go",
			data: data,
		},
		{
			src:  "templates/go.mod.tmpl",
			dst:  "go.mod",
			data: data,
		},
	}

	for _, t := range templates {
		if err := writeTemplate(dir, t.src, t.dst, t.data); err != nil {
			return fmt.Errorf("failed to write template %s: %v", t.dst, err)
		}
	}

	return nil
}

func getBuildID() string {
	// Default if we can't get VCS info
	buildID := fmt.Sprintf("scripttest-%s", time.Now().Format("20060102"))

	if bi, ok := debug.ReadBuildInfo(); ok {
		var revision, modified string
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) >= 12 {
					revision = s.Value[:12]
				}
			case "vcs.modified":
				if s.Value == "true" {
					modified = "-d"
				}
			}
		}
		if revision != "" {
			buildID = fmt.Sprintf("scripttest-%s-%s%s",
				revision,
				time.Now().Format("20060102"),
				modified)
		}
	}
	return buildID
}

func writeTemplate(dir, src, dst string, data interface{}) error {
	content, err := templateFS.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %v", src, err)
	}

	tmpl, err := template.New(filepath.Base(src)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %v", src, err)
	}

	f, err := os.Create(filepath.Join(dir, dst))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", dst, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %v", src, err)
	}

	return nil
}
