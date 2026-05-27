package webkithost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/wanix-macapp/internal/applefs"
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
		`<wanix-bind dst="macos" src="#macos">`,
		`<wanix-bind dst="mnt/macos" src="#macos">`,
		`<wanix-bind type="fetch" dst="rc.wasm" src="./rc.wasm">`,
		`<wanix-task id="rc" cmd="rc.wasm" type="gojs" wd="web" term start>`,
		`<wanix-term path="#task/rc/term">`,
		`<script type="module" src="./wanix.min.js"></script>`,
		`<script>`,
		`customElements.whenDefined("wanix-system")`,
		`waitForRuntimeHooks`,
		`ensureNamespace`,
		`wanix bootstrap timeout`,
		`wanix-system ready event timeout`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bootstrap missing %s", want)
		}
	}
	if !strings.Contains(NativeBridgeScript, `native.fs`) {
		t.Fatal("native bridge missing native fs transport")
	}
}

func TestNativeBridgeDesktopSurface(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"native fs read", `"readFile"`},
		{"native fs write", `"writeFile"`},
		{"native fs readdir", `"readDir"`},
		{"native fs cache", `__wanixHydrateMacOSFS`},
		{"native fs reply", `__wanixNativeFSReply`},
		{"native fs refresh after write", `refreshAfterWrite`},
		{"clone schema injection", `__wanixCloneSchemas`},
		{"initial fs injection", `__wanixInitialMacOSFS`},
		{"global clone ids", `globalCloneNext`},
		{"cache update hook", `_updateCache`},
		{"cache append hook", `_appendCache`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(NativeBridgeScript, tt.want) {
				t.Fatalf("native bridge missing %s", tt.want)
			}
		})
	}
	for _, old := range []string{`legacy =`, `legacy(name)`, `fps `, `duration `, `format `} {
		if strings.Contains(NativeBridgeScript, old) {
			t.Fatalf("native bridge still has Apple-specific routing %s", old)
		}
	}
	for _, old := range []string{`"app/name"`, `"dialog/open/ctl"`, `"dialog/open/result"`, `"dialog.open"`, `"window/0/title"`, `"window/0/ctl"`, `"alert/show"`, `"alert.show"`} {
		if strings.Contains(NativeBridgeScript, old) {
			t.Fatalf("native bridge still exposes %s", old)
		}
	}
}

func TestNativeBridgeScriptInjectsCloneSchemas(t *testing.T) {
	script := nativeBridgeScript(applefs.NewRoot())
	if strings.Contains(script, "__WANIX_CLONE_SCHEMAS__") {
		t.Fatal("native bridge script still contains clone schema placeholder")
	}
	if strings.Contains(script, "__WANIX_INITIAL_FS__") {
		t.Fatal("native bridge script still contains initial fs placeholder")
	}
	for _, want := range []string{`"appkit/alert"`, `"alert"`, `"vision"`, `"ax/app"`} {
		if !strings.Contains(script, want) {
			t.Fatalf("native bridge script missing schema %s", want)
		}
	}
	for _, want := range []string{`"appkit"`, `"notify"`, `"touchid"`, `"status"`} {
		if !strings.Contains(script, want) {
			t.Fatalf("native bridge script missing initial fs %s", want)
		}
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

func TestAssetServerSummary(t *testing.T) {
	var s assetServer
	if got := s.Summary(); got != "no asset requests" {
		t.Fatalf("empty summary = %q", got)
	}
	s.record("/")
	s.record("/wanix.js")
	s.record("/wanix.js")
	if got := s.Summary(); got != "/:1, /wanix.js:2" {
		t.Fatalf("summary = %q", got)
	}
}
