package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

// astWalker walks the AST and transforms testify assertions
type astWalker struct {
	transformer *testifyTransformer
	file        *ast.File
	fset        *token.FileSet
	
	// Current context
	currentFunc  *ast.FuncDecl
	currentBlock *ast.BlockStmt
	blockStack   []*ast.BlockStmt
	
	// Track modifications
	modified bool
}

func newASTWalker(transformer *testifyTransformer, file *ast.File, fset *token.FileSet) *astWalker {
	return &astWalker{
		transformer: transformer,
		file:        file,
		fset:        fset,
		blockStack:  make([]*ast.BlockStmt, 0),
	}
}

func (w *astWalker) walk() {
	ast.Inspect(w.file, w.inspect)
}

func (w *astWalker) inspect(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.FuncDecl:
		w.currentFunc = n
		if n.Body != nil {
			w.pushBlock(n.Body)
			defer w.popBlock()
		}
		
	case *ast.BlockStmt:
		if n != w.currentBlock && n != nil {
			w.pushBlock(n)
			defer w.popBlock()
		}
		
	case *ast.IfStmt:
		// Handle if body
		if n.Body != nil {
			w.pushBlock(n.Body)
			ast.Inspect(n.Body, w.inspect)
			w.popBlock()
		}
		// Handle else body
		if n.Else != nil {
			if block, ok := n.Else.(*ast.BlockStmt); ok {
				w.pushBlock(block)
				ast.Inspect(block, w.inspect)
				w.popBlock()
			} else {
				ast.Inspect(n.Else, w.inspect)
			}
		}
		return false
		
	case *ast.ForStmt:
		if n.Body != nil {
			w.pushBlock(n.Body)
			defer w.popBlock()
		}
		
	case *ast.RangeStmt:
		if n.Body != nil {
			w.pushBlock(n.Body)
			defer w.popBlock()
		}
		
	case *ast.ImportSpec:
		w.transformer.checkImport(n)
		
	case *ast.ExprStmt:
		if call, ok := n.X.(*ast.CallExpr); ok {
			if w.shouldTransform(call) {
				w.transformStatement(n, call)
				return false
			}
		}
	}
	
	return true
}

func (w *astWalker) pushBlock(block *ast.BlockStmt) {
	w.blockStack = append(w.blockStack, block)
	w.currentBlock = block
}

func (w *astWalker) popBlock() {
	if len(w.blockStack) > 0 {
		w.blockStack = w.blockStack[:len(w.blockStack)-1]
		if len(w.blockStack) > 0 {
			w.currentBlock = w.blockStack[len(w.blockStack)-1]
		} else {
			w.currentBlock = nil
		}
	}
}

func (w *astWalker) shouldTransform(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	
	return pkgIdent.Name == "assert" || pkgIdent.Name == "require"
}

func (w *astWalker) transformStatement(stmt *ast.ExprStmt, call *ast.CallExpr) {
	sel := call.Fun.(*ast.SelectorExpr)
	pkgIdent := sel.X.(*ast.Ident)
	methodName := sel.Sel.Name
	isFatal := pkgIdent.Name == "require"
	
	// Create a new transformer context
	ctx := &transformerContext{
		walker:       w,
		stmt:         stmt,
		call:         call,
		isFatal:      isFatal,
		currentBlock: w.currentBlock,
	}
	
	// Transform based on method name
	switch methodName {
	case "Equal":
		ctx.transformEqual()
	case "NotEqual":
		ctx.transformNotEqual()
	case "True":
		ctx.transformTrue()
	case "False":
		ctx.transformFalse()
	case "Nil":
		ctx.transformNil()
	case "NotNil":
		ctx.transformNotNil()
	case "Empty":
		ctx.transformEmpty()
	case "NotEmpty":
		ctx.transformNotEmpty()
	case "Error":
		ctx.transformError()
	case "NoError":
		ctx.transformNoError()
	case "ErrorIs":
		ctx.transformErrorIs()
	case "ErrorAs":
		ctx.transformErrorAs()
	case "Contains":
		ctx.transformContains()
	case "NotContains":
		ctx.transformNotContains()
	case "Len":
		ctx.transformLen()
	case "Greater":
		ctx.transformGreater()
	case "GreaterOrEqual":
		ctx.transformGreaterOrEqual()
	case "Less":
		ctx.transformLess()
	case "LessOrEqual":
		ctx.transformLessOrEqual()
	case "InDelta":
		ctx.transformInDelta()
	case "InEpsilon":
		ctx.transformInEpsilon()
	}
	
	if ctx.modified {
		w.modified = true
	}
}

