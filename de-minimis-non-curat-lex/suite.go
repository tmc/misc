package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// suiteTransformer handles transformation of testify suite tests
type suiteTransformer struct {
	file *ast.File
	fset *token.FileSet
}

func transformSuites(file *ast.File, fset *token.FileSet) {
	st := &suiteTransformer{file: file, fset: fset}
	st.transform()
}

func (st *suiteTransformer) transform() {
	// Find all suite types
	suites := st.findSuites()
	
	// Transform each suite
	for _, suite := range suites {
		st.transformSuite(suite)
	}
}

type suiteInfo struct {
	typeName    string
	typeDecl    *ast.GenDecl
	fields      []*ast.Field
	setupTest   *ast.FuncDecl
	tearDown    *ast.FuncDecl
	testMethods []*ast.FuncDecl
	runFunc     *ast.FuncDecl
}

func (st *suiteTransformer) findSuites() []*suiteInfo {
	var suites []*suiteInfo
	suiteMap := make(map[string]*suiteInfo)
	
	// First pass: find suite types
	for _, decl := range st.file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						if st.embedsSuite(structType) {
							info := &suiteInfo{
								typeName:    typeSpec.Name.Name,
								typeDecl:    genDecl,
								fields:      st.getNonSuiteFields(structType),
								testMethods: []*ast.FuncDecl{},
							}
							suites = append(suites, info)
							suiteMap[info.typeName] = info
						}
					}
				}
			}
		}
	}
	
	// Second pass: find methods
	for _, decl := range st.file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if len(funcDecl.Recv.List) > 0 {
				recvType := st.getReceiverType(funcDecl.Recv.List[0].Type)
				if info, ok := suiteMap[recvType]; ok {
					switch funcDecl.Name.Name {
					case "SetupTest":
						info.setupTest = funcDecl
					case "TearDownTest":
						info.tearDown = funcDecl
					default:
						if strings.HasPrefix(funcDecl.Name.Name, "Test") {
							info.testMethods = append(info.testMethods, funcDecl)
						}
					}
				}
			}
		}
	}
	
	// Third pass: find Test*Suite functions
	for _, decl := range st.file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv == nil {
			if strings.HasPrefix(funcDecl.Name.Name, "Test") && strings.HasSuffix(funcDecl.Name.Name, "Suite") {
				// Check if it calls suite.Run
				suiteName := st.findSuiteRun(funcDecl)
				if suiteName != "" {
					if info, ok := suiteMap[suiteName]; ok {
						info.runFunc = funcDecl
					}
				}
			}
		}
	}
	
	return suites
}

func (st *suiteTransformer) embedsSuite(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if field.Names == nil { // embedded field
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "suite" {
					if sel.Sel.Name == "Suite" {
						return true
					}
				}
			}
		}
	}
	return false
}

func (st *suiteTransformer) getNonSuiteFields(structType *ast.StructType) []*ast.Field {
	var fields []*ast.Field
	for _, field := range structType.Fields.List {
		if field.Names != nil || !st.isSuiteField(field) {
			fields = append(fields, field)
		}
	}
	return fields
}

func (st *suiteTransformer) isSuiteField(field *ast.Field) bool {
	if sel, ok := field.Type.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "suite" {
			return sel.Sel.Name == "Suite"
		}
	}
	return false
}

