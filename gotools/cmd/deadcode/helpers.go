// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// functionToJSON converts an SSA function to JSON format
func functionToJSON(fn *ssa.Function, fset *token.FileSet) jsonFunction {
	pos := fset.Position(fn.Pos())
	return jsonFunction{
		Name: fn.Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// typeToJSON converts a named type to JSON format
func typeToJSON(typ *types.Named, fset *token.FileSet) jsonType {
	pos := fset.Position(typ.Obj().Pos())
	return jsonType{
		Name: typ.Obj().Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// interfaceToJSON converts an interface type to JSON format
func interfaceToJSON(iface *types.Interface, fset *token.FileSet) jsonInterface {
	// Interface types don't have an Obj() method, so we need to get position differently
	pos := token.Position{} // Default position if we can't determine it
	return jsonInterface{
		Name: "<interface>", // We might need to pass the name separately
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// fieldToJSON converts a struct field to JSON format
func fieldToJSON(field *types.Var, fset *token.FileSet) jsonField {
	pos := fset.Position(field.Pos())
	return jsonField{
		Type:  field.Type().String(),
		Field: field.Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}
