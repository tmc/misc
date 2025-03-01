package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// createFileFromTemplate creates a file from a template string
func createFileFromTemplate(path, tmplStr string, data interface{}) error {
	tmpl, err := template.New(filepath.Base(path)).Parse(tmplStr)
	if err != nil {
		return err
	}
	
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	
	return tmpl.Execute(f, data)
}

// createFileFromTemplateFile creates a file from a template file
func createFileFromTemplateFile(path, tmplPath string, data interface{}) error {
	// Read the template file
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", tmplPath, err)
	}
	
	return createFileFromTemplate(path, string(tmplContent), data)
}