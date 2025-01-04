package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: %s title\n", os.Args[0])
		os.Exit(1)
	}
	title := flag.Arg(0)
	if err := setWindowTitle(title); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectTerminal() string {
	if term := os.Getenv("TERM_PROGRAM"); term != "" {
		return strings.ToLower(term)
	}
	return strings.ToLower(os.Getenv("TERM"))
}

