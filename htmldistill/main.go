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

var verbose bool

func main() {
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: htmldistill [-v] <url1> [url2] [url3] ...")
		os.Exit(1)
	}

	for _, arg := range flag.Args() {
		if err := run(arg, "", verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", arg, err)
		}
	}
}

func run(input string, baseURL string, verbose bool) error {
	if verbose {
		fmt.Printf("Processing %s\n", input)
	}

	if urlLike(input) {
		return runURL(input, verbose)
	}
	if input == "-" {
		return runReader(os.Stdin, baseURL, verbose)
	}
	return runFile(input, verbose)
}

func runURL(url string, verbose bool) error {
	if verbose {
		fmt.Printf("Fetching URL: %s\n", url)
	}
	logFlags := distiller.LogFlag(0)
	if verbose {
		logFlags = distiller.LogFlag(255)
	}
	result, err := distiller.ApplyForURL(url, time.Minute, &distiller.Options{
		LogFlags: logFlags,
	})
	return handle(result, err, verbose)
}

func runReader(r io.Reader, baseURL string, verbose bool) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	logFlags := distiller.LogFlag(0)
	if verbose {
		logFlags = distiller.LogFlag(255)
	}
	opts := &distiller.Options{
		OriginalURL: u,
		LogFlags:    logFlags,
	}
	result, err := distiller.ApplyForReader(r, opts)
	return handle(result, err, verbose)
}

func runFile(path string, verbose bool) error {
	if verbose {
		fmt.Printf("Reading file: %s\n", path)
	}
	logFlags := distiller.LogFlag(0)
	if verbose {
		logFlags = distiller.LogFlag(255)
	}
	result, err := distiller.ApplyForFile(path, &distiller.Options{
		LogFlags: logFlags,
	})
	return handle(result, err, verbose)
}

func urlLike(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func handle(result *distiller.Result, err error, verbose bool) error {
	if err != nil {
		return fmt.Errorf("failed to distill content: %w", err)
	}
	if result == nil || result.Node == nil {
		return fmt.Errorf("no content extracted")
	}
	if verbose {
		fmt.Println("Rendering HTML output...")
	}
	output, err := outerHTML(result.Node)
	if err != nil {
		return fmt.Errorf("failed to get outer HTML: %w", err)
	}
	fmt.Println(output)
	return nil
}

// Define the outerHTML function
func outerHTML(node *html.Node) (string, error) {
	var b strings.Builder
	err := html.Render(&b, node)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}
