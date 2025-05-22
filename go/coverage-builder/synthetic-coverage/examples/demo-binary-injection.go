// demo-binary-injection.go - Complete demonstration of GOCOVERDIR binary injection
package main

import (
	"fmt"
	"internal/coverage"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/encodecounter"
	"internal/coverage/encodemeta"
	"internal/coverage/pods"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	workDir := "/tmp/binary-injection-demo"
	os.RemoveAll(workDir)
	os.MkdirAll(workDir, 0755)

	// Create test application
	if err := createTestApp(workDir); err != nil {
		log.Fatal(err)
	}

	// Run tests with coverage
	coverageDir := filepath.Join(workDir, "coverage")
	if err := runTestsWithCoverage(workDir, coverageDir); err != nil {
		log.Fatal(err)
	}

	// Show original coverage
	fmt.Println("\n=== Original Coverage ===")
	showCoverage(coverageDir)

	// Inject synthetic coverage
	syntheticDir := filepath.Join(workDir, "synthetic")
	if err := injectSyntheticCoverage(coverageDir, syntheticDir); err != nil {
		log.Fatal(err)
	}

	// Show coverage with synthetic data
	fmt.Println("\n=== Coverage with Synthetic Data ===")
	showCoverage(syntheticDir)

	fmt.Printf("\nDemo complete! Files in: %s\n", workDir)
}