// transformerContext holds the context for a single transformation
type transformerContext struct {
	walker       *astWalker
	stmt         *ast.ExprStmt
	call         *ast.CallExpr
	isFatal      bool
	currentBlock *ast.BlockStmt
	modified     bool
}

func (ctx *transformerContext) replace(newStmt ast.Stmt) {
	for i, stmt := range ctx.currentBlock.List {
		if stmt == ctx.stmt {
			ctx.currentBlock.List[i] = newStmt
			ctx.modified = true
			if ctx.walker.transformer.preserveMessages {
				// Use verbose flag for debugging
				fmt.Printf("Replaced statement at index %d\n", i)
			}
			return
		}
	}
	if ctx.walker.transformer.preserveMessages {
		fmt.Printf("Warning: Could not find statement to replace\n")
	}
}

func (ctx *transformerContext) extractMessage(args []ast.Expr) string {
	if len(args) == 0 || !ctx.walker.transformer.preserveMessages {
		return ""
	}
	
	if lit, ok := args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value[1:len(lit.Value)-1] // Remove quotes
	}
	
	return ""
}

// Transformation methods

func (ctx *transformerContext) transformEqual() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	expected := ctx.call.Args[1]
	actual := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	// Check if we need cmp for complex types
	if ctx.walker.transformer.needsCmp(expected) || ctx.walker.transformer.needsCmp(actual) {
		if !ctx.walker.transformer.stdlibOnly {
			ctx.walker.transformer.importsToAdd["github.com/google/go-cmp/cmp"] = true
			ctx.replaceWithCmpDiff(tVar, expected, actual, msg)
			return
		}
	}
	
	ctx.replaceWithIfNotEqual(tVar, expected, actual, msg)
}

func (ctx *transformerContext) transformNotEqual() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	notExpected := ctx.call.Args[1]
	actual := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithIfEqual(tVar, notExpected, actual, msg)
}

func (ctx *transformerContext) transformTrue() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	condition := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfNotCondition(tVar, condition, msg)
}

func (ctx *transformerContext) transformFalse() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	condition := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfCondition(tVar, condition, msg)
}

func (ctx *transformerContext) transformNil() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	value := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfNotNil(tVar, value, msg)
}

func (ctx *transformerContext) transformNotNil() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	value := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfNil(tVar, value, msg)
}

func (ctx *transformerContext) transformEmpty() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	value := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfNotEmpty(tVar, value, msg)
}

func (ctx *transformerContext) transformNotEmpty() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	value := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfEmpty(tVar, value, msg)
}

func (ctx *transformerContext) transformError() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	err := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfNoError(tVar, err, msg)
}

func (ctx *transformerContext) transformNoError() {
	if len(ctx.call.Args) < 2 {
		return
	}
	
	tVar := ctx.call.Args[0]
	err := ctx.call.Args[1]
	msg := ""
	if len(ctx.call.Args) > 2 {
		msg = ctx.extractMessage(ctx.call.Args[2:])
	}
	
	ctx.replaceWithIfError(tVar, err, msg)
}