func (st *suiteTransformer) getReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func (st *suiteTransformer) findSuiteRun(funcDecl *ast.FuncDecl) string {
	var suiteName string
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "suite" {
					if sel.Sel.Name == "Run" && len(call.Args) >= 2 {
						// Extract suite type from new(SuiteType) call
						if newCall, ok := call.Args[1].(*ast.CallExpr); ok {
							if newIdent, ok := newCall.Fun.(*ast.Ident); ok && newIdent.Name == "new" {
								if len(newCall.Args) > 0 {
									if typeIdent, ok := newCall.Args[0].(*ast.Ident); ok {
										suiteName = typeIdent.Name
									}
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	return suiteName
}

func (st *suiteTransformer) transformSuite(suite *suiteInfo) {
	// Remove the suite.Suite embedding
	st.removeSuiteEmbedding(suite)
	
	// Create a setup helper function
	if suite.setupTest != nil || suite.tearDown != nil {
		st.createSetupHelper(suite)
	}
	
	// Transform test methods to regular test functions
	for _, method := range suite.testMethods {
		st.transformTestMethod(suite, method)
	}
	
	// Remove the original methods and run function
	st.removeOriginalMethods(suite)
}

func (st *suiteTransformer) removeSuiteEmbedding(suite *suiteInfo) {
	// Update the struct to remove suite.Suite
	genDecl := suite.typeDecl
	if genDecl != nil {
		for _, spec := range genDecl.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					var newFields []*ast.Field
					for _, field := range structType.Fields.List {
						if !st.isSuiteField(field) {
							newFields = append(newFields, field)
						}
					}
					structType.Fields.List = newFields
				}
			}
		}
	}
}

func (st *suiteTransformer) createSetupHelper(suite *suiteInfo) {
	// Create setup function name
	baseName := strings.TrimSuffix(suite.typeName, "Suite")
	if !strings.HasSuffix(baseName, "Test") {
		baseName += "Test"
	}
	setupName := fmt.Sprintf("setup%s", baseName)
	
	// Build the function body
	var stmts []ast.Stmt
	
	// Create suite instance
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("suite")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.UnaryExpr{
				Op: token.AND,
				X: &ast.CompositeLit{
					Type: ast.NewIdent(suite.typeName),
				},
			},
		},
	})
	
	// Call SetupTest if exists
	if suite.setupTest != nil {
		for _, stmt := range suite.setupTest.Body.List {
			// Transform suite receiver references
			transformedStmt := st.transformReceiverRef(stmt, "suite", "suite")
			stmts = append(stmts, transformedStmt)
		}
	}
	
	// Add cleanup if TearDownTest exists
	if suite.tearDown != nil {
		cleanupStmts := []ast.Stmt{}
		for _, stmt := range suite.tearDown.Body.List {
			transformedStmt := st.transformReceiverRef(stmt, "suite", "suite")
			cleanupStmts = append(cleanupStmts, transformedStmt)
		}
		
		stmts = append(stmts, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("t"),
					Sel: ast.NewIdent("Cleanup"),
				},
				Args: []ast.Expr{
					&ast.FuncLit{
						Type: &ast.FuncType{},
						Body: &ast.BlockStmt{
							List: cleanupStmts,
						},
					},
				},
			},
		})
	}
	
	// Return suite
	stmts = append(stmts, &ast.ReturnStmt{
		Results: []ast.Expr{ast.NewIdent("suite")},
	})
	
	// Create the helper function
	helperFunc := &ast.FuncDecl{
		Name: ast.NewIdent(setupName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("t")},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   ast.NewIdent("testing"),
								Sel: ast.NewIdent("T"),
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: ast.NewIdent(suite.typeName),
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: stmts,
		},
	}
	
	// Add the helper function to the file
	st.file.Decls = append(st.file.Decls, helperFunc)
}

