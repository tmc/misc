package pack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

func TestPackage(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/packed\n\ngo 1.27.0\n")
	write("api.go", "package api\n//wasmbind:export echo\nfunc Echo(s string)string{return s}\n")
	write("license.txt", "Explicit fixture license\n")
	a, err := schema.Load(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "web")
	if _, err := generate.Write(context.Background(), a, filepath.Join(root, "cmd", "bind"), bundle, "", true); err != nil {
		t.Fatal(err)
	}
	opt := Options{Bundle: bundle, Out: filepath.Join(root, "package"), Name: "@fixture/echo", Version: "1.0.0", LicenseFile: filepath.Join(root, "license.txt")}
	path, err := Write(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	if _, err := Write(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if string(original) != string(again) {
		t.Fatal("nondeterministic package")
	}
	data, _ := os.ReadFile(filepath.Join(opt.Out, "package.json"))
	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg["type"] != "module" || pkg["scripts"] != nil {
		t.Fatalf("invalid package metadata: %s", data)
	}
	license, _ := os.ReadFile(filepath.Join(opt.Out, "LICENSE"))
	if string(license) != "Explicit fixture license\n" {
		t.Fatal("license changed")
	}
	if err := os.WriteFile(filepath.Join(opt.Out, "bindings.js"), []byte("user edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(context.Background(), opt); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("got %v", err)
	}
	opt.Out = filepath.Join(root, "fresh")
	if err := os.WriteFile(filepath.Join(bundle, "module.wasm"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(context.Background(), opt); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(opt.Out, "package-manifest.json")); !os.IsNotExist(err) {
		t.Fatal("published invalid bundle")
	}
}

func TestIdentity(t *testing.T) {
	for _, opt := range []Options{{Name: "UPPER", Version: "1.0.0", LicenseFile: "x"}, {Name: "valid", Version: "next", LicenseFile: "x"}, {Name: "valid", Version: "1.0.0"}} {
		if _, err := Write(context.Background(), opt); err == nil {
			t.Fatalf("accepted %+v", opt)
		}
	}
}
