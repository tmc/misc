// Package main provides the jsfmt command line tool for formatting JavaScript.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tmc/misc/jsfmt/formatter"
)

func main() {
	var (
		write   = flag.Bool("w", false, "write result to source file instead of stdout")
		list    = flag.Bool("l", false, "list files whose formatting differs from jsfmt's")
		doDiff  = flag.Bool("d", false, "display diffs instead of rewriting files")
		tabSize = flag.Int("tab-size", 2, "size of tabs in spaces")
	)
	flag.Parse()

	if flag.NArg() == 0 {
		if err := processFile("<stdin>", os.Stdin, os.Stdout, *tabSize); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	for i := 0; i < flag.NArg(); i++ {
		path := flag.Arg(i)
		switch dir, err := os.Stat(path); {
		case err != nil:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		case dir.IsDir():
			walkDir(path, *write, *list, *doDiff, *tabSize)
		default:
			if err := processPath(path, *write, *list, *doDiff, *tabSize); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func walkDir(path string, write, list, doDiff bool, tabSize int) {
	filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if err != nil || f.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".js" {
			err = processPath(path, write, list, doDiff, tabSize)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error processing %s: %v\n", path, err)
			}
		}
		return nil
	})
}

func processPath(path string, write, list, doDiff bool, tabSize int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if list {
		if formatted, err := formatter.IsFormatted(f, tabSize); err != nil {
			return err
		} else if !formatted {
			fmt.Println(path)
		}
		return nil
	}

	if write {
		var nf *os.File
		nf, err = os.CreateTemp("", "jsfmt")
		if err != nil {
			return err
		}
		defer os.Remove(nf.Name())
		defer nf.Close()

		if err = processFile(path, f, nf, tabSize); err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}

		if err = nf.Close(); err != nil {
			return err
		}

		return os.Rename(nf.Name(), path)
	}

	if doDiff {
		return formatter.WriteDiff(os.Stdout, f, path, tabSize)
	}

	return processFile(path, f, os.Stdout, tabSize)
}

func processFile(filename string, in io.Reader, out io.Writer, tabSize int) error {
	src, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	res, err := formatter.Format(filename, src, tabSize)
	if err != nil {
		return err
	}

	_, err = out.Write(res)
	return err
}