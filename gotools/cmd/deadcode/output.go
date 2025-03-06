// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

// Map of generated filenames
var generated = make(map[string]bool)

// formatResults outputs the analysis results according to the specified format.
func formatResults(fset *token.FileSet, res *analysisResult, formatStr string, jsonOutput bool) error {
	// Create a filter regex
	filter, err := regexp.Compile(*filterFlag)
	if err != nil {
		return fmt.Errorf("invalid filter pattern %q: %v", *filterFlag, err)
	}

	// Group functions by package path
	byPkgPath := make(map[string]map[*ssa.Function]bool)
	for fn := range res.deadFuncs {
		// Skip functions in generated files
		if *generatedFlag == false {
			if posn := fset.Position(fn.Pos()); posn.IsValid() {
				if generated[posn.Filename] {
					continue
				}
			}
		}

		pkgPath := fn.Pkg.Pkg.Path()
		if byPkgPath[pkgPath] == nil {
			byPkgPath[pkgPath] = make(map[*ssa.Function]bool)
		}
		byPkgPath[pkgPath][fn] = true
	}

	// Build array of jsonPackage objects.
	var packages []jsonPackage
	for _, pkgpath := range slices.Sorted(maps.Keys(byPkgPath)) {
		if !filter.MatchString(pkgpath) {
			continue
		}
		m := byPkgPath[pkgpath]

		// Sort functions within each package by position to approximate
		// declaration order. This tends to keep related
		// methods such as (T).Marshal and (*T).Unmarshal
		// together better than sorting.
		fns := slices.Collect(maps.Keys(m))
		sort.Slice(fns, func(i, j int) bool {
			xposn := fset.Position(fns[i].Pos())
			yposn := fset.Position(fns[j].Pos())
			if xposn.Filename != yposn.Filename {
				return xposn.Filename < yposn.Filename
			}
			return xposn.Line < yposn.Line
		})

		// Each package in report
		funcs := make([]jsonFunc, 0, len(fns))
		for _, fn := range fns {
			pos := fset.Position(fn.Pos())
			name := prettyName(fn, false)
			funcs = append(funcs, jsonFunc{
				Name:     name,
				FullName: prettyName(fn, true),
				Pos:      toJSONPosition(pos),
			})
		}
		packages = append(packages, jsonPackage{
			Name:  packageName(pkgpath),
			Path:  pkgpath,
			Funcs: funcs,
		})
	}

	// Add types if requested
	if *typesFlag || *allFlag {
		printDeadTypes(fset, res, filter)
	}

	// Add interfaces if requested
	if *ifacesFlag || *allFlag {
		printDeadInterfaces(fset, res, filter)
	}

	// Add fields if requested
	if *fieldsFlag || *allFlag {
		printDeadFields(fset, res, filter)
	}

	// Format output
	if jsonOutput {
		printJSON(packages)
	} else if formatStr != "" {
		printTemplate(formatStr, packages)
	} else {
		printObjects("%s %s", packages)
	}

	return nil
}

// prettyName returns a user-readable name for a function.
func prettyName(fn *ssa.Function, qualified bool) string {
	var buf strings.Builder
	format := func(fn *ssa.Function) {
		// Package-qualified?
		if qualified && fn.Pkg != nil {
			buf.WriteString(packageName(fn.Pkg.Pkg.Path()))
			buf.WriteString(".")
		}

		// Anonymous?
		if fn.Parent() != nil {
			format(fn.Parent())
			i := slices.Index(fn.Parent().AnonFuncs, fn)
			fmt.Fprintf(&buf, "$%d", i+1)
			return
		}

		// Method?
		if recv := fn.Signature.Recv(); recv != nil {
			buf.WriteByte('(')
			if qualified {
				types.WriteType(&buf, recv.Type(), types.RelativeTo(fn.Pkg.Pkg))
			} else {
				tname := types.TypeString(recv.Type(), types.RelativeTo(fn.Pkg.Pkg))
				// Remove package qualification from receiver
				tname = strings.TrimPrefix(tname, fn.Pkg.Pkg.Path()+".")
				buf.WriteString(tname)
			}
			buf.WriteByte(')')
			buf.WriteByte('.')
		}

		buf.WriteString(fn.Name())
	}
	format(fn)
	return buf.String()
}

