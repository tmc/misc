/*
Package html2md converts HTML input to Markdown format.

This command-line tool reads HTML from either standard input or a specified file
and outputs the equivalent Markdown representation. It uses the html-to-markdown
library with the GitHub Flavored Markdown plugin enabled by default.

Usage:

	html2md [-input=<filename>] [-sanitize]

The -input flag specifies the input file. If omitted or set to "-", html2md
reads from standard input.

The -sanitize flag enables HTML sanitization via bluemonday before conversion to Markdown.
This helps remove potentially malicious HTML elements and attributes like script tags,
javascript: URLs, and event handlers (onclick, etc.).

html2md is designed to be simple and composable, following Unix philosophy. It
can be easily integrated into pipelines or scripts for processing HTML content.

For more information and examples, see https://pkg.go.dev/github.com/tmc/misc/html2md

License: ISC
*/
package main
