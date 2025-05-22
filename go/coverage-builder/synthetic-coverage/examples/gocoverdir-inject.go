// gocoverdir-inject demonstrates direct GOCOVERDIR binary format manipulation
package main

import (
	"flag"
	"fmt"
	"internal/coverage"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/encodecounter"
	"internal/coverage/encodemeta"
	"internal/coverage/pods"
	"log"
	"os"
	"path/filepath"
)

var (
	inputDir    = flag.String("i", "", "Input GOCOVERDIR path")
	outputDir   = flag.String("o", "", "Output GOCOVERDIR path")
	fakePackage = flag.String("pkg", "github.com/example/fake", "Fake package to inject")
	fakeFile    = flag.String("file", "synthetic.go", "Fake file name")
	fakeFunc    = flag.String("func", "SyntheticFunction", "Fake function name")
	lineStart   = flag.Int("line-start", 1, "Start line")
	lineEnd     = flag.Int("line-end", 20, "End line")
	statements  = flag.Int("statements", 10, "Number of statements")
	executed    = flag.Int("executed", 1, "Execution count")
)

type SyntheticCoverage struct {
	Package    string
	File       string
	Function   string
	LineStart  int
	LineEnd    int
	Statements int
	Executed   int
}

func main() {
	flag.Parse()

	if *inputDir == "" || *outputDir == "" {
		log.Fatal("Must specify both -i (input) and -o (output) directories")
	}

	synthetic := SyntheticCoverage{
		Package:    *fakePackage,
		File:       *fakeFile,
		Function:   *fakeFunc,
		LineStart:  *lineStart,
		LineEnd:    *lineEnd,
		Statements: *statements,
		Executed:   *executed,
	}

	if err := injectSyntheticCoverage(*inputDir, *outputDir, synthetic); err != nil {
		log.Fatalf("Failed to inject synthetic coverage: %v", err)
	}

	log.Printf("Successfully injected synthetic coverage for %s into %s", synthetic.Package, *outputDir)
}

func injectSyntheticCoverage(inputDir, outputDir string, synthetic SyntheticCoverage) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Collect coverage pods from input directory
	pods, err := pods.CollectPods([]string{inputDir}, true)
	if err != nil {
		return fmt.Errorf("collecting pods: %w", err)
	}

	if len(pods) == 0 {
		return fmt.Errorf("no coverage data found in %s", inputDir)
	}

	// Process each pod
	for _, pod := range pods {
		if err := processPod(pod, outputDir, synthetic); err != nil {
			return fmt.Errorf("processing pod: %w", err)
		}
	}

	return nil
}

func processPod(pod pods.Pod, outputDir string, synthetic SyntheticCoverage) error {
	// Read meta-data file
	metaReader, err := decodemeta.NewCoverageMetaFileReader(pod.MetaFile, nil)
	if err != nil {
		return fmt.Errorf("reading meta file: %w", err)
	}

	// Create output meta file  
	outMetaPath := filepath.Join(outputDir, filepath.Base(pod.MetaFile))
	outMetaFile, err := os.Create(outMetaPath)
	if err != nil {
		return fmt.Errorf("creating output meta file: %w", err)
	}
	defer outMetaFile.Close()

	metaWriter := encodemeta.NewCoverageMetaFileWriter(outMetaFile, nil)

	// Copy existing packages
	syntheticPkgID := uint32(metaReader.NumPackages())
	for i := uint32(0); i < syntheticPkgID; i++ {
		if err := copyPackage(metaWriter, metaReader.GetPackageDecoder(i)); err != nil {
			return fmt.Errorf("copying package %d: %w", i, err)
		}
	}

	// Add synthetic package
	if err := addSyntheticPackage(metaWriter, synthetic); err != nil {
		return fmt.Errorf("adding synthetic package: %w", err)
	}

	// Emit meta file
	metaHash, err := metaWriter.Emit()
	if err != nil {
		return fmt.Errorf("emitting meta file: %w", err)
	}

	// Process counter files
	for _, counterFile := range pod.CounterDataFiles {
		outCounterPath := filepath.Join(outputDir, filepath.Base(counterFile))
		if err := processCounterFile(counterFile, outCounterPath, metaHash, syntheticPkgID, synthetic); err != nil {
			return fmt.Errorf("processing counter file %s: %w", counterFile, err)
		}
	}

	return nil
}

func copyPackage(writer *encodemeta.CoverageMetaFileWriter, decoder *decodemeta.CoverageMetaDataDecoder) error {
	pe := writer.AddPackage(decoder.PackagePath())
	
	numFuncs := decoder.NumFuncs()
	for i := uint32(0); i < numFuncs; i++ {
		fname, file, units, lit := decoder.ReadFunc(i, nil)
		fe := pe.AddFunc(coverage.FuncDesc{
			Funcname: fname,
			Srcfile:  file,
			Units:    units,
			Lit:      lit,
		})
		fe.Emit()
	}
	
	return nil
}

func addSyntheticPackage(writer *encodemeta.CoverageMetaFileWriter, synthetic SyntheticCoverage) error {
	pe := writer.AddPackage(synthetic.Package)
	
	units := []coverage.CoverableUnit{
		{
			StLine:  uint32(synthetic.LineStart),
			StCol:   1,
			EnLine:  uint32(synthetic.LineEnd),
			EnCol:   1,
			NxStmts: uint32(synthetic.Statements),
		},
	}
	
	fe := pe.AddFunc(coverage.FuncDesc{
		Funcname: synthetic.Function,
		Srcfile:  synthetic.File,
		Units:    units,
		Lit:      false,
	})
	fe.Emit()
	
	return nil
}

func processCounterFile(inputPath, outputPath string, metaHash [16]byte, syntheticPkgID uint32, synthetic SyntheticCoverage) error {
	// Read input counter file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer inFile.Close()

	reader, err := decodecounter.NewCounterDataReader(inputPath, inFile)
	if err != nil {
		return fmt.Errorf("creating counter reader: %w", err)
	}

	// Create output counter file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	writer := encodecounter.NewCoverageDataWriter(outFile, reader.CFlavor())

	// Write with visitor that includes synthetic data
	visitor := &syntheticCounterVisitor{
		reader:        reader,
		syntheticPkg:  syntheticPkgID,
		syntheticFunc: 0,
		synthetic:     synthetic,
	}

	if err := writer.Write(metaHash, reader.OsArgs(), visitor); err != nil {
		return fmt.Errorf("writing counter data: %w", err)
	}

	return nil
}

type syntheticCounterVisitor struct {
	reader        *decodecounter.CounterDataReader
	syntheticPkg  uint32
	syntheticFunc uint32
	synthetic     SyntheticCoverage
}

func (v *syntheticCounterVisitor) VisitFuncs(fn encodecounter.CounterVisitorFn) error {
	// First, copy existing counters
	var copyErr error
	v.reader.VisitFuncs(func(data decodecounter.FuncPayload) {
		if copyErr != nil {
			return
		}
		copyErr = fn(data.PkgIdx, data.FuncIdx, data.Counters)
	})
	if copyErr != nil {
		return copyErr
	}

	// Add synthetic counters
	counters := make([]uint32, 1) // One unit
	counters[0] = uint32(v.synthetic.Executed)
	
	return fn(v.syntheticPkg, v.syntheticFunc, counters)
}