func createTestApp(workDir string) error {
	appDir := filepath.Join(workDir, "testapp")
	os.MkdirAll(appDir, 0755)

	// go.mod
	goMod := `module testapp
go 1.20`
	if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// main.go
	mainGo := `package main

import "fmt"

func main() {
    result := ProcessData("test")
    fmt.Println(result)
}

func ProcessData(input string) string {
    if input == "" {
        return "empty"
    }
    return fmt.Sprintf("processed: %s", input)
}

func UntestedFunction() int {
    // This function won't be covered by tests
    return 42
}
`
	if err := os.WriteFile(filepath.Join(appDir, "main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	// main_test.go
	testGo := `package main

import "testing"

func TestProcessData(t *testing.T) {
    tests := []struct {
        input string
        want  string
    }{
        {"test", "processed: test"},
        {"", "empty"},
    }
    
    for _, tt := range tests {
        if got := ProcessData(tt.input); got != tt.want {
            t.Errorf("ProcessData(%q) = %q, want %q", tt.input, got, tt.want)
        }
    }
}
`
	return os.WriteFile(filepath.Join(appDir, "main_test.go"), []byte(testGo), 0644)
}

func runTestsWithCoverage(workDir, coverageDir string) error {
	appDir := filepath.Join(workDir, "testapp")
	os.MkdirAll(coverageDir, 0755)

	cmd := exec.Command("go", "test", "-cover", ".")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverageDir)
	
	output, err := cmd.CombinedOutput()
	fmt.Printf("Test output: %s\n", output)
	return err
}

func showCoverage(coverageDir string) {
	// Show function coverage
	cmd := exec.Command("go", "tool", "covdata", "func", "-i="+coverageDir)
	output, _ := cmd.Output()
	fmt.Printf("%s", output)

	// Show percentage
	cmd = exec.Command("go", "tool", "covdata", "percent", "-i="+coverageDir)
	output, _ = cmd.Output()
	fmt.Printf("Total coverage: %s", output)
}

func injectSyntheticCoverage(inputDir, outputDir string) error {
	os.MkdirAll(outputDir, 0755)

	// Collect pods
	pods, err := pods.CollectPods([]string{inputDir}, true)
	if err != nil {
		return fmt.Errorf("collecting pods: %w", err)
	}

	// Process first pod only for simplicity
	if len(pods) == 0 {
		return fmt.Errorf("no coverage data found")
	}

	pod := pods[0]

	// Read meta file
	metaReader, err := decodemeta.NewCoverageMetaFileReader(pod.MetaFile, nil)
	if err != nil {
		return fmt.Errorf("reading meta: %w", err)
	}

	// Create output meta file
	outMetaPath := filepath.Join(outputDir, filepath.Base(pod.MetaFile))
	outMetaFile, err := os.Create(outMetaPath)
	if err != nil {
		return err
	}
	defer outMetaFile.Close()

	writer := encodemeta.NewCoverageMetaFileWriter(outMetaFile, nil)

	// Copy existing packages
	maxPkgID := uint32(0)
	numPkgs := metaReader.NumPackages()
	for i := uint32(0); i < numPkgs; i++ {
		pd := metaReader.GetPackageDecoder(i)
		if err := copyPackage(writer, pd); err != nil {
			return err
		}
		if i > maxPkgID {
			maxPkgID = i
		}
	}

	// Add synthetic packages
	synthetic := []struct {
		pkg     string
		file    string
		fn      string
		lines   int
		stmts   int
		covered int
	}{
		{
			pkg:     "testapp/generated",
			file:    "generated.go", 
			fn:      "GeneratedCode",
			lines:   100,
			stmts:   50,
			covered: 1,
		},
		{
			pkg:     "github.com/vendor/somelib",
			file:    "vendor.go",
			fn:      "VendorFunction", 
			lines:   200,
			stmts:   80,
			covered: 1,
		},
	}

	// Add each synthetic package
	for i, syn := range synthetic {
		pkgID := maxPkgID + uint32(i) + 1
		if err := addSyntheticPackage(writer, syn.pkg, syn.file, syn.fn, syn.lines, syn.stmts); err != nil {
			return err
		}
	}

	// Emit meta file
	metaHash, err := writer.Emit()
	if err != nil {
		return err
	}

	// Process counter files
	for _, counterFile := range pod.CounterDataFiles {
		if err := processCounterFile(counterFile, outputDir, metaHash, maxPkgID+1, synthetic); err != nil {
			return err
		}
	}

	return nil
}

func copyPackage(writer *encodemeta.CoverageMetaFileWriter, pd *decodemeta.CoverageMetaDataDecoder) error {
	pe := writer.AddPackage(pd.PackagePath())
	
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

func addSyntheticPackage(writer *encodemeta.CoverageMetaFileWriter, pkg, file, fn string, lines, stmts int) error {
	pe := writer.AddPackage(pkg)
	
	units := []coverage.CoverableUnit{
		{
			StLine:  1,
			StCol:   1,
			EnLine:  uint32(lines),
			EnCol:   1,
			NxStmts: uint32(stmts),
		},
	}
	
	fe := pe.AddFunc(coverage.FuncDesc{
		Funcname: fn,
		Srcfile:  file,
		Units:    units,
		Lit:      false,
	})
	fe.Emit()
	
	return nil
}

func processCounterFile(inputPath, outputDir string, metaHash [16]byte, firstSynPkgID uint32, synthetic []struct{pkg, file, fn string; lines, stmts, covered int}) error {
	// Read input
	inFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	reader, err := decodecounter.NewCounterDataReader(inputPath, inFile)
	if err != nil {
		return err
	}

	// Create output
	outPath := filepath.Join(outputDir, filepath.Base(inputPath))
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := encodecounter.NewCoverageDataWriter(outFile, reader.CFlavor())

	// Create visitor
	visitor := &syntheticVisitor{
		reader:        reader,
		firstSynPkgID: firstSynPkgID,
		synthetic:     synthetic,
	}

	return writer.Write(metaHash, reader.OsArgs(), visitor)
}

type syntheticVisitor struct {
	reader        *decodecounter.CounterDataReader
	firstSynPkgID uint32
	synthetic     []struct{pkg, file, fn string; lines, stmts, covered int}
}

func (v *syntheticVisitor) VisitFuncs(fn encodecounter.CounterVisitorFn) error {
	// Copy existing
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

	// Add synthetic
	for i, syn := range v.synthetic {
		pkgID := v.firstSynPkgID + uint32(i)
		counters := []uint32{uint32(syn.covered)}
		if err := fn(pkgID, 0, counters); err != nil {
			return err
		}
	}

	return nil
}

// Helper to copy a file
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}