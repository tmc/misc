package generate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

func hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func canonical(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		if _, err := os.Lstat(p); err == nil {
			p, err = filepath.EvalSymlinks(p)
			if err != nil {
				return "", err
			}
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fmt.Errorf("cannot resolve path")
		}
		suffix = append(suffix, filepath.Base(p))
		p = parent
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		p = filepath.Join(p, suffix[i])
	}
	return p, nil
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Write generates or builds an API and publishes its manifest last.
func Write(ctx context.Context, a *schema.API, goOut, out, tags string, build bool) (string, error) {
	goOut, err := canonical(goOut)
	if err != nil {
		return "", err
	}
	out, err = canonical(out)
	if err != nil {
		return "", err
	}
	moduleDir, modulePath, err := consumerModule(goOut)
	if err != nil {
		return "", err
	}
	inputDir, err := canonical(a.Dir)
	if err != nil {
		return "", err
	}
	if !within(moduleDir, goOut) || goOut == inputDir || goOut == moduleDir {
		return "", fmt.Errorf("go-out must be inside the consumer module and outside the input package and module root")
	}
	if goOut == out {
		return "", fmt.Errorf("go-out and out must be distinct directories")
	}
	rel, _ := filepath.Rel(moduleDir, goOut)
	outImport := modulePath + "/" + filepath.ToSlash(rel)
	for path := range a.Imports {
		for i, part := range strings.Split(path, "/") {
			if part == "internal" {
				prefix := strings.Join(strings.Split(path, "/")[:i], "/")
				if outImport != prefix && !strings.HasPrefix(outImport, prefix+"/") {
					return "", fmt.Errorf("go-out cannot import internal package %s", path)
				}
			}
		}
	}
	a.Compiler = ""
	a.Tags = ""
	a.Artifacts = nil
	a.BuildIdentity = ""
	files, err := Files(a)
	if err != nil {
		return "", err
	}
	// Validate ownership before invoking a compiler or creating output files.
	old, err := owned(goOut, out)
	if err != nil {
		return "", err
	}
	for name := range files {
		path := filepath.Join(out, name)
		if name == "main.go" {
			path = filepath.Join(goOut, name)
		}
		if err := canReplace(path, name, old); err != nil {
			return "", err
		}
	}
	if build {
		for _, name := range []string{"module.wasm", "wasm_exec.js", "runtime.js"} {
			if err := canReplace(filepath.Join(out, name), name, old); err != nil {
				return "", err
			}
		}
		if err := os.MkdirAll(filepath.Dir(goOut), 0755); err != nil {
			return "", err
		}
		stage, err := os.MkdirTemp(filepath.Dir(goOut), ".wasmbind-build-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(stage)
		if err := os.WriteFile(filepath.Join(stage, "main.go"), files["main.go"], 0644); err != nil {
			return "", err
		}
		if err := os.MkdirAll(goOut, 0755); err != nil {
			return "", err
		}
		overlayPath := filepath.Join(stage, "overlay.json")
		overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(goOut, "main.go"): filepath.Join(stage, "main.go")}})
		if err := os.WriteFile(overlayPath, overlay, 0644); err != nil {
			return "", err
		}
		wasm := filepath.Join(stage, "module.wasm")
		identity, err := buildIdentity(ctx, a, files["main.go"], tags, goOut, moduleDir)
		if err != nil {
			return "", err
		}
		a.BuildIdentity = identity
		args := []string{"build", "-mod=readonly", "-trimpath", "-ldflags=-X main.artifactIdentity=" + identity, "-overlay=" + overlayPath, "-o", wasm}
		if tags != "" {
			args = append(args, "-tags="+tags)
		}
		args = append(args, goOut)
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = moduleDir
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		if data, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("build generated consumer (ensure its dependencies are cached and recorded): %w\n%s", err, data)
		}
		files["module.wasm"], err = os.ReadFile(wasm)
		if err != nil {
			return "", err
		}
		cmd = exec.CommandContext(ctx, "go", "env", "-json", "GOROOT", "GOVERSION")
		cmd.Dir = moduleDir
		data, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("identify compiler: %w", err)
		}
		var env struct{ GOROOT, GOVERSION string }
		if err := json.Unmarshal(data, &env); err != nil {
			return "", err
		}
		runtime, err := os.ReadFile(filepath.Join(env.GOROOT, "lib", "wasm", "wasm_exec.js"))
		if err != nil {
			return "", fmt.Errorf("read selected compiler runtime: %w", err)
		}
		a.Compiler = env.GOVERSION
		a.Tags = tags
		files["wasm_exec.js"] = runtime
		files["bindings.js"] = []byte(strings.Replace(string(files["bindings.js"]), `const artifactIdentity = "";`, `const artifactIdentity = "`+identity+`";`, 1))
		files["runtime.js"] = []byte(runtimeAdapter(env.GOVERSION, hash(runtime)))
	}
	a.Artifacts = map[string]string{}
	for name, data := range files {
		if name != "bindings.json" {
			a.Artifacts[name] = hash(data)
		}
	}
	files["bindings.json"], err = json.MarshalIndent(a.Manifest, "", "  ")
	if err != nil {
		return "", err
	}
	files["bindings.json"] = append(files["bindings.json"], '\n')
	if err := publish(goOut, out, files, old); err != nil {
		return "", err
	}
	return filepath.Join(out, "bindings.json"), nil
}
func runtimeAdapter(version, digest string) string {
	return fmt.Sprintf(`// Code generated by wasmbind. DO NOT EDIT.
const identity = %q;
export async function getGo() {
  const key = Symbol.for('wasmbind.runtime.v1');
  if (globalThis[key]) {
    if (globalThis[key].identity !== identity) throw Object.assign(new Error('conflicting Go runtime in this realm'), {name: 'WasmBindError', code: 'abi-mismatch'});
    return globalThis[key].promise;
  }
  const entry = { identity };
  globalThis[key] = entry;
  entry.promise = (async () => {
    const previous = Object.getOwnPropertyDescriptor(globalThis, 'Go');
    try {
      await import('./wasm_exec.js');
      const BaseGo=globalThis.Go;
      return class extends BaseGo {
        constructor(){super();const exit=this.exit;this.exit=code=>{
          // The matching Go 1.27 runtime leaves scheduled wakeups on abrupt exit.
          for(const timer of this._scheduledTimeouts?.values()??[])clearTimeout(timer);
          this._scheduledTimeouts?.clear();exit(code);
        };}
      };
    }
    finally { if (previous) Object.defineProperty(globalThis, 'Go', previous); else delete globalThis.Go; }
  })();
  try { return await entry.promise; }
  catch (error) { if (globalThis[key] === entry) delete globalThis[key]; throw error; }
}
`, version+":"+digest+":wasmbind-2")
}
func owned(goOut, out string) (*schema.Manifest, error) {
	p := filepath.Join(out, "bindings.json")
	info, err := os.Lstat(p)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing nonregular manifest %s", p)
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m schema.Manifest
	if json.Unmarshal(data, &m) != nil || m.Generated != schema.Marker || m.SchemaVersion != 1 || m.Fingerprint == "" || len(m.Artifacts) == 0 {
		return nil, fmt.Errorf("refusing to replace unrecognized manifest %s", p)
	}
	for name, digest := range m.Artifacts {
		if filepath.Base(name) != name || !allowed(name) {
			return nil, fmt.Errorf("manifest contains unknown artifact %q", name)
		}
		path := filepath.Join(out, name)
		if name == "main.go" {
			path = filepath.Join(goOut, name)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("verify prior artifact %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("refusing nonregular output %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if hash(data) != digest {
			return nil, fmt.Errorf("refusing to replace modified artifact %s", path)
		}
		if name != "module.wasm" && name != "wasm_exec.js" && !bytesMarker(data) {
			return nil, fmt.Errorf("missing generated marker in %s", path)
		}
	}
	return &m, nil
}
func bytesMarker(data []byte) bool { return strings.HasPrefix(string(data), "// "+schema.Marker) }
func allowed(name string) bool {
	switch name {
	case "main.go", "bindings.js", "bindings.d.ts", "runtime.js", "wasm_exec.js", "module.wasm":
		return true
	}
	return false
}
func canReplace(path, name string, old *schema.Manifest) error {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if name == "bindings.json" && old != nil {
		return nil
	}
	if old != nil && old.Artifacts[name] != "" {
		return nil
	}
	return fmt.Errorf("refusing to overwrite user file %s", path)
}
func publish(goOut, out string, files map[string][]byte, old *schema.Manifest) error {
	for _, dir := range []string{goOut, out} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	staged := map[string]string{}
	defer func() {
		for _, p := range staged {
			os.Remove(p)
		}
	}()
	var names []string
	for name, data := range files {
		dir := out
		if name == "main.go" {
			dir = goOut
		}
		f, err := os.CreateTemp(dir, ".wasmbind-")
		if err != nil {
			return err
		}
		staged[name] = f.Name()
		if _, err := f.Write(data); err != nil {
			f.Close()
			return err
		}
		if err := f.Chmod(0644); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if name != "bindings.json" {
			names = append(names, name)
		}
	}
	// From this point a failed replacement has no completion marker.
	if err := os.Remove(filepath.Join(out, "bindings.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		dir := out
		if name == "main.go" {
			dir = goOut
		}
		if err := os.Rename(staged[name], filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	if old != nil {
		for name := range old.Artifacts {
			if files[name] == nil {
				dir := out
				if name == "main.go" {
					dir = goOut
				}
				if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	return os.Rename(staged["bindings.json"], filepath.Join(out, "bindings.json"))
}

func buildIdentity(ctx context.Context, a *schema.API, source []byte, tags, goOut, moduleDir string) (string, error) {
	h := sha256.New()
	h.Write(source)
	fmt.Fprintln(h, tags)
	cmd := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	cmd.Dir = moduleDir
	version, err := cmd.Output()
	if err != nil {
		return "", err
	}
	h.Write(version)
	companions, err := os.ReadDir(goOut)
	if err != nil {
		return "", err
	}
	for _, entry := range companions {
		if entry.IsDir() || entry.Name() == "main.go" || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(goOut, entry.Name()))
		if err != nil {
			return "", err
		}
		fmt.Fprintln(h, entry.Name(), hash(content))
	}
	var paths []string
	data := map[string][]byte{}
	var readErr error
	packages.Visit([]*packages.Package{a.Pkg}, nil, func(p *packages.Package) {
		files := append(append(append([]string{}, p.CompiledGoFiles...), p.OtherFiles...), p.EmbedFiles...)
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				readErr = err
				return
			}
			key := p.PkgPath + "/" + filepath.Base(file)
			if _, ok := data[key]; !ok {
				paths = append(paths, key)
			}
			data[key] = content
		}
	})
	if readErr != nil {
		return "", fmt.Errorf("hash build inputs: %w", readErr)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Fprintln(h, path, hash(data[path]))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func consumerModule(path string) (string, string, error) {
	for dir := path; ; dir = filepath.Dir(dir) {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			m, err := modfile.Parse("go.mod", data, nil)
			if err != nil {
				return "", "", err
			}
			if m.Module == nil {
				return "", "", fmt.Errorf("consumer go.mod has no module directive")
			}
			return dir, m.Module.Mod.Path, nil
		}
		if !os.IsNotExist(err) {
			return "", "", err
		}
		if filepath.Dir(dir) == dir {
			return "", "", fmt.Errorf("go-out must reside in a consumer module")
		}
	}
}