// packageName returns just the name part of a package path
func packageName(pkgpath string) string {
	return filepath.Base(pkgpath)
}

// printJSON outputs the results in JSON format
func printJSON(objects []jsonPackage) {
	b, err := json.MarshalIndent(objects, "", "  ")
	if err != nil {
		log.Fatalf("internal error marshaling JSON: %v", err)
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte{'\n'})
}

// printTemplate outputs the results using a template
func printTemplate(format string, objects []jsonPackage) {
	tmpl, err := template.New("").Parse(format)
	if err != nil {
		log.Fatalf("invalid format template: %v", err)
	}
	for _, obj := range objects {
		if err := tmpl.Execute(os.Stdout, obj); err != nil {
			log.Printf("error executing template: %v", err)
		}
	}
}

// printObjects outputs the results in a basic format
func printObjects(format string, objects []jsonPackage) {
	for _, p := range objects {
		for _, fn := range p.Funcs {
			fmt.Printf(format+"\n", fn.Pos, fn.FullName)
		}
	}
}

// printDeadTypes outputs information about dead types
func printDeadTypes(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead types:\n")
	for t := range res.deadTypes {
		if pkg := t.Obj().Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(t.Obj().Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s\n", pos, packageName(pkgPath), t.Obj().Name())
		}
	}
}

// printDeadInterfaces outputs information about dead interfaces
func printDeadInterfaces(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead interfaces:\n")
	for iface := range res.deadIfaces {
		obj := res.typeInfo[iface]
		if pkg := obj.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(obj.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s\n", pos, packageName(pkgPath), obj.Name())
		}
	}
}

// printDeadFields outputs information about dead struct fields
func printDeadFields(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead fields:\n")
	for field := range res.deadFields {
		if pkg := field.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(field.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s.%s\n", pos, packageName(pkgPath), field.Parent().Name(), field.Name())
		}
	}
}

// pathSearch returns the shortest path from one of the roots to one
// of the targets (along with the root itself), or zero if no path was found.
func pathSearch(roots []*ssa.Function, res *rta.Result, targets map[*ssa.Function]bool) (*callgraph.Node, []*callgraph.Edge) {
	// Sort roots into preferred order.
	importsTesting := func(fn *ssa.Function) bool {
		isTesting := func(p *types.Package) bool { return p.Path() == "testing" }
		return slices.ContainsFunc(fn.Pkg.Pkg.Imports(), isTesting)
	}
	sort.Slice(roots, func(i, j int) bool {
		x, y := roots[i], roots[j]
		if xt, yt := importsTesting(x), importsTesting(y); xt != yt {
			return !xt // non-testing first
		}
		return x.String() < y.String() // break ties deterministically
	})

	cg := callgraph.FromRTA(res)

	// BFS traversal from each root, preferring shorter paths.
	for _, root := range roots {
		visited := make(map[*callgraph.Node]bool)
		seen := make(map[*callgraph.Node]*callgraph.Edge)
		var queue []*callgraph.Node
		rootNode := cg.Nodes[root]
		visited[rootNode] = true
		queue = append(queue, rootNode)
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			if targets[node.Func] {
				// found path to a target
				var path []*callgraph.Edge
				for {
					edge := seen[node]
					if edge == nil {
						slices.Reverse(path)
						return rootNode, path
					}
					path = append(path, edge)
					node = edge.Caller
				}
			}
			for _, edge := range node.Out {
				if succ := edge.Callee; !visited[succ] {
					visited[succ] = true
					seen[succ] = edge
					queue = append(queue, succ)
				}
			}
		}
	}
	return nil, nil
}

// jsonFunc is a JSON-serializable representation of a function
type jsonFunc struct {
	Name     string       `json:"name"`
	FullName string       `json:"fullName"`
	Pos      jsonPosition `json:"pos"`
}

// jsonPackage is a JSON-serializable representation of a package
type jsonPackage struct {
	Name  string     `json:"name"`
	Path  string     `json:"path"`
	Funcs []jsonFunc `json:"funcs"`
	Types []jsonType `json:"types,omitempty"`
}

// jsonType is a JSON-serializable representation of a type
type jsonType struct {
	Name string       `json:"name"`
	Pos  jsonPosition `json:"pos"`
}

// jsonPosition is a JSON-serializable representation of a source position
type jsonPosition struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

func (p jsonPosition) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}