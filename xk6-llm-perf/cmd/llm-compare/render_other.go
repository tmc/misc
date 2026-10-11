//go:build !unix

package main

import (
	"fmt"
	"os"
)

func renderPNG(path string, svg []byte) ([]byte, error) {
	return nil, fmt.Errorf("PNG process supervision requires Unix")
}

func openSnapshot(path string) (*os.File, error) {
	return nil, fmt.Errorf("bounded snapshot loading requires Unix")
}
