package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// transformContext holds the context for transforming a file
type transformContext struct {
	file         *ast.File
	fset         *token.FileSet
	currentFunc  *ast.FuncDecl
	currentBlock *ast.BlockStmt
	insertIndex  int
}

// createIfStmt creates an if statement with the given condition and body
func createIfStmt(condition ast.Expr, body *ast.BlockStmt) *ast.IfStmt {
	return &ast.IfStmt{
		Cond: condition,
		Body: body,
	}
}

// createTestErrorf creates a t.Errorf or t.Fatalf call
func createTestErrorf(t ast.Expr, format string, args []ast.Expr, isFatal bool) *ast.ExprStmt {
	method := "Errorf"
	if isFatal {
		method = "Fatalf"
	}
	
	callArgs := []ast.Expr{
		&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(format)},
	}
	callArgs = append(callArgs, args...)
	
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   t,
				Sel: ast.NewIdent(method),
			},
			Args: callArgs,
		},
	}
}

// replaceStatement replaces a statement in the current block
func (tc *transformContext) replaceStatement(old ast.Stmt, new []ast.Stmt) {
	if tc.currentBlock == nil {
		return
	}
	
	for i, stmt := range tc.currentBlock.List {
		if stmt == old {
			// Replace with new statements
			newList := make([]ast.Stmt, 0, len(tc.currentBlock.List)-1+len(new))
			newList = append(newList, tc.currentBlock.List[:i]...)
			newList = append(newList, new...)
			newList = append(newList, tc.currentBlock.List[i+1:]...)
			tc.currentBlock.List = newList
			break
		}
	}
}

// Implementation of replacement functions

