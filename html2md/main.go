// Embeds the documentation from doc.go
//
//go:generate cp doc.go docs.txt

/*
html2md converts HTML input to Markdown format.

This command-line tool reads HTML from either standard input or a specified file
and outputs the equivalent Markdown representation. It uses the html-to-markdown
library with the GitHub Flavored Markdown plugin enabled by default.

Usage:

	html2md [-input=<filename>] [-sanitize]

The -input flag specifies the input file. If omitted or set to "-", html2md
reads from standard input.

The -sanitize flag enables HTML sanitization via bluemonday before conversion to Markdown.

html2md is designed to be simple and composable, following Unix philosophy. It
can be easily integrated into pipelines or scripts for processing HTML content.
*/
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/microcosm-cc/bluemonday"
)

//go:embed doc.go
var documentation string

var (
	flagInput    = flag.String("input", "-", "input file (default: stdin)")
	flagSanitize = flag.Bool("sanitize", false, "sanitize HTML before conversion (using bluemonday)")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(1)
	}
	if err := run(*flagInput, *flagSanitize); err != nil {
		log.Fatal(err)
	}
}

func run(input string, sanitize bool) error {
	var r io.Reader
	if input == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(input)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	md, err := convert(r, sanitize)
	if err != nil {
		return err
	}
	fmt.Println(md)
	return nil
}

func convert(r io.Reader, sanitize bool) (string, error) {
	// Read the entire input
	content, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	// Sanitize if requested
	if sanitize {
		p := bluemonday.UGCPolicy()
		content = []byte(p.SanitizeBytes(content))
	}

	// Create a new reader from the processed content
	contentReader := strings.NewReader(string(content))

	// Convert to markdown
	conv := md.NewConverter("", true, nil)
	conv.Use(plugin.GitHubFlavored())
	markdown, err := conv.ConvertReader(contentReader)
	if err != nil {
		return "", err
	}
	return markdown.String(), nil
}