func (ctx *transformerContext) transformErrorIs() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	ctx.walker.transformer.importsToAdd["errors"] = true
	
	tVar := ctx.call.Args[0]
	err := ctx.call.Args[1]
	target := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithErrorsIs(tVar, err, target, msg)
}

func (ctx *transformerContext) transformErrorAs() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	ctx.walker.transformer.importsToAdd["errors"] = true
	
	tVar := ctx.call.Args[0]
	err := ctx.call.Args[1]
	target := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithErrorsAs(tVar, err, target, msg)
}

func (ctx *transformerContext) transformContains() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	container := ctx.call.Args[1]
	element := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	if ctx.walker.transformer.isString(container) {
		ctx.walker.transformer.importsToAdd["strings"] = true
		ctx.replaceWithStringsContains(tVar, container, element, msg)
	} else {
		ctx.walker.transformer.importsToAdd["slices"] = true
		ctx.replaceWithSlicesContains(tVar, container, element, msg)
	}
}

func (ctx *transformerContext) transformNotContains() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	container := ctx.call.Args[1]
	element := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	if ctx.walker.transformer.isString(container) {
		ctx.walker.transformer.importsToAdd["strings"] = true
		ctx.replaceWithStringsNotContains(tVar, container, element, msg)
	} else {
		ctx.walker.transformer.importsToAdd["slices"] = true
		ctx.replaceWithSlicesNotContains(tVar, container, element, msg)
	}
}

func (ctx *transformerContext) transformLen() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	value := ctx.call.Args[1]
	expectedLen := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithLenCheck(tVar, value, expectedLen, msg)
}

func (ctx *transformerContext) transformGreater() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	a := ctx.call.Args[1]
	b := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithComparison(tVar, a, ">", b, msg)
}

func (ctx *transformerContext) transformGreaterOrEqual() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	a := ctx.call.Args[1]
	b := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithComparison(tVar, a, ">=", b, msg)
}

func (ctx *transformerContext) transformLess() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	a := ctx.call.Args[1]
	b := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithComparison(tVar, a, "<", b, msg)
}

func (ctx *transformerContext) transformLessOrEqual() {
	if len(ctx.call.Args) < 3 {
		return
	}
	
	tVar := ctx.call.Args[0]
	a := ctx.call.Args[1]
	b := ctx.call.Args[2]
	msg := ""
	if len(ctx.call.Args) > 3 {
		msg = ctx.extractMessage(ctx.call.Args[3:])
	}
	
	ctx.replaceWithComparison(tVar, a, "<=", b, msg)
}

func (ctx *transformerContext) transformInDelta() {
	if len(ctx.call.Args) < 4 {
		return
	}
	
	ctx.walker.transformer.importsToAdd["math"] = true
	
	tVar := ctx.call.Args[0]
	expected := ctx.call.Args[1]
	actual := ctx.call.Args[2]
	delta := ctx.call.Args[3]
	msg := ""
	if len(ctx.call.Args) > 4 {
		msg = ctx.extractMessage(ctx.call.Args[4:])
	}
	
	ctx.replaceWithDeltaCheck(tVar, expected, actual, delta, msg)
}

func (ctx *transformerContext) transformInEpsilon() {
	if len(ctx.call.Args) < 4 {
		return
	}
	
	ctx.walker.transformer.importsToAdd["math"] = true
	
	tVar := ctx.call.Args[0]
	expected := ctx.call.Args[1]
	actual := ctx.call.Args[2]
	epsilon := ctx.call.Args[3]
	msg := ""
	if len(ctx.call.Args) > 4 {
		msg = ctx.extractMessage(ctx.call.Args[4:])
	}
	
	ctx.replaceWithEpsilonCheck(tVar, expected, actual, epsilon, msg)
}

// Include all the replacement methods from transform.go here
// (They would be methods on transformerContext instead)

