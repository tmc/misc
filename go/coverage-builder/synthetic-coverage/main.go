// synthetic-coverage is a tool that adds synthetic coverage data to existing
// Go coverage files. This allows adding coverage information for files or
// functions that weren't originally instrumented.
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/fnv"
	"internal/coverage"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/encodecounter"
	"internal/coverage/encodemeta"
	"internal/coverage/pods"
	"internal/coverage/slicewriter"
	"internal/coverage/stringtab"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	inputDir    = flag.String("i", "", "Input coverage directory")
	outputDir   = flag.String("o", "", "Output coverage directory")
	packagePath = flag.String("pkg", "", "Package path to add synthetic coverage for")
	funcName    = flag.String("func", "", "Function name to add synthetic coverage for")
	fileName    = flag.String("file", "", "File name to add synthetic coverage for")
	lineStart   = flag.Int("line-start", 1, "Start line number")
	lineEnd     = flag.Int("line-end", 10, "End line number")
	statements  = flag.Int("statements", 1, "Number of statements")
	executed    = flag.Int("executed", 1, "Number of times executed")
	debug       = flag.Bool("debug", false, "Enable debug output")
)

func main() {
	flag.Parse()

	if *inputDir == "" || *outputDir == "" {
		log.Fatal("Must specify both -i (input) and -o (output) directories")
	}

	if *packagePath == "" || *fileName == "" || *funcName == "" {
		log.Fatal("Must specify -pkg, -file, and -func")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Read existing coverage data
	pods, err := pods.CollectPods([]string{*inputDir}, true)
	if err != nil {
		log.Fatalf("Failed to collect pods: %v", err)
	}

	if len(pods) == 0 {
		log.Fatal("No coverage data found in input directory")
	}

	// Process each pod
	for _, pod := range pods {
		if err := processPod(pod); err != nil {
			log.Fatalf("Failed to process pod: %v", err)
		}
	}
}

func processPod(pod pods.Pod) error {
	// Read the meta-data file
	metaReader, err := decodemeta.NewCoverageMetaFileReader(pod.MetaFile, nil)
	if err != nil {
		return fmt.Errorf("failed to read meta file: %w", err)
	}

	// Create synthetic meta-data
	syntheticMeta := createSyntheticMeta(metaReader)

	// Write new meta-data file
	newMetaPath := filepath.Join(*outputDir, filepath.Base(pod.MetaFile))
	if err := writeSyntheticMeta(newMetaPath, metaReader, syntheticMeta); err != nil {
		return fmt.Errorf("failed to write synthetic meta: %w", err)
	}

	// Process counter files
	for _, counterFile := range pod.CounterDataFiles {
		if err := processCoutnerFile(counterFile, syntheticMeta); err != nil {
			return fmt.Errorf("failed to process counter file %s: %w", counterFile, err)
		}
	}

	return nil
}

type syntheticMetaData struct {
	pkgID    uint32
	pkgPath  string
	funcID   uint32
	funcName string
	units    []coverage.CoverableUnit
}

func createSyntheticMeta(reader *decodemeta.CoverageMetaFileReader) *syntheticMetaData {
	// Find the next available package ID
	maxPkgID := uint32(0)
	numPkgs := reader.NumPackages()
	for i := uint32(0); i < numPkgs; i++ {
		if i > maxPkgID {
			maxPkgID = i
		}
	}

	return &syntheticMetaData{
		pkgID:    maxPkgID + 1,
		pkgPath:  *packagePath,
		funcID:   0, // First function in synthetic package
		funcName: *funcName,
		units: []coverage.CoverableUnit{
			{
				StLine:  uint32(*lineStart),
				StCol:   1,
				EnLine:  uint32(*lineEnd),
				EnCol:   1,
				NxStmts: uint32(*statements),
			},
		},
	}
}

func writeSyntheticMeta(outPath string, reader *decodemeta.CoverageMetaFileReader, synthetic *syntheticMetaData) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := encodemeta.NewCoverageMetaFileWriter(outFile, nil)

	// Copy existing packages
	numPkgs := reader.NumPackages()
	for i := uint32(0); i < numPkgs; i++ {
		pd := reader.GetPackageDecoder(i)
		if err := copyPackage(writer, pd); err != nil {
			return fmt.Errorf("failed to copy package %d: %w", i, err)
		}
	}

	// Add synthetic package
	if err := writeSyntheticPackage(writer, synthetic); err != nil {
		return fmt.Errorf("failed to write synthetic package: %w", err)
	}

	// Finalize the meta file
	if _, err := writer.Emit(); err != nil {
		return fmt.Errorf("failed to emit meta file: %w", err)
	}

	return nil
}

func copyPackage(writer *encodemeta.CoverageMetaFileWriter, pd *decodemeta.CoverageMetaDataDecoder) error {
	// Create package encoder
	pe := writer.AddPackage(pd.PackagePath())

	// Copy functions
	numFuncs := pd.NumFuncs()
	for i := uint32(0); i < numFuncs; i++ {
		fname, file, units, lit := pd.ReadFunc(i, nil)
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

func writeSyntheticPackage(writer *encodemeta.CoverageMetaFileWriter, synthetic *syntheticMetaData) error {
	// Add synthetic package
	pe := writer.AddPackage(synthetic.pkgPath)

	// Add synthetic function
	fe := pe.AddFunc(coverage.FuncDesc{
		Funcname: synthetic.funcName,
		Srcfile:  *fileName,
		Units:    synthetic.units,
		Lit:      false,
	})
	fe.Emit()

	return nil
}

func processCoutnerFile(counterPath string, synthetic *syntheticMetaData) error {
	// Read existing counter file
	file, err := os.Open(counterPath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, err := decodecounter.NewCounterDataReader(counterPath, file)
	if err != nil {
		return err
	}

	// Create output counter file
	outPath := filepath.Join(*outputDir, filepath.Base(counterPath))
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create writer with same flavor
	cdfw := encodecounter.NewCoverageDataWriter(outFile, reader.CFlavor())

	// Write header with same meta hash
	if err := cdfw.Write(reader.MetaHash(), reader.OsArgs(), &counterCopier{
		reader:    reader,
		synthetic: synthetic,
	}); err != nil {
		return fmt.Errorf("failed to write counter data: %w", err)
	}

	return nil
}

type counterCopier struct {
	reader    *decodecounter.CounterDataReader
	synthetic *syntheticMetaData
}

func (cc *counterCopier) VisitFuncs(fn encodecounter.CounterVisitorFn) error {
	// First, copy existing counters
	var copyErr error
	cc.reader.VisitFuncs(func(data decodecounter.FuncPayload) {
		if copyErr != nil {
			return
		}
		copyErr = fn(data.PkgIdx, data.FuncIdx, data.Counters)
	})
	if copyErr != nil {
		return copyErr
	}

	// Add synthetic counters
	syntheticCounters := make([]uint32, len(cc.synthetic.units))
	for i := range syntheticCounters {
		syntheticCounters[i] = uint32(*executed)
	}

	return fn(cc.synthetic.pkgID, cc.synthetic.funcID, syntheticCounters)
}

// Helper function to compute meta-data hash
func computeMetaHash(packages []coverage.MetaSymbolHeader) [16]byte {
	h := fnv.New128a()
	for _, p := range packages {
		h.Write([]byte(p.PkgPath))
		binary.Write(h, binary.LittleEndian, p.NumFuncs)
	}
	var hash [16]byte
	copy(hash[:], h.Sum(nil))
	return hash
}