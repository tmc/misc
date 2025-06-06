// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gomvpkg command moves go packages, updating import declarations.
// See the -help message or Usage constant for details.
package main

import (
	"context"
	"flag"
	"fmt"
	"go/build"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/packages"
)

var (
	fromFlag     = flag.String("from", "", "Import path of package to be moved")
	toFlag       = flag.String("to", "", "Destination import path for package")
	vcsMvCmdFlag = flag.String("vcs_mv_cmd", "", `A template for the version control system's "move directory" command, e.g. "git mv {{.Src}} {{.Dst}}"`)
	helpFlag     = flag.Bool("help", false, "show usage message")
)

func init() {
	flag.Var((*buildutil.TagsFlag)(&build.Default.BuildTags), "tags", buildutil.TagsFlagDoc)
}

const Usage = `gomvpkg: moves a package, updating import declarations

Usage:

 gomvpkg -from <path> -to <path> [-vcs_mv_cmd <template>]

Flags:

-from        specifies the import path of the package to be moved

-to          specifies the destination import path

-vcs_mv_cmd  specifies a shell command to inform the version control system of a
             directory move.  The argument is a template using the syntax of the
             text/template package. It has two fields: Src and Dst, the absolute
             paths of the directories.

             For example: "git mv {{.Src}} {{.Dst}}"

gomvpkg determines the set of packages that might be affected, including all
packages importing the 'from' package and any of its subpackages. It will move
the 'from' package and all its subpackages to the destination path and update all
imports of those packages to point to its new import path.

gomvpkg rejects moves in which a package already exists at the destination import
path, or in which a directory already exists at the location the package would be
moved to.

gomvpkg will not always be able to rename imports when a package's name is changed.
Import statements may want further cleanup.

gomvpkg's behavior is not defined if any of the packages to be moved are
imported using dot imports.

Examples:

% gomvpkg -from myproject/foo -to myproject/bar

  Move the package with import path "myproject/foo" to the new path
  "myproject/bar".

% gomvpkg -from myproject/foo -to myproject/bar -vcs_mv_cmd "git mv {{.Src}} {{.Dst}}"

  Move the package with import path "myproject/foo" to the new path
  "myproject/bar" using "git mv" to execute the directory move.
`

