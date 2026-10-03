package generate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func fixtureAPI(t *testing.T) *schema.API {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "api.go"), []byte("package fixture\n//wasmbind:export echo\nfunc Echo(v string)string{return v}\n"), 0644)
	a, err := schema.Load(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestPublication(t *testing.T) {
	a := fixtureAPI(t)
	goOut, out := filepath.Join(a.ModuleDir, "cmd", "bind"), filepath.Join(a.ModuleDir, "web")
	p, err := Write(context.Background(), a, goOut, out, "", true)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m schema.Manifest
	if err := json.Unmarshal(original, &m); err != nil {
		t.Fatal(err)
	}
	if m.Compiler == "" || m.Artifacts["module.wasm"] == "" {
		t.Fatal("missing build provenance")
	}
	if _, err := Write(context.Background(), a, goOut, out, "", true); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(p)
	if string(original) != string(again) {
		t.Fatal("build is nondeterministic")
	}
	// A generate after a build removes stale owned runtime artifacts.
	a.Compiler = ""
	a.Artifacts = nil
	if _, err := Write(context.Background(), a, goOut, out, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "module.wasm")); !os.IsNotExist(err) {
		t.Fatal("stale wasm survived generation")
	}
	os.WriteFile(filepath.Join(out, "bindings.js"), []byte("user edit"), 0644)
	if _, err := Write(context.Background(), a, goOut, out, "", false); err == nil || !strings.Contains(err.Error(), "modified artifact") {
		t.Fatalf("got %v", err)
	}
}
func TestUserFileAndPaths(t *testing.T) {
	a := fixtureAPI(t)
	goOut, out := filepath.Join(a.ModuleDir, "cmd", "bind"), filepath.Join(a.ModuleDir, "web")
	os.MkdirAll(out, 0755)
	os.WriteFile(filepath.Join(out, "bindings.js"), []byte("user file"), 0644)
	if _, err := Write(context.Background(), a, goOut, out, "", false); err == nil {
		t.Fatal("overwrote user file")
	}
	if _, err := Write(context.Background(), a, a.Dir, out, "", false); err == nil {
		t.Fatal("accepted input directory")
	}
	if _, err := Write(context.Background(), a, t.TempDir(), out, "", false); err == nil {
		t.Fatal("accepted foreign module directory")
	}
}

func TestSchemaGolden(t *testing.T) {
	a := fixtureAPI(t)
	data, err := json.MarshalIndent(a.Manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "schema.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(want) {
		t.Fatalf("schema differs from golden:\n%s", data)
	}
}

func TestFailedBuildPreservesPriorBundle(t *testing.T) {
	a := fixtureAPI(t)
	goOut, out := filepath.Join(a.ModuleDir, "cmd", "bind"), filepath.Join(a.ModuleDir, "web")
	path, err := Write(context.Background(), a, goOut, out, "", true)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Introduce a compile error after validation to force the staged build to fail.
	if err := os.WriteFile(filepath.Join(a.Dir, "broken.go"), []byte("package fixture\nfunc Broken(){undefined()}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(context.Background(), a, goOut, out, "", true); err == nil {
		t.Fatal("broken source built")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed build published a new manifest")
	}
	if _, err := owned(goOut, out); err != nil {
		t.Fatal(err)
	}
}