func (t *testifyTransformer) replaceWithIfNotEqual(call *ast.CallExpr, tVar, expected, actual ast.Expr, msg string, isFatal bool) {
	// Find the statement containing this call
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if got := actual; got != expected { t.Errorf(...) }
	gotIdent := ast.NewIdent("got")
	
	// Create the condition: got != expected
	condition := &ast.BinaryExpr{
		X:  gotIdent,
		Op: token.NEQ,
		Y:  expected,
	}
	
	// Create the error message
	format := "got %v, want %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	// Create the error call
	errorCall := createTestErrorf(tVar, format, []ast.Expr{gotIdent, expected}, isFatal)
	
	// Create the if statement with initialization
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{gotIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{actual},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	// Find and replace the expression statement containing the call
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfEqual(call *ast.CallExpr, tVar, notExpected, actual ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	gotIdent := ast.NewIdent("got")
	
	// Create the condition: got == notExpected
	condition := &ast.BinaryExpr{
		X:  gotIdent,
		Op: token.EQL,
		Y:  notExpected,
	}
	
	format := "got %v, want not %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{gotIdent, notExpected}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{gotIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{actual},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithCmpDiff(call *ast.CallExpr, tVar, expected, actual ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if diff := cmp.Diff(expected, actual); diff != "" { t.Errorf(...) }
	diffIdent := ast.NewIdent("diff")
	
	// Create cmp.Diff call
	cmpDiffCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("cmp"),
			Sel: ast.NewIdent("Diff"),
		},
		Args: []ast.Expr{expected, actual},
	}
	
	// Create the condition: diff != ""
	condition := &ast.BinaryExpr{
		X:  diffIdent,
		Op: token.NEQ,
		Y:  &ast.BasicLit{Kind: token.STRING, Value: `""`},
	}
	
	format := "mismatch (-want +got):\n%s"
	if msg != "" {
		format = msg + " " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{diffIdent}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{diffIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{cmpDiffCall},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfNotCondition(call *ast.CallExpr, tVar, condition ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if !condition { t.Errorf(...) }
	notCondition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  condition,
	}
	
	format := "expected true, got false"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: notCondition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfCondition(call *ast.CallExpr, tVar, condition ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	format := "expected false, got true"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfNotNil(call *ast.CallExpr, tVar, value ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if value != nil { t.Errorf(...) }
	condition := &ast.BinaryExpr{
		X:  value,
		Op: token.NEQ,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "expected nil, got %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{value}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfNil(call *ast.CallExpr, tVar, value ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Check if we need to assign first
	needsAssignment := false
	varName := "value"
	var valueExpr ast.Expr = value
	
	if callExpr, ok := value.(*ast.CallExpr); ok {
		needsAssignment = true
		varName = extractVarName(callExpr)
		valueExpr = ast.NewIdent(varName)
	}
	
	condition := &ast.BinaryExpr{
		X:  valueExpr,
		Op: token.EQL,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "expected non-nil value"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, isFatal)
	
	var ifStmt ast.Stmt
	if needsAssignment {
		ifStmt = &ast.IfStmt{
			Init: &ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent(varName)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{value},
			},
			Cond: condition,
			Body: &ast.BlockStmt{
				List: []ast.Stmt{errorCall},
			},
		}
	} else {
		ifStmt = &ast.IfStmt{
			Cond: condition,
			Body: &ast.BlockStmt{
				List: []ast.Stmt{errorCall},
			},
		}
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfNotEmpty(call *ast.CallExpr, tVar, value ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	gotIdent := ast.NewIdent("got")
	
	// Create: if got := value; len(got) != 0 { t.Errorf(...) }
	lenCall := &ast.CallExpr{
		Fun:  ast.NewIdent("len"),
		Args: []ast.Expr{gotIdent},
	}
	
	condition := &ast.BinaryExpr{
		X:  lenCall,
		Op: token.NEQ,
		Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
	}
	
	format := "expected empty, got length %d"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{lenCall}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{gotIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{value},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfEmpty(call *ast.CallExpr, tVar, value ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	gotIdent := ast.NewIdent("got")
	
	lenCall := &ast.CallExpr{
		Fun:  ast.NewIdent("len"),
		Args: []ast.Expr{gotIdent},
	}
	
	condition := &ast.BinaryExpr{
		X:  lenCall,
		Op: token.EQL,
		Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
	}
	
	format := "expected non-empty value"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, isFatal)
	
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{gotIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{value},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfNoError(call *ast.CallExpr, tVar, err ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	condition := &ast.BinaryExpr{
		X:  err,
		Op: token.EQL,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "expected error, got nil"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithIfError(call *ast.CallExpr, tVar, err ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Check if we need to assign the error first
	needsAssignment := false
	varName := "err"
	var errExpr ast.Expr = err
	
	if _, ok := err.(*ast.CallExpr); ok {
		needsAssignment = true
		errExpr = ast.NewIdent(varName)
	}
	
	condition := &ast.BinaryExpr{
		X:  errExpr,
		Op: token.NEQ,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "unexpected error: %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{errExpr}, isFatal)
	
	var ifStmt ast.Stmt
	if needsAssignment {
		ifStmt = &ast.IfStmt{
			Init: &ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent(varName)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{err},
			},
			Cond: condition,
			Body: &ast.BlockStmt{
				List: []ast.Stmt{errorCall},
			},
		}
	} else {
		ifStmt = &ast.IfStmt{
			Cond: condition,
			Body: &ast.BlockStmt{
				List: []ast.Stmt{errorCall},
			},
		}
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithErrorsIs(call *ast.CallExpr, tVar, err, target ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if !errors.Is(err, target) { t.Errorf(...) }
	errorsIsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("errors"),
			Sel: ast.NewIdent("Is"),
		},
		Args: []ast.Expr{err, target},
	}
	
	condition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  errorsIsCall,
	}
	
	format := "expected error to be %v, got %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{target, err}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithErrorsAs(call *ast.CallExpr, tVar, err, target ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if !errors.As(err, target) { t.Errorf(...) }
	errorsAsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("errors"),
			Sel: ast.NewIdent("As"),
		},
		Args: []ast.Expr{err, target},
	}
	
	condition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  errorsAsCall,
	}
	
	format := "expected error to be assignable to %T"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{target}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithStringsContains(call *ast.CallExpr, tVar, container, element ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if !strings.Contains(container, element) { t.Errorf(...) }
	stringsContainsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("strings"),
			Sel: ast.NewIdent("Contains"),
		},
		Args: []ast.Expr{container, element},
	}
	
	condition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  stringsContainsCall,
	}
	
	format := "expected %q to contain %q"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{container, element}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithSlicesContains(call *ast.CallExpr, tVar, container, element ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if !slices.Contains(container, element) { t.Errorf(...) }
	slicesContainsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("slices"),
			Sel: ast.NewIdent("Contains"),
		},
		Args: []ast.Expr{container, element},
	}
	
	condition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  slicesContainsCall,
	}
	
	format := "expected slice to contain %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{element}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithStringsNotContains(call *ast.CallExpr, tVar, container, element ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	stringsContainsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("strings"),
			Sel: ast.NewIdent("Contains"),
		},
		Args: []ast.Expr{container, element},
	}
	
	format := "expected %q not to contain %q"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{container, element}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: stringsContainsCall,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithSlicesNotContains(call *ast.CallExpr, tVar, container, element ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	slicesContainsCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("slices"),
			Sel: ast.NewIdent("Contains"),
		},
		Args: []ast.Expr{container, element},
	}
	
	format := "expected slice not to contain %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{element}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: slicesContainsCall,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithLenCheck(call *ast.CallExpr, tVar, value, expectedLen ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	gotIdent := ast.NewIdent("got")
	
	lenCall := &ast.CallExpr{
		Fun:  ast.NewIdent("len"),
		Args: []ast.Expr{gotIdent},
	}
	
	condition := &ast.BinaryExpr{
		X:  lenCall,
		Op: token.NEQ,
		Y:  expectedLen,
	}
	
	format := "got length %d, want %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{lenCall, expectedLen}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{gotIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{value},
		},
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithComparison(call *ast.CallExpr, tVar, a ast.Expr, op string, b ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Map operator string to token
	var negOp token.Token
	switch op {
	case ">":
		negOp = token.LEQ
	case ">=":
		negOp = token.LSS
	case "<":
		negOp = token.GEQ
	case "<=":
		negOp = token.GTR
	}
	
	// Create the negated condition
	condition := &ast.BinaryExpr{
		X:  a,
		Op: negOp,
		Y:  b,
	}
	
	format := fmt.Sprintf("expected %%v %s %%v", op)
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{a, b}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithDeltaCheck(call *ast.CallExpr, tVar, expected, actual, delta ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if math.Abs(expected-actual) > delta { t.Errorf(...) }
	subExpr := &ast.BinaryExpr{
		X:  expected,
		Op: token.SUB,
		Y:  actual,
	}
	
	absCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("math"),
			Sel: ast.NewIdent("Abs"),
		},
		Args: []ast.Expr{subExpr},
	}
	
	condition := &ast.BinaryExpr{
		X:  absCall,
		Op: token.GTR,
		Y:  delta,
	}
	
	format := "expected %v to be within delta %v of %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{actual, delta, expected}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

