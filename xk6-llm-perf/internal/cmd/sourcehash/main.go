// Command sourcehash identifies the local extension sources and example scripts.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sourcehash:", err)
		os.Exit(1)
	}
}

func run() error {
	var paths []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case ".git", "bin", "dist", "outputs", "docs/specs":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if path == "go.mod" || path == "go.sum" || path == "Makefile" || strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".js") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(path), filepath.ToSlash(path), info.Size())
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	fmt.Printf("%x\n", h.Sum(nil))
	return nil
}
