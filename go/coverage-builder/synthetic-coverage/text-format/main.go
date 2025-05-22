// synthetic-coverage-text adds synthetic coverage data to Go coverage text files.
// This is a simpler approach that works with the standard text format.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	inputFile   = flag.String("i", "", "Input coverage text file")
	outputFile  = flag.String("o", "", "Output coverage text file")
	syntheticFile = flag.String("s", "", "File with synthetic coverage lines (format: file:startLine.startCol,endLine.endCol statements count)")
	merge       = flag.Bool("merge", false, "Merge with existing coverage instead of adding")
	debug       = flag.Bool("debug", false, "Enable debug output")
)

// CoverageLine represents a single line of coverage data
type CoverageLine struct {
	File       string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	Statements int
	Count      int
	Original   string // Original line text
}

// ParseCoverageLine parses a coverage line in the format:
// filename:start.line,end.line statements count
func ParseCoverageLine(line string) (*CoverageLine, error) {
	// Skip empty lines and mode line
	if line == "" || strings.HasPrefix(line, "mode:") {
		return nil, nil
	}

	// Regular expression to parse coverage lines
	re := regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+)\s+(\d+)\s+(\d+)$`)
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("invalid coverage line format: %s", line)
	}

	cl := &CoverageLine{
		File:     matches[1],
		Original: line,
	}

	// Parse numeric values
	var err error
	cl.StartLine, err = strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}
	cl.StartCol, err = strconv.Atoi(matches[3])
	if err != nil {
		return nil, err
	}
	cl.EndLine, err = strconv.Atoi(matches[4])
	if err != nil {
		return nil, err
	}
	cl.EndCol, err = strconv.Atoi(matches[5])
	if err != nil {
		return nil, err
	}
	cl.Statements, err = strconv.Atoi(matches[6])
	if err != nil {
		return nil, err
	}
	cl.Count, err = strconv.Atoi(matches[7])
	if err != nil {
		return nil, err
	}

	return cl, nil
}

// FormatCoverageLine formats a coverage line for output
func FormatCoverageLine(cl *CoverageLine) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d %d",
		cl.File, cl.StartLine, cl.StartCol,
		cl.EndLine, cl.EndCol, cl.Statements, cl.Count)
}

// CoverageKey creates a unique key for a coverage line for merging
func CoverageKey(cl *CoverageLine) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d",
		cl.File, cl.StartLine, cl.StartCol, cl.EndLine, cl.EndCol)
}

func main() {
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		log.Fatal("Must specify both -i (input) and -o (output) files")
	}

	// Read existing coverage
	existingCoverage, mode, err := readCoverageFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Read synthetic coverage if provided
	var syntheticCoverage []CoverageLine
	if *syntheticFile != "" {
		syntheticCoverage, err = readSyntheticFile(*syntheticFile)
		if err != nil {
			log.Fatalf("Failed to read synthetic file: %v", err)
		}
	} else {
		// Read from stdin
		syntheticCoverage, err = readSyntheticFromStdin()
		if err != nil {
			log.Fatalf("Failed to read synthetic data from stdin: %v", err)
		}
	}

	// Merge or add coverage
	var finalCoverage []CoverageLine
	if *merge {
		finalCoverage = mergeCoverage(existingCoverage, syntheticCoverage)
	} else {
		finalCoverage = append(existingCoverage, syntheticCoverage...)
	}

	// Sort coverage lines
	sortCoverage(finalCoverage)

	// Write output
	if err := writeCoverageFile(*outputFile, mode, finalCoverage); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	if *debug {
		log.Printf("Processed %d existing + %d synthetic = %d total coverage lines",
			len(existingCoverage), len(syntheticCoverage), len(finalCoverage))
	}
}

func readCoverageFile(filename string) ([]CoverageLine, string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	return readCoverage(file)
}

func readCoverage(r io.Reader) ([]CoverageLine, string, error) {
	scanner := bufio.NewScanner(r)
	var lines []CoverageLine
	var mode string

	for scanner.Scan() {
		line := scanner.Text()
		
		// Handle mode line
		if strings.HasPrefix(line, "mode:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
			continue
		}

		// Parse coverage line
		cl, err := ParseCoverageLine(line)
		if err != nil {
			if *debug {
				log.Printf("Skipping invalid line: %s", line)
			}
			continue
		}
		if cl != nil {
			lines = append(lines, *cl)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, mode, err
	}

	return lines, mode, nil
}

func readSyntheticFile(filename string) ([]CoverageLine, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readSynthetic(file)
}

func readSyntheticFromStdin() ([]CoverageLine, error) {
	return readSynthetic(os.Stdin)
}

func readSynthetic(r io.Reader) ([]CoverageLine, error) {
	scanner := bufio.NewScanner(r)
	var lines []CoverageLine

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cl, err := ParseCoverageLine(line)
		if err != nil {
			return nil, fmt.Errorf("invalid synthetic line: %s", line)
		}
		if cl != nil {
			lines = append(lines, *cl)
		}
	}

	return lines, scanner.Err()
}

func mergeCoverage(existing, synthetic []CoverageLine) []CoverageLine {
	// Create map of existing coverage
	coverageMap := make(map[string]*CoverageLine)
	for i := range existing {
		key := CoverageKey(&existing[i])
		coverageMap[key] = &existing[i]
	}

	// Merge synthetic coverage
	for i := range synthetic {
		key := CoverageKey(&synthetic[i])
		if existing, ok := coverageMap[key]; ok {
			// Merge counts (use max or sum based on mode)
			if synthetic[i].Count > existing.Count {
				existing.Count = synthetic[i].Count
			}
		} else {
			// Add new coverage
			coverageMap[key] = &synthetic[i]
		}
	}

	// Convert back to slice
	var result []CoverageLine
	for _, cl := range coverageMap {
		result = append(result, *cl)
	}

	return result
}

func sortCoverage(lines []CoverageLine) {
	sort.Slice(lines, func(i, j int) bool {
		// Sort by file, then line, then column
		if lines[i].File != lines[j].File {
			return lines[i].File < lines[j].File
		}
		if lines[i].StartLine != lines[j].StartLine {
			return lines[i].StartLine < lines[j].StartLine
		}
		if lines[i].StartCol != lines[j].StartCol {
			return lines[i].StartCol < lines[j].StartCol
		}
		if lines[i].EndLine != lines[j].EndLine {
			return lines[i].EndLine < lines[j].EndLine
		}
		return lines[i].EndCol < lines[j].EndCol
	})
}

func writeCoverageFile(filename, mode string, lines []CoverageLine) error {
	// Create directory if needed
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write mode line
	if _, err := fmt.Fprintf(file, "mode: %s\n", mode); err != nil {
		return err
	}

	// Write coverage lines
	for _, cl := range lines {
		if _, err := fmt.Fprintln(file, FormatCoverageLine(&cl)); err != nil {
			return err
		}
	}

	return nil
}