package main

import (
	"go/ast"
	"go/token"
)

// transformMocks handles transformation of testify mock usage
func transformMocks(file *ast.File, fset *token.FileSet) {
	// Add TODO comments for mock usage
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			// Check for mock types
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							if embedsMock(structType) {
								// Add TODO comment
								addTODOComment(node, "Consider replacing "+typeSpec.Name.Name+" with an interface-based test double")
							}
						}
					}
				}
			}
		case *ast.CallExpr:
			// Check for mock method calls
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if isMockMethod(sel.Sel.Name) {
					// Find the statement containing this call
					addInlineTODO(file, fset, node, "Implement test double behavior without testify/mock")
				}
			}
		}
		return true
	})
}

func embedsMock(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if field.Names == nil { // embedded field
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "mock" {
					if sel.Sel.Name == "Mock" {
						return true
					}
				}
			}
		}
	}
	return false
}

func isMockMethod(name string) bool {
	mockMethods := []string{
		"On", "Return", "Run", "Once", "Twice", "Times",
		"Called", "AssertExpectations", "AssertNotCalled",
		"AssertCalled", "AssertNumberOfCalls",
	}
	for _, method := range mockMethods {
		if name == method {
			return true
		}
	}
	return false
}

func addTODOComment(node ast.Node, message string) {
	if decl, ok := node.(*ast.GenDecl); ok {
		comment := &ast.Comment{
			Text: "// TODO: " + message,
		}
		if decl.Doc == nil {
			decl.Doc = &ast.CommentGroup{}
		}
		decl.Doc.List = append([]*ast.Comment{comment}, decl.Doc.List...)
	}
}

func addInlineTODO(file *ast.File, fset *token.FileSet, node ast.Node, message string) {
	// This is a simplified approach - in practice, we'd need to properly
	// track positions and insert comments at the right locations
	// For now, we'll add TODO in the transformation phase
}

// transformMockCalls transforms mock.Mock embedded field references
func transformMockCalls(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok {
			// Keep mock.Mock fields but add TODO comment
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "mock" {
					if sel.Sel.Name == "Mock" {
						if field.Comment == nil {
							field.Comment = &ast.CommentGroup{}
						}
						field.Comment.List = append(field.Comment.List, &ast.Comment{
							Text: "// TODO: Remove mock.Mock dependency",
						})
					}
				}
			}
		}
		return true
	})
}