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
func functionToJSON(fn *ssa.Function, fset *token.FileSet) jsonFunc {
	pos := fset.Position(fn.Pos())
	return jsonFunc{
		Name: fn.Name(),
		Pos: jsonPosition{
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
		Pos: jsonPosition{
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

// ifaceMethodToJSON converts an interface method to JSON format
func ifaceMethodToJSON(method *types.Func, iface *types.Interface, typeName *types.TypeName, fset *token.FileSet) jsonIfaceMethod {
	pos := fset.Position(method.Pos())
	ifaceName := "<unknown>"
	if typeName != nil {
		ifaceName = typeName.Name()
	}
	return jsonIfaceMethod{
		Interface: ifaceName,
		Method: method.Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// constantToJSON converts a constant to JSON format
func constantToJSON(constant *types.Const, fset *token.FileSet) jsonConstant {
	pos := fset.Position(constant.Pos())
	return jsonConstant{
		Name: constant.Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// variableToJSON converts a variable to JSON format
func variableToJSON(variable *types.Var, fset *token.FileSet) jsonVariable {
	pos := fset.Position(variable.Pos())
	return jsonVariable{
		Name: variable.Name(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// typeAliasToJSON converts a type alias to JSON format
func typeAliasToJSON(alias *types.TypeName, fset *token.FileSet) jsonTypeAlias {
	pos := fset.Position(alias.Pos())
	return jsonTypeAlias{
		Name: alias.Name(),
		Original: alias.Type().String(),
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}

// exportedUnusedToJSON converts an unused exported symbol to JSON format
func exportedUnusedToJSON(obj types.Object, kind string, fset *token.FileSet) jsonExportedUnused {
	pos := fset.Position(obj.Pos())
	return jsonExportedUnused{
		Name: obj.Name(),
		Kind: kind,
		Position: jsonPosition{
			File: pos.Filename,
			Line: pos.Line,
			Col:  pos.Column,
		},
	}
}