func (t *testifyTransformer) replaceWithEpsilonCheck(call *ast.CallExpr, tVar, expected, actual, epsilon ast.Expr, msg string, isFatal bool) {
	tc := t.findContext(call)
	if tc == nil {
		return
	}
	
	// Create: if math.Abs(expected-actual)/math.Abs(expected) > epsilon { t.Errorf(...) }
	subExpr := &ast.BinaryExpr{
		X:  expected,
		Op: token.SUB,
		Y:  actual,
	}
	
	absNumerator := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("math"),
			Sel: ast.NewIdent("Abs"),
		},
		Args: []ast.Expr{subExpr},
	}
	
	absDenominator := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("math"),
			Sel: ast.NewIdent("Abs"),
		},
		Args: []ast.Expr{expected},
	}
	
	divExpr := &ast.BinaryExpr{
		X:  absNumerator,
		Op: token.QUO,
		Y:  absDenominator,
	}
	
	condition := &ast.BinaryExpr{
		X:  divExpr,
		Op: token.GTR,
		Y:  epsilon,
	}
	
	format := "expected %v to be within epsilon %v of %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{actual, epsilon, expected}, isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	for i, stmt := range tc.currentBlock.List {
		if exprStmt, ok := stmt.(*ast.ExprStmt); ok && exprStmt.X == call {
			tc.currentBlock.List[i] = ifStmt
			return
		}
	}
}

// Helper function to find the context of a call
func (t *testifyTransformer) findContext(call *ast.CallExpr) *transformContext {
	// This is a simplified version - in a real implementation,
	// we would need to track the current context while walking the AST
	return nil
}

// Helper function to extract a variable name from a function call
// Helper function to extract a variable name from a function call
func extractVarName(call *ast.CallExpr) string {
	// Handle method calls (e.g., obj.getError())
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		name := sel.Sel.Name
		if strings.HasPrefix(name, "get") && len(name) > 3 {
			return strings.ToLower(name[3:])
		}
		if strings.HasPrefix(name, "Get") && len(name) > 3 {
			if len(name) > 4 {
				return strings.ToLower(name[3:4]) + name[4:]
			}
			return strings.ToLower(name[3:])
		}
		if len(name) > 0 {
			return strings.ToLower(name[:1]) + name[1:]
		}
	}
	
	// Handle plain function calls (e.g., getError())
	if ident, ok := call.Fun.(*ast.Ident); ok {
		name := ident.Name
		// Special case for error-returning functions
		if strings.Contains(strings.ToLower(name), "error") {
			return "err"
		}
		if strings.HasPrefix(name, "get") && len(name) > 3 {
			return strings.ToLower(name[3:])
		}
		if strings.HasPrefix(name, "Get") && len(name) > 3 {
			if len(name) > 4 {
				return strings.ToLower(name[3:4]) + name[4:]
			}
			return strings.ToLower(name[3:])
		}
		if len(name) > 0 {
			return strings.ToLower(name[:1]) + name[1:]
		}
	}
	
	return "value"
}
