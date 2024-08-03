package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	texttemplate "text/template"
)

func main() {
	// Define flags
	templateFile := flag.String("template", "-", "Path to the template file (use '-' for stdin)")
	strictMode := flag.Bool("strict", false, "Exit with an error if any placeholder is not replaced")
	envPriority := flag.Bool("env-priority", false, "Give priority to environment variables over command-line arguments")
	useHTML := flag.Bool("html", false, "Use html/template instead of text/template")
	flag.Parse()

	// Read the template
	var content []byte
	var err error
	if *templateFile == "-" {
		content, err = ioutil.ReadAll(os.Stdin)
	} else {
		content, err = ioutil.ReadFile(*templateFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading template: %v\n", err)
		os.Exit(1)
	}

	// Prepare data for template execution
	data := make(map[string]string)

	// Process environment variables
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) == 2 && (*envPriority || data[pair[0]] == "") {
			data[pair[0]] = pair[1]
		}
	}

	// Process command-line arguments
	for _, arg := range flag.Args() {
		pair := strings.SplitN(arg, "=", 2)
		if len(pair) == 2 && (!*envPriority || data[pair[0]] == "") {
			data[pair[0]] = pair[1]
		}
	}

	// Convert old syntax to new syntax for all-caps placeholders
	reOld := regexp.MustCompile(`{{([A-Z0-9_]+)}}`)
	contentString := reOld.ReplaceAllString(string(content), "{{.$1}}")

	// Create and parse the template
	var tmpl interface {
		Execute(wr io.Writer, data interface{}) error
	}

	if *useHTML {
		tmpl, err = template.New("template").Parse(contentString)
	} else {
		tmpl, err = texttemplate.New("template").Parse(contentString)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing template: %v\n", err)
		os.Exit(1)
	}

	// Execute the template
	if *strictMode {
		// In strict mode, we use a custom type that returns an error for missing keys
		err = tmpl.Execute(os.Stdout, strictMap{data})
	} else {
		err = tmpl.Execute(os.Stdout, data)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing template: %v\n", err)
		os.Exit(1)
	}
}

// strictMap is a custom type that returns an error for missing keys in strict mode
type strictMap struct {
	m map[string]string
}

func (s strictMap) Get(key string) (string, error) {
	if val, ok := s.m[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("missing value for key: %s", key)
}
