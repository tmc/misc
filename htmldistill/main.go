package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	distiller "github.com/markusmobius/go-domdistiller"
	"golang.org/x/net/html"
)

var (
	flagInput   = flag.String("input", "-", "Path to the input HTML file, or URL to download")
	flagBaseURL = flag.String("base-url", "", "Optional base URL for input from stdin")
)

func main() {
	flag.Parse()
	if err := run(*flagInput, *flagBaseURL); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input string, baseURL string) error {
	if urlLike(input) {
		return runURL(input)
	}
	if input == "-" {
		return runReader(os.Stdin, baseURL)
	}
	return runFile(input)
}

func urlLike(input string) bool {
	return strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}

func runURL(url string) error {
	return handle(distiller.ApplyForURL(url, time.Minute, &distiller.Options{
		LogFlags: distiller.LogEverything,
	}))
}

func runReader(r io.Reader, baseURL string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	opts := &distiller.Options{
		OriginalURL: u,
		LogFlags:    distiller.LogEverything,
	}
	return handle(distiller.ApplyForReader(r, opts))
}

func runFile(path string) error {
	return handle(distiller.ApplyForFile(path, &distiller.Options{
		LogFlags: distiller.LogEverything,
	}))
}

func handle(result *distiller.Result, err error) error {
	if err != nil {
		return fmt.Errorf("failed to distill content: %w", err)
	}
	if result == nil || result.Node == nil {
		return fmt.Errorf("no content extracted")
	}
	output, err := outerHTML(result.Node)
	if err != nil {
		return fmt.Errorf("failed to get outer HTML: %w", err)
	}
	fmt.Println(output)
	return nil
}

// outerHTML returns an HTML serialization of the element and its descendants.
func outerHTML(node *html.Node) (string, error) {
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
