/*
htmldistill is a command-line tool that extracts and distills the main content
from HTML documents. It processes input from URLs, files, or standard input,
removing clutter such as navigation, ads, and other non-essential elements.

Usage:

	htmldistill <url1> [url2] [url3] ...

htmldistill accepts one or more URLs as arguments. For each URL, it fetches
the content, processes it using the go-domdistiller library, and outputs the
extracted main content as HTML to stdout.

The tool can also process local files or input from stdin by using '-' as an
argument. When reading from stdin, an optional base URL can be provided to
resolve relative links.

htmldistill is useful for cleaning up web content for further processing,
improving readability, or preparing data for natural language processing tasks.
*/
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	distiller "github.com/markusmobius/go-domdistiller"
	"golang.org/x/net/html"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: htmldistill <url1> [url2] [url3] ...")
		os.Exit(1)
	}

	for _, arg := range os.Args[1:] {
		if err := run(arg, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", arg, err)
		}
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
	fmt.Fprintf(os.Stderr, "Processing URL: %s\n", url)
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