func (st *suiteTransformer) transformTestMethod(suite *suiteInfo, method *ast.FuncDecl) {
	// Create new test function name
	testName := method.Name.Name
	
	// Build the function body
	var stmts []ast.Stmt
	
	// Call setup helper if exists
	if suite.setupTest != nil || suite.tearDown != nil {
		baseName := strings.TrimSuffix(suite.typeName, "Suite")
		if !strings.HasSuffix(baseName, "Test") {
			baseName += "Test"
		}
		setupName := fmt.Sprintf("setup%s", baseName)
		stmts = append(stmts, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("suite")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun:  ast.NewIdent(setupName),
					Args: []ast.Expr{ast.NewIdent("t")},
				},
			},
		})
	} else {
		// Just create suite instance
		stmts = append(stmts, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("suite")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.UnaryExpr{
					Op: token.AND,
					X: &ast.CompositeLit{
						Type: ast.NewIdent(suite.typeName),
					},
				},
			},
		})
	}
	
	// Copy method body, transforming assertions
	for _, stmt := range method.Body.List {
		transformedStmt := st.transformSuiteAssertion(stmt, suite)
		stmts = append(stmts, transformedStmt)
	}
	
	// Create the new test function
	testFunc := &ast.FuncDecl{
		Name: ast.NewIdent(testName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("t")},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   ast.NewIdent("testing"),
								Sel: ast.NewIdent("T"),
							},
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: stmts,
		},
	}
	
	// Add the test function to the file
	st.file.Decls = append(st.file.Decls, testFunc)
}

func (st *suiteTransformer) transformSuiteAssertion(stmt ast.Stmt, suite *suiteInfo) ast.Stmt {
	// Transform suite.Equal() to assert.Equal(t, ...)
	return st.transformStmt(stmt, func(n ast.Node) ast.Node {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "suite" {
					// Check if it's an assertion method
					if st.isAssertionMethod(sel.Sel.Name) {
						// Transform to assert.Method(t, ...)
						newArgs := []ast.Expr{ast.NewIdent("t")}
						newArgs = append(newArgs, call.Args...)
						
						return &ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   ast.NewIdent("assert"),
								Sel: sel.Sel,
							},
							Args: newArgs,
						}
					}
				}
			}
		}
		return n
	})
}

func (st *suiteTransformer) isAssertionMethod(name string) bool {
	methods := []string{
		"Equal", "NotEqual", "True", "False", "Nil", "NotNil",
		"Empty", "NotEmpty", "Error", "NoError", "ErrorIs", "ErrorAs",
		"Contains", "NotContains", "Len", "Greater", "GreaterOrEqual",
		"Less", "LessOrEqual", "InDelta", "InEpsilon",
	}
	for _, m := range methods {
		if name == m {
			return true
		}
	}
	return false
}

func (st *suiteTransformer) transformReceiverRef(stmt ast.Stmt, oldName, newName string) ast.Stmt {
	return st.transformStmt(stmt, func(n ast.Node) ast.Node {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == oldName {
				sel.X = ast.NewIdent(newName)
			}
		}
		return n
	})
}

func (st *suiteTransformer) transformStmt(stmt ast.Stmt, transform func(ast.Node) ast.Node) ast.Stmt {
	// This is a simplified version - a full implementation would need
	// to handle all statement types properly
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		s.X = transform(s.X).(ast.Expr)
	case *ast.AssignStmt:
		for i, rhs := range s.Rhs {
			s.Rhs[i] = transform(rhs).(ast.Expr)
		}
	case *ast.IfStmt:
		s.Cond = transform(s.Cond).(ast.Expr)
		for i, bodyStmt := range s.Body.List {
			s.Body.List[i] = st.transformStmt(bodyStmt, transform)
		}
	}
	return stmt
}

func (st *suiteTransformer) removeOriginalMethods(suite *suiteInfo) {
	// Build a set of declarations to remove
	toRemove := make(map[ast.Decl]bool)
	
	if suite.setupTest != nil {
		toRemove[suite.setupTest] = true
	}
	if suite.tearDown != nil {
		toRemove[suite.tearDown] = true
	}
	if suite.runFunc != nil {
		toRemove[suite.runFunc] = true
	}
	for _, method := range suite.testMethods {
		toRemove[method] = true
	}
	
	// Filter declarations
	var newDecls []ast.Decl
	for _, decl := range st.file.Decls {
		if !toRemove[decl] {
			newDecls = append(newDecls, decl)
		}
	}
	st.file.Decls = newDecls
}