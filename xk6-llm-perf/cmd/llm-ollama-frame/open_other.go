//go:build !unix

package main

import (
	"fmt"
	"os"
)

func openRegular(string) (*os.File, error) {
	return nil, fmt.Errorf("offline frame replay requires Unix")
}
