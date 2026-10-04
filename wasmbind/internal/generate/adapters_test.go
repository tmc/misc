package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAdapters(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteAdapters(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = WriteAdapters(dir); err != nil {
		t.Fatal(err)
	}
	tests, err := os.ReadFile("testdata/adapter_test.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "adapters_test.go"), tests, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/adapters\n\ngo 1.27.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-race", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("adapter tests: %v\n%s", err, output)
	}
	if err = os.WriteFile(path, []byte("package user\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = WriteAdapters(dir); err == nil {
		t.Fatal("overwrote user file")
	}
}

func ExampleAdapterSource() {
	source, err := AdapterSource()
	fmt.Println(err == nil && len(source) > 0)
	// Output: true
}
