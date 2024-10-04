/*
html2md converts HTML input to Markdown format.

This command-line tool reads HTML from either standard input or a specified file
and outputs the equivalent Markdown representation. It uses the html-to-markdown
library with the GitHub Flavored Markdown plugin enabled by default.

Usage:

	html2md [-input=<filename>]

The -input flag specifies the input file. If omitted or set to "-", html2md
reads from standard input.

html2md is designed to be simple and composable, following Unix philosophy. It
can be easily integrated into pipelines or scripts for processing HTML content.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

var flagInput = flag.String("input", "-", "input file (default: stdin)")

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(1)
	}
	if err := run(*flagInput); err != nil {
		log.Fatal(err)
	}
}

func run(input string) error {
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

	md, err := convert(r)
	if err != nil {
		return err
	}
	fmt.Println(md)
	return nil
}

func convert(r io.Reader) (string, error) {
	conv := md.NewConverter("", true, nil)
	conv.Use(plugin.GitHubFlavored())
	markdown, err := conv.ConvertReader(r)
	if err != nil {
		return "", err
	}
	return markdown.String(), nil
}
