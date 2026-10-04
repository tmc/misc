package qualify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiredGates(t *testing.T) {
	err := Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("got %v", err)
	}
}

func TestPreserveExistingOutput(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	if err := os.WriteFile(marker, []byte("owned by caller"), 0644); err != nil {
		t.Fatal(err)
	}
	o := Options{Package: dir, Out: dir, TSC: "go", NPM: "go", Node: "go", Playwright: dir, Consumer: marker, Examples: []string{marker}}
	err := Run(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "create qualification directory") {
		t.Fatalf("got %v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "owned by caller" {
		t.Fatalf("marker %q, %v", got, err)
	}
}

func TestFailureHasNoPassingReceipt(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "reject")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'qualification tool rejected input\\n'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "output")
	o := Options{Package: dir, Out: out, TSC: tool, NPM: tool, Node: tool, Playwright: dir, Consumer: tool, Examples: []string{tool}}
	err := Run(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "qualification tool rejected input") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "qualification.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected receipt: %v", err)
	}
	log, err := os.ReadFile(filepath.Join(out, "qualification.log"))
	if err != nil || !strings.Contains(string(log), "rejected input") {
		t.Fatalf("log %q, %v", log, err)
	}
}
