package main

import (
	"fmt"
	"os"

	"github.com/tmc/misc/cli-skeleton/commands"
)

func main() {
	if err := commands.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
