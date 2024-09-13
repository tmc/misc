package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	distiller "github.com/markusmobius/go-domdistiller"
	"golang.org/x/net/html"
)

var (
	flagInput = flag.String("input", "-", "Path to the input HTML file, or URL to download")
)

func main() {
	fmt.Println("Hello World")
	flag.Parse()
	if err := run(*flagInput); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input string) error {
	if urlLike(input) {
		return runURL(input)
	}
	if input == "-" {
		return runReader(os.Stdin)
	}
	return runFile(input)
}

func urlLike(input string) bool {
	return strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}

func runURL(url string) error {
	return handle(distiller.ApplyForURL(url, time.Minute, nil))
}

func runReader(r io.Reader) error {
	return handle(distiller.ApplyForReader(r, nil))
}

func runFile(path string) error {
	return handle(distiller.ApplyForFile(path, &distiller.Options{
		LogFlags: distiller.LogEverything,
	}))
}

func handle(result *distiller.Result, err error) error {
	if err != nil {
		return fmt.Errorf("failed to distill file: %w", err)
	}
	o, err := OuterHTML(result.Node)
	if err != nil {
		return fmt.Errorf("failed to get outer html: %w", err)
	}
	fmt.Println(o)
	return nil
}

// OuterHTML returns an HTML serialization of the element and its descendants.
// The returned HTML value is escaped.
func OuterHTML(node *html.Node) (string, error) {
	if node == nil {
		return "", nil
	}

	var buffer bytes.Buffer
	err := html.Render(&buffer, node)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	return buffer.String(), nil
}
