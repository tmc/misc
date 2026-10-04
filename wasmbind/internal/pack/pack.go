// Package pack creates local ES module packages from completed browser bundles.
package pack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
	"golang.org/x/mod/semver"
)

// Options specifies the explicit package identity and license.
type Options struct{ Bundle, Out, Name, Version, LicenseFile, ReadmeFile string }
type receipt struct {
	Generated string            `json:"generated"`
	Files     map[string]string `json:"files"`
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
var namePattern = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

// Write validates a completed bundle and publishes a local package directory.
func Write(ctx context.Context, opt Options) (string, error) {
	if !namePattern.MatchString(opt.Name) || opt.Name == "node_modules" || opt.Name == "favicon.ico" || !versionPattern.MatchString(opt.Version) || len(opt.Name) > 214 || !semver.IsValid("v"+opt.Version) || opt.LicenseFile == "" {
		return "", fmt.Errorf("pack requires a valid npm name, semver version, and explicit license-file")
	}
	if opt.Out == "" || opt.Bundle == "" {
		return "", fmt.Errorf("pack requires bundle and out")
	}
	bundle, err := canonicalPath(opt.Bundle)
	if err != nil {
		return "", err
	}
	out, err := canonicalPath(opt.Out)
	if err != nil {
		return "", err
	}
	if overlaps(bundle, out) {
		return "", fmt.Errorf("package out must not overlap bundle")
	}
	data, err := regular(filepath.Join(bundle, "bindings.json"))
	if err != nil {
		return "", err
	}
	var manifest schema.Manifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Generated != schema.Marker || manifest.Compiler == "" || manifest.BuildIdentity == "" || manifest.SchemaVersion != 1 {
		return "", fmt.Errorf("pack requires a completed build manifest")
	}
	allow := map[string]bool{"bindings.js": true, "bindings.d.ts": true, "runtime.js": true, "wasm_exec.js": true, "module.wasm": true, "API.md": true}
	files := map[string][]byte{"bindings.json": data}
	for name, hash := range manifest.Artifacts {
		if name == "main.go" {
			continue
		} // Compiler source is provenance, not a shipped browser asset.
		if !allow[name] {
			return "", fmt.Errorf("unlisted bundle artifact %q", name)
		}
		data, err := regular(filepath.Join(bundle, name))
		if err != nil {
			return "", err
		}
		if digest(data) != hash {
			return "", fmt.Errorf("artifact hash mismatch: %s", name)
		}
		files[name] = data
	}
	for name := range allow {
		if files[name] == nil {
			return "", fmt.Errorf("missing bundle artifact %s", name)
		}
	}
	license, err := regular(opt.LicenseFile)
	if err != nil {
		return "", fmt.Errorf("read license: %w", err)
	}
	if len(license) == 0 || len(license) > 1<<20 {
		return "", fmt.Errorf("license must be nonempty and at most one MiB")
	}
	files["LICENSE"] = license
	files["index.js"] = []byte("// " + schema.Marker + "\nexport * from './bindings.js';\nimport {load as open} from './bindings.js';\nexport function load(source = new URL('./module.wasm', import.meta.url), options) {\n  return open(source, options);\n}\n")
	optional := "?"
	if hasRequiredImports(manifest) {
		optional = ""
	}
	files["copy-assets.js"] = []byte("// " + schema.Marker + `
import {copyFile, mkdir} from 'node:fs/promises';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
/** Copy browser assets to a directory path or file URL, replacing existing assets. */
export async function copyAssets(destination) {
  const directory = destination instanceof URL ? fileURLToPath(destination) : destination;
  await mkdir(directory, {recursive: true});
  for (const name of ['module.wasm', 'runtime.js', 'wasm_exec.js']) {
    await copyFile(new URL(name, import.meta.url), join(directory, name));
  }
}
`)
	files["copy-assets.d.ts"] = []byte("// " + schema.Marker + "\n/** Copy browser assets to a directory path or file URL, replacing existing assets. Node only. */\nexport function copyAssets(destination: string | URL): Promise<void>;\n")
	files["index.d.ts"] = []byte("// " + schema.Marker + "\nexport * from './bindings.js';\nimport type {Module, LoadOptions} from './bindings.js';\nexport function load(source" + optional + ": URL | Response | BufferSource | WebAssembly.Module | undefined, options" + optional + ": LoadOptions): Promise<Module>;\n")
	loadExample := "await load()"
	if hasRequiredImports(manifest) {
		loadExample = "await load(undefined, {imports})"
	}
	files["README.md"] = []byte(fmt.Sprintf("<!-- %s -->\n# %s\n\nBrowser Window and dedicated Worker package.\n\n```js\nimport {load} from '%s';\nconst module = %s;\ntry { /* call the API */ } finally { await module.dispose(); }\n```\n\nPass an explicit URL, Response, BufferSource, or compiled Module to load when a\nbundler handles assets separately. Use the Node-only copyAssets helper after bundling; see [BUNDLING.md](BUNDLING.md).\nSee [API.md](API.md) for the generated API reference.\nCancellation is cooperative; use a dedicated Worker for hard termination.\nNode execution and browser-to-native direct UDP are not qualified by packaging.\nThe supplied LICENSE is copied verbatim; dependency licensing remains the\npackage author's responsibility. See bindings.json for compiler and schema\nprovenance. This directory has not been published.\n", schema.Marker, opt.Name, opt.Name, loadExample))
	if opt.ReadmeFile != "" {
		data, err := regular(opt.ReadmeFile)
		if err != nil {
			return "", fmt.Errorf("read README: %w", err)
		}
		if len(data) > 1<<20 {
			return "", fmt.Errorf("README exceeds one MiB")
		}
		files["README.md"] = append([]byte("<!-- "+schema.Marker+" -->\n"), data...)
		files["README.md"] = append(files["README.md"], []byte("\n\n[API reference](API.md) | [Bundling](BUNDLING.md)\n")...)
	}
	provenance, _ := json.MarshalIndent(map[string]any{"generated": schema.Marker, "generator": manifest.GeneratorVersion, "compiler": manifest.Compiler, "fingerprint": manifest.Fingerprint, "buildIdentity": manifest.BuildIdentity, "typescriptMinTested": manifest.TypeScriptMinTested, "features": manifest.Features, "backend": manifest.Backend, "inputArtifactHashes": manifest.Artifacts}, "", "  ")
	files["provenance.json"] = append(provenance, '\n')
	files["BUNDLING.md"] = []byte(fmt.Sprintf(`<!-- %s -->
# Bundle browser consumers

The package entry point loads module.wasm beside the executing ES module.
For an esbuild bundle, copy the browser assets beside the output file:

~~~js
// build.mjs (Node)
import {build} from 'esbuild';
import {copyAssets} from '%s/copy-assets';

await build({entryPoints: ['app.js'], outfile: 'public/app.js',
  bundle: true, format: 'esm', platform: 'browser', target: 'es2022'});
await copyAssets('public');
~~~

Then call the package's load() in app.js. Packages with required imports use
load(undefined, {imports}). Serve .wasm as application/wasm. Dedicated Worker
bundles use the same layout. copyAssets replaces module.wasm, runtime.js, and
wasm_exec.js in the chosen directory; use a directory owned by this build.
One directory supports one package's default Wasm URL. For multiple packages,
use separate output directories or pass each package an explicit asset URL.

Other bundlers should use their asset loader to obtain a URL or use copyAssets
with an ES module output layout. Only the esbuild layout is covered by the
package browser qualification suite.
`, schema.Marker, opt.Name))
	var names []string
	for name := range files {
		names = append(names, name)
	}
	names = append(names, "package.json")
	sort.Strings(names)
	pkg, _ := json.MarshalIndent(map[string]any{"name": opt.Name, "version": opt.Version, "type": "module", "license": "SEE LICENSE IN LICENSE", "files": names, "exports": map[string]any{".": map[string]string{"types": "./index.d.ts", "import": "./index.js"}, "./module.wasm": "./module.wasm", "./bindings.json": "./bindings.json", "./API.md": "./API.md", "./copy-assets": map[string]string{"types": "./copy-assets.d.ts", "node": "./copy-assets.js"}}}, "", "  ")
	files["package.json"] = append(pkg, '\n')
	if err := publish(ctx, out, files); err != nil {
		return "", err
	}
	return filepath.Join(out, "package-manifest.json"), nil
}
func hasRequiredImports(m schema.Manifest) bool {
	for _, d := range m.Declarations {
		if d.Kind == "imports" && !d.Optional {
			return true
		}
	}
	return false
}
func regular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing nonregular file %s", path)
	}
	return os.ReadFile(path)
}
func publish(ctx context.Context, out string, files map[string][]byte) error {
	var old receipt
	prior := filepath.Join(out, "package-manifest.json")
	if data, err := regular(prior); err == nil {
		if json.Unmarshal(data, &old) != nil || old.Generated != schema.Marker {
			return fmt.Errorf("unrecognized package manifest")
		}
		for name, hash := range old.Files {
			if filepath.Base(name) != name {
				return fmt.Errorf("invalid package artifact name")
			}
			data, err := regular(filepath.Join(out, name))
			if err != nil {
				return err
			}
			if digest(data) != hash {
				return fmt.Errorf("modified package artifact %s", name)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for name := range files {
		if _, err := os.Lstat(filepath.Join(out, name)); err == nil && old.Files[name] == "" {
			return fmt.Errorf("refusing user file %s", name)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	staged := map[string]string{}
	defer func() {
		for _, file := range staged {
			os.Remove(file)
		}
	}()
	next := receipt{schema.Marker, map[string]string{}}
	for name, data := range files {
		next.Files[name] = digest(data)
	}
	marker, _ := json.MarshalIndent(next, "", "  ")
	files["package-manifest.json"] = append(marker, '\n')
	var names []string
	for name, data := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.CreateTemp(out, ".wasmbind-pack-")
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
		if name != "package-manifest.json" {
			names = append(names, name)
		}
	}
	if err := os.Remove(prior); err != nil && !os.IsNotExist(err) {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.Rename(staged[name], filepath.Join(out, name)); err != nil {
			return err
		}
	}
	for name := range old.Files {
		if files[name] == nil {
			if err := os.Remove(filepath.Join(out, name)); err != nil {
				return err
			}
		}
	}
	return os.Rename(staged["package-manifest.json"], prior)
}

func overlaps(a, b string) bool {
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
func canonicalPath(path string) (string, error) {
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
			return "", fmt.Errorf("cannot resolve package path")
		}
		suffix = append(suffix, filepath.Base(p))
		p = parent
	}
	for j := len(suffix) - 1; j >= 0; j-- {
		p = filepath.Join(p, suffix[j])
	}
	return p, nil
}