// movePackage implements package moving functionality using modern Go tools APIs
func movePackage(ctxt *build.Context, from, to, moveTmpl string) error {
	// Find the source package's location first
	fromPkg, err := ctxt.Import(from, "", build.FindOnly)
	if err != nil {
		return fmt.Errorf("cannot find package %q: %v", from, err)
	}

	// Determine the source root (module or GOPATH)
	srcRoot := ""
	modRoot := findModuleRoot()
	if modRoot != "" {
		// We're in a module
		srcRoot = modRoot
		// Verify source package is in the same module
		if !strings.HasPrefix(fromPkg.Dir, modRoot) {
			return fmt.Errorf("source package %q is not in the current module", from)
		}
	} else {
		// GOPATH mode - find which GOPATH entry contains the source
		for _, gopath := range filepath.SplitList(ctxt.GOPATH) {
			if strings.HasPrefix(fromPkg.Dir, filepath.Join(gopath, "src")) {
				srcRoot = filepath.Join(gopath, "src")
				break
			}
		}
		if srcRoot == "" {
			return fmt.Errorf("source package %q not found in GOPATH", from)
		}
	}

	// Validate and resolve destination path
	var destImportPath string
	var destDir string

	if modRoot != "" {
		// Module mode
		modPath := findModulePath(modRoot)
		if modPath == "" {
			return fmt.Errorf("cannot determine module path")
		}

		// Convert destination to absolute import path
		if strings.HasPrefix(to, "./") || strings.HasPrefix(to, "../") {
			// Relative path - resolve relative to source package
			relPath := strings.TrimPrefix(fromPkg.Dir, modRoot)
			srcPkgPath := modPath + relPath
			destImportPath = filepath.Join(filepath.Dir(srcPkgPath), to)
			destImportPath = filepath.ToSlash(destImportPath)
		} else if strings.HasPrefix(to, modPath) {
			// Already a full import path in the module
			destImportPath = to
		} else {
			return fmt.Errorf("destination %q must be within module %q", to, modPath)
		}

		// Calculate physical directory
		destDir = filepath.Join(modRoot, strings.TrimPrefix(destImportPath, modPath))
	} else {
		// GOPATH mode
		if strings.HasPrefix(to, "./") || strings.HasPrefix(to, "../") {
			// Relative path - resolve relative to source package
			srcImportPath := strings.TrimPrefix(fromPkg.Dir, srcRoot+string(filepath.Separator))
			destImportPath = filepath.Join(filepath.Dir(srcImportPath), to)
			destImportPath = filepath.ToSlash(destImportPath)
		} else {
			// Absolute import path
			destImportPath = to
		}

		destDir = filepath.Join(srcRoot, filepath.FromSlash(destImportPath))
	}

	// Validate the destination package name
	baseName := filepath.Base(destImportPath)
	if !token.IsIdentifier(baseName) {
		return fmt.Errorf("invalid package name %q", baseName)
	}

	// Check if destination already exists
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination %q already exists", destDir)
	}

	// Now load packages and update imports as before
	// Load packages to find all that import the package being moved
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax,
		Context:    context.Background(),
		BuildFlags: ctxt.BuildTags,
	}

	// Load all packages in the current module/workspace
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("loading packages: %v", err)
	}

	// Find source package
	var srcPkg *packages.Package
	for _, pkg := range pkgs {
		if pkg.PkgPath == from {
			srcPkg = pkg
			break
		}
	}
	if srcPkg == nil {
		return fmt.Errorf("package %q not found", from)
	}

	// Check if destination package already exists
	for _, pkg := range pkgs {
		if pkg.PkgPath == destImportPath {
			return fmt.Errorf("package %q already exists", destImportPath)
		}
	}

	// Update import statements in all packages that import the moved package
	for _, pkg := range pkgs {
		if pkg.PkgPath == from {
			continue // Skip the package being moved itself
		}

		// Check if this package imports the one being moved
		importsMovedPkg := false
		for imp := range pkg.Imports {
			if imp == from || strings.HasPrefix(imp, from+"/") {
				importsMovedPkg = true
				break
			}
		}

		if !importsMovedPkg {
			continue
		}

		// Update import statements in this package's files
		for _, file := range pkg.Syntax {
			modified := false
			for _, imp := range file.Imports {
				if imp.Path != nil {
					path, err := strconv.Unquote(imp.Path.Value)
					if err != nil {
						continue
					}
					if path == from {
						imp.Path.Value = strconv.Quote(destImportPath)
						modified = true
					} else if strings.HasPrefix(path, from+"/") {
						newPath := destImportPath + strings.TrimPrefix(path, from)
						imp.Path.Value = strconv.Quote(newPath)
						modified = true
					}
				}
			}

			if modified {
				// Write the updated file
				fset := token.NewFileSet()
				filename := pkg.Fset.Position(file.Pos()).Filename

				f, err := os.Create(filename)
				if err != nil {
					return fmt.Errorf("opening %s: %v", filename, err)
				}
				defer f.Close()

				if err := format.Node(f, fset, file); err != nil {
					return fmt.Errorf("formatting %s: %v", filename, err)
				}
			}
		}
	}

	// Update package declaration in moved package files
	for _, file := range srcPkg.Syntax {
		// Update package name if it changes
		if file.Name != nil {
			newPkgName := filepath.Base(destImportPath)
			if newPkgName != file.Name.Name {
				file.Name.Name = newPkgName

				// Write the updated file
				fset := token.NewFileSet()
				filename := srcPkg.Fset.Position(file.Pos()).Filename

				f, err := os.Create(filename)
				if err != nil {
					return fmt.Errorf("opening %s: %v", filename, err)
				}
				defer f.Close()

				if err := format.Node(f, fset, file); err != nil {
					return fmt.Errorf("formatting %s: %v", filename, err)
				}
			}
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return fmt.Errorf("creating destination parent directory: %v", err)
	}

	// Move the directory
	if moveTmpl != "" {
		// Execute VCS move command
		tmpl, err := template.New("vcs").Parse(moveTmpl)
		if err != nil {
			return fmt.Errorf("parsing VCS template: %v", err)
		}

		var cmdBuf strings.Builder
		if err := tmpl.Execute(&cmdBuf, struct{ Src, Dst string }{fromPkg.Dir, destDir}); err != nil {
			return fmt.Errorf("executing VCS template: %v", err)
		}

		cmdStr := cmdBuf.String()
		fmt.Printf("Executing VCS command: %s\n", cmdStr)

		// Parse and execute the VCS command
		cmdParts := strings.Fields(cmdStr)
		if len(cmdParts) == 0 {
			return fmt.Errorf("empty VCS command")
		}

		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("VCS command failed: %v", err)
		}
	} else {
		// Use os.Rename for simple directory move
		if err := os.Rename(fromPkg.Dir, destDir); err != nil {
			return fmt.Errorf("moving directory from %s to %s: %v", fromPkg.Dir, destDir, err)
		}
		fmt.Printf("Moved %s to %s\n", fromPkg.Dir, destDir)
	}

	return nil
}

// findModuleRoot finds the root directory of the current Go module
func findModuleRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return ""
}

// findModulePath extracts the module path from go.mod
func findModulePath(modRoot string) string {
	goModPath := filepath.Join(modRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return ""
}

func main() {
	flag.Parse()

	if len(flag.Args()) > 0 {
		fmt.Fprintln(os.Stderr, "gomvpkg: surplus arguments.")
		os.Exit(1)
	}

	if *helpFlag || *fromFlag == "" || *toFlag == "" {
		fmt.Print(Usage)
		return
	}

	if err := movePackage(&build.Default, *fromFlag, *toFlag, *vcsMvCmdFlag); err != nil {
		fmt.Fprintf(os.Stderr, "gomvpkg: %s.\n", err)
		os.Exit(1)
	}
}