func (ctx *transformerContext) replaceWithIfNotEqual(tVar, expected, actual ast.Expr, msg string) {
	gotIdent := ast.NewIdent("got")
	
	condition := &ast.BinaryExpr{
		X:  gotIdent,
		Op: token.NEQ,
		Y:  expected,
	}
	
	format := "got %v, want %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{gotIdent, expected}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfEqual(tVar, notExpected, actual ast.Expr, msg string) {
	gotIdent := ast.NewIdent("got")
	
	condition := &ast.BinaryExpr{
		X:  gotIdent,
		Op: token.EQL,
		Y:  notExpected,
	}
	
	format := "got %v, want not %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{gotIdent, notExpected}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithCmpDiff(tVar, expected, actual ast.Expr, msg string) {
	diffIdent := ast.NewIdent("diff")
	
	cmpDiffCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("cmp"),
			Sel: ast.NewIdent("Diff"),
		},
		Args: []ast.Expr{expected, actual},
	}
	
	condition := &ast.BinaryExpr{
		X:  diffIdent,
		Op: token.NEQ,
		Y:  &ast.BasicLit{Kind: token.STRING, Value: `""`},
	}
	
	format := "mismatch (-want +got):\n%s"
	if msg != "" {
		format = msg + " " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{diffIdent}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfNotCondition(tVar, condition ast.Expr, msg string) {
	notCondition := &ast.UnaryExpr{
		Op: token.NOT,
		X:  condition,
	}
	
	format := "expected true, got false"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: notCondition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfCondition(tVar, condition ast.Expr, msg string) {
	format := "expected false, got true"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfNotNil(tVar, value ast.Expr, msg string) {
	condition := &ast.BinaryExpr{
		X:  value,
		Op: token.NEQ,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "expected nil, got %v"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{value}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfNil(tVar, value ast.Expr, msg string) {
	needsAssignment := false
	varName := "value"
	var valueExpr ast.Expr = value
	
	if _, ok := value.(*ast.CallExpr); ok {
		needsAssignment = true
		varName = extractVarName(value.(*ast.CallExpr))
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
	
	errorCall := createTestErrorf(tVar, format, nil, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfNotEmpty(tVar, value ast.Expr, msg string) {
	gotIdent := ast.NewIdent("got")
	
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{lenCall}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfEmpty(tVar, value ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, nil, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfNoError(tVar, err ast.Expr, msg string) {
	condition := &ast.BinaryExpr{
		X:  err,
		Op: token.EQL,
		Y:  ast.NewIdent("nil"),
	}
	
	format := "expected error, got nil"
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, nil, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithIfError(tVar, err ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{errExpr}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithErrorsIs(tVar, err, target ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{target, err}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithErrorsAs(tVar, err, target ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{target}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithStringsContains(tVar, container, element ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{container, element}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithSlicesContains(tVar, container, element ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{element}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithStringsNotContains(tVar, container, element ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{container, element}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: stringsContainsCall,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithSlicesNotContains(tVar, container, element ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{element}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: slicesContainsCall,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithLenCheck(tVar, value, expectedLen ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{lenCall, expectedLen}, ctx.isFatal)
	
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
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithComparison(tVar, a ast.Expr, op string, b ast.Expr, msg string) {
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
	
	condition := &ast.BinaryExpr{
		X:  a,
		Op: negOp,
		Y:  b,
	}
	
	format := fmt.Sprintf("expected %%v %s %%v", op)
	if msg != "" {
		format = msg + ": " + format
	}
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{a, b}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithDeltaCheck(tVar, expected, actual, delta ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{actual, delta, expected}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}

func (ctx *transformerContext) replaceWithEpsilonCheck(tVar, expected, actual, epsilon ast.Expr, msg string) {
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
	
	errorCall := createTestErrorf(tVar, format, []ast.Expr{actual, epsilon, expected}, ctx.isFatal)
	
	ifStmt := &ast.IfStmt{
		Cond: condition,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{errorCall},
		},
	}
	
	ctx.replace(ifStmt)
}