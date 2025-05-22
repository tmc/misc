package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/tools/txtar"
)

// Command-line flags
var (
	examplesPath string
	outputFile   string
	allFlag      bool
)

func init() {
	flag.StringVar(&outputFile, "o", "", "Output file (defaults to stdout)")
	flag.BoolVar(&allFlag, "a", false, "Package all examples")
	flag.StringVar(&examplesPath, "path", "", "Path to examples directory (auto-detected if not specified)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gen [options] [example_name]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  gen openapi                  # Package the OpenAPI example to stdout\n")
		fmt.Fprintf(os.Stderr, "  gen graphql -o graphql.txtar # Save the GraphQL example to a file\n")
		fmt.Fprintf(os.Stderr, "  gen -a -o examples.txtar     # Package all examples into a single file\n")
		fmt.Fprintf(os.Stderr, "  gen -path=/path/to/examples openapi # Specify examples directory explicitly\n")
	}
}

func main() {
	flag.Parse()

	// Get the example name from positional arguments
	var exampleName string
	args := flag.Args()
	if len(args) > 0 {
		exampleName = args[0]
	}

	// Handle special cases
	if exampleName == "all" {
		allFlag = true
		exampleName = ""
	}

	// Validate arguments
	if exampleName == "" && !allFlag {
		fmt.Fprintf(os.Stderr, "Error: must specify an example name or use -a for all examples\n")
		flag.Usage()
		os.Exit(1)
	}

	// Find the base directory for examples
	var baseDir string
	
	// If examples path is explicitly specified, use that
	if examplesPath != "" {
		if stat, err := os.Stat(examplesPath); err != nil || !stat.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: Specified examples path '%s' does not exist or is not a directory\n", examplesPath)
			os.Exit(1)
		}
		baseDir = examplesPath
	} else {
		// Try to auto-detect the examples directory
		// First, try to find it relative to the executable
		execPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding executable path: %v\n", err)
			os.Exit(1)
		}
		
		// Try different relative paths to locate the examples directory
		possiblePaths := []string{
			// Same directory as the executable
			filepath.Join(filepath.Dir(execPath), "..", "..", "examples"),
			// Current working directory up two levels
			filepath.Join("..", "..", "examples"),
			// Plain relative path
			"../../examples",
			// If all else fails, look in the current directory
			"examples",
		}
		
		// Find the first valid path
		for _, path := range possiblePaths {
			cleanPath := filepath.Clean(path)
			if stat, err := os.Stat(cleanPath); err == nil && stat.IsDir() {
				baseDir = cleanPath
				break
			}
		}
		
		if baseDir == "" {
			fmt.Fprintf(os.Stderr, "Error: Could not find examples directory\n")
			fmt.Fprintf(os.Stderr, "Please run this tool from the protoc-gen-anything project directory or specify the path with -path\n")
			os.Exit(1)
		}
	}
	
	// Inform the user which examples directory we're using
	fmt.Fprintf(os.Stderr, "Using examples directory: %s\n", baseDir)
	
	var archive *txtar.Archive

	// Process examples
	if allFlag {
		archive = processAllExamples(baseDir)
	} else {
		archive = processExample(baseDir, exampleName)
	}

	// Output the archive
	output := txtar.Format(archive)
	
	if outputFile == "" {
		// Write to stdout
		os.Stdout.Write(output)
	} else {
		// Write to file
		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Successfully wrote txtar file to %s\n", outputFile)
	}
}

// processAllExamples packages all examples into a txtar archive
func processAllExamples(baseDir string) *txtar.Archive {
	archive := &txtar.Archive{
		Comment: []byte("# protoc-gen-anything examples\n# Generated with gen tool\n\n"),
	}
	
	// Get all example directories
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading examples directory: %v\n", err)
		os.Exit(1)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip non-example directories like "basic" or example support files
			if isSkippableDir(entry.Name()) {
				continue
			}
			
			// Process this example
			exampleArchive := processExample(baseDir, entry.Name())
			
			// Add a header for this example
			header := []byte(fmt.Sprintf("\n# Example: %s\n", entry.Name()))
			archive.Comment = append(archive.Comment, header...)
			
			// Add all files from this example to the main archive
			for _, file := range exampleArchive.Files {
				// Prefix files with the example name
				prefixedName := filepath.Join(entry.Name(), file.Name)
				archive.Files = append(archive.Files, txtar.File{
					Name: prefixedName,
					Data: file.Data,
				})
			}
		}
	}
	
	return archive
}

// processExample packages a single example into a txtar archive
func processExample(baseDir, exampleName string) *txtar.Archive {
	examplePath := filepath.Join(baseDir, exampleName)
	
	// Check if the example exists
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: example '%s' not found\n", exampleName)
		os.Exit(1)
	}
	
	archive := &txtar.Archive{
		Comment: []byte(fmt.Sprintf("# %s example for protoc-gen-anything\n\n", exampleName)),
	}
	
	// Walk the example directory and add all files to the archive
	err := filepath.WalkDir(examplePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories and generated files
		if d.IsDir() {
			if d.Name() == "gen" {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Read the file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		// Calculate the relative path from the example directory
		relPath, err := filepath.Rel(examplePath, path)
		if err != nil {
			return err
		}
		
		// Add the file to the archive
		archive.Files = append(archive.Files, txtar.File{
			Name: relPath,
			Data: data,
		})
		
		return nil
	})
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing example '%s': %v\n", exampleName, err)
		os.Exit(1)
	}
	
	return archive
}

// isSkippableDir returns true if a directory should be skipped
func isSkippableDir(name string) bool {
	// List of directories to skip
	skipDirs := []string{
		"basic",      // Basic example, not interesting
		"advanced",   // Advanced example, complex for demo
		"extensions", // Extension handling
		"file",       // Simple file-level template
		"helpers",    // Helpers
		"jsonschema", // We have better examples now
	}
	
	for _, skipDir := range skipDirs {
		if name == skipDir {
			return true
		}
	}
	
	return false
}