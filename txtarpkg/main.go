package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/tools/txtar"
	"strings"
)

// Command-line flags
var (
	outputFile string
	allFlag    bool
	dirPath    string
	comment    string
)

func init() {
	flag.StringVar(&outputFile, "o", "", "Output file (defaults to stdout)")
	flag.BoolVar(&allFlag, "a", false, "Package all directories in the source path")
	flag.StringVar(&dirPath, "path", ".", "Path to the source directory to package")
	flag.StringVar(&comment, "comment", "# Generated with txtarpkg", "Comment to include at the top of the archive")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: txtarpkg [options] [directory_name]\n\n")
		fmt.Fprintf(os.Stderr, "txtarpkg packages directories into txtar archives for easy sharing.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  txtarpkg openapi                  # Package 'openapi' subdirectory to stdout\n")
		fmt.Fprintf(os.Stderr, "  txtarpkg graphql -o graphql.txtar # Save the 'graphql' subdirectory to a file\n")
		fmt.Fprintf(os.Stderr, "  txtarpkg -a -o all.txtar          # Package all subdirectories into a single file\n")
		fmt.Fprintf(os.Stderr, "  txtarpkg -path=/other/path ui     # Specify a different source directory\n")
	}
}

func main() {
	flag.Parse()
	
	// Get the directory name from positional arguments
	var dirName string
	args := flag.Args()
	if len(args) > 0 {
		dirName = args[0]
	}
	
	// Handle special cases
	if dirName == "all" {
		allFlag = true
		dirName = ""
	}
	
	// Validate arguments
	if dirName == "" && !allFlag {
		fmt.Fprintf(os.Stderr, "Error: must specify a directory name or use -a for all directories\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check if the source directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Source directory '%s' does not exist\n", dirPath)
		os.Exit(1)
	}
	
	// Inform the user which source directory we're using
	fmt.Fprintf(os.Stderr, "Using source directory: %s\n", dirPath)
	
	var archive *txtar.Archive
	
	// Process directories
	if allFlag {
		archive = processAllDirectories(dirPath)
	} else {
		archive = processDirectory(dirPath, dirName)
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

// processAllDirectories packages all directories into a txtar archive
func processAllDirectories(basePath string) *txtar.Archive {
	archive := &txtar.Archive{
		Comment: []byte(comment + "\n\n"),
	}
	
	// Get all directories
	entries, err := os.ReadDir(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source directory: %v\n", err)
		os.Exit(1)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip hidden directories
			if entry.Name()[0] == '.' {
				continue
			}
			
			// Process this directory
			dirArchive := processDirectory(basePath, entry.Name())
			
			// Add a header for this directory
			header := []byte(fmt.Sprintf("\n# Directory: %s\n", entry.Name()))
			archive.Comment = append(archive.Comment, header...)
			
			// Add all files from this directory to the main archive
			for _, file := range dirArchive.Files {
				// Prefix files with the directory name
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

// processDirectory packages a single directory into a txtar archive
func processDirectory(basePath, dirName string) *txtar.Archive {
	dirPath := filepath.Join(basePath, dirName)
	
	// Check if the directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory '%s' not found in %s\n", dirName, basePath)
		os.Exit(1)
	}
	
	archive := &txtar.Archive{
		Comment: []byte(fmt.Sprintf("# %s\n\n", dirName)),
	}
	
	// Walk the directory and add all files to the archive
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if d.IsDir() {
			// Skip hidden directories
			if len(d.Name()) > 0 && d.Name()[0] == '.' {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Skip hidden files
		if len(d.Name()) > 0 && d.Name()[0] == '.' {
			return nil
		}
		
		// Read the file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		// Calculate the relative path from the directory
		relPath, err := filepath.Rel(dirPath, path)
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
		fmt.Fprintf(os.Stderr, "Error processing directory '%s': %v\n", dirName, err)
		os.Exit(1)
	}
	
	return archive
}

// escapeTxtarContent escapes txtar marker lines and backslashes in file content
func escapeTxtarContent(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	
	for _, line := range lines {
		// Escape existing backslashes first
		if strings.HasPrefix(line, "\\") {
			result = append(result, "\\"+line)
		} else if strings.HasPrefix(line, "-- ") && strings.HasSuffix(line, " --") {
			// Escape txtar file markers with backslash
			result = append(result, "\\"+line)
		} else {
			result = append(result, line)
		}
	}
	
	return []byte(strings.Join(result, "\n"))
}

// unescapeTxtarContent unescapes previously escaped txtar marker lines and backslashes
func unescapeTxtarContent(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	
	for _, line := range lines {
		// Check for escaped content
		if strings.HasPrefix(line, "\\-- ") && strings.HasSuffix(line, " --") {
			// Unescape txtar marker by removing leading backslash
			result = append(result, line[1:])
		} else if strings.HasPrefix(line, "\\\\") {
			// Unescape double backslash to single backslash
			result = append(result, line[1:])
		} else {
			result = append(result, line)
		}
	}
	
	return []byte(strings.Join(result, "\n"))
}