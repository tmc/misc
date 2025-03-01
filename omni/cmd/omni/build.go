package main

import (
	"flag"
	"fmt"

	"github.com/tmc/misc/omni/internal/validate"
)

func buildCommand(args []string) error {
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	dryRun := buildFlags.Bool("n", false, "Dry run")
	longDryRun := buildFlags.Bool("dry-run", false, "Dry run")
	format := buildFlags.String("f", "text", "Output format (text, json)")
	longFormat := buildFlags.String("format", "text", "Output format (text, json)")

	if err := buildFlags.Parse(args); err != nil {
		return err
	}

	// Combine short and long flags
	isDryRun := *dryRun || *longDryRun
	outputFormat := *format
	if *longFormat != "text" {
		outputFormat = *longFormat
	}

	if buildFlags.NArg() < 1 {
		return fmt.Errorf("version argument is required")
	}

	version := buildFlags.Arg(0)
	if err := validate.VersionFormat(version); err != nil {
		return err
	}

	if isDryRun {
		fmt.Printf("Build Plan for %s\n", version)
		fmt.Println("1. Validate version")
		fmt.Println("2. Build binaries")
		fmt.Println("3. Generate packages")
		fmt.Println("4. Validate packages")
		return nil
	}

	fmt.Printf("Building packages for %s (format: %s)...\n", version, outputFormat)
	// TODO: Implement actual build process
	fmt.Println("Done.")
	return nil
}