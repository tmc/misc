package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  build [-n] [-f format] <version>    Build packages\n")
		fmt.Fprintf(os.Stderr, "  publish [-n] <version>              Publish packages\n")
		fmt.Fprintf(os.Stderr, "  run <package> [args...]             Run a package\n")
		fmt.Fprintf(os.Stderr, "  version                             Show version\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  -n, --dry-run     Show what would be done\n")
		fmt.Fprintf(os.Stderr, "  -f, --format      Output format (text, json)\n")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "build":
		err := buildCommand(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "publish":
		err := publishCommand(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "run":
		err := runCommand(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("omni version %s\n", Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		flag.Usage()
		os.Exit(1)
	}
}