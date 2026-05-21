package webkithost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMIME(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"wanix.debug.wasm", "application/wasm"},
		{"wanix.js", "text/javascript"},
		{"index.html", "text/html; charset=utf-8"},
		{"style.css", "text/css"},
		{"manifest.json", "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MIME(tt.name); got != tt.want {
				t.Fatalf("MIME(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestAssetsOpenBootstrap(t *testing.T) {
	asset, err := (Assets{}).Open("/")
	if err != nil {
		t.Fatal(err)
	}
	if asset.MIME != "text/html; charset=utf-8" {
		t.Fatalf("MIME = %q", asset.MIME)
	}
	if len(asset.Data) == 0 {
		t.Fatal("empty bootstrap")
	}
	text := string(asset.Data)
	for _, want := range []string{
		`<wanix-bind dst="term" src="#term">`,
		`<wanix-bind type="fetch" dst="rc.wasm" src="./rc.wasm">`,
		`<wanix-task id="rc" cmd="rc.wasm" type="gojs" wd="web" term start>`,
		`<wanix-term path="#task/rc/term">`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bootstrap missing %s", want)
		}
	}
	if !strings.Contains(NativeBridgeScript, `alert/show`) {
		t.Fatal("native bridge missing alert/show")
	}
}

func TestAssetsOpenRCOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc.wasm")
	if err := os.WriteFile(path, []byte("rc"), 0644); err != nil {
		t.Fatal(err)
	}
	asset, err := (Assets{RCPath: path}).Open("rc.wasm")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(asset.Data); got != "rc" {
		t.Fatalf("rc asset = %q, want rc", got)
	}
	if asset.MIME != "application/wasm" {
		t.Fatalf("rc MIME = %q", asset.MIME)
	}
}

func TestAssetsOpenRejectsEscapes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inside.txt"), []byte("inside"), 0644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := (Assets{Dir: dir}).Open("../outside.txt"); err == nil {
		t.Fatal("Open accepted parent-relative path")
	}
	if _, err := (Assets{Dir: dir}).Open("/../outside.txt"); err == nil {
		t.Fatal("Open accepted absolute parent-relative path")
	}

	asset, err := (Assets{Dir: dir}).Open("inside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(asset.Data); got != "inside" {
		t.Fatalf("inside asset = %q, want inside", got)
	}
}
