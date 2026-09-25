// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
	"path"
	"strings"
)

// valueKind is what a testing value is.
type valueKind int

const (
	kindT valueKind = iota
	kindTB
	kindF
)

// testingPath is the testing package's import path.
const testingPath = "testing"

// pkg is one package's test files, with what every check needs to know about
// them: its helpers, which of them assert, and the rules' fixed tables.
type pkg struct {
	fset  *token.FileSet
	files []*ast.File
	// funcs and methods are the package's non-test declarations by name, and
	// fileOf where each is declared.
	funcs     map[string][]*ast.FuncDecl
	methods   map[string][]*ast.FuncDecl
	fileOf    map[*ast.FuncDecl]*ast.File
	asserting map[*ast.FuncDecl]bool

	failureMethods map[string]bool
	keywords       map[string]markerKind
	loose          map[string]bool
	transitions    map[state]map[markerKind]transition
	unfinished     map[state]string
	messages       map[Rule]string
}

// newPackage indexes a package's test files and works out which helpers assert.
func newPackage(fset *token.FileSet, files []*ast.File) *pkg {
	indexed := &pkg{
		fset: fset, files: files,
		funcs: map[string][]*ast.FuncDecl{}, methods: map[string][]*ast.FuncDecl{},
		fileOf: map[*ast.FuncDecl]*ast.File{}, asserting: map[*ast.FuncDecl]bool{},
		failureMethods: map[string]bool{
			"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true,
		},
		keywords: markerKeywords(), loose: looseKeywords(),
		transitions: transitions(), unfinished: unfinished(), messages: messages(),
	}

	for _, file := range files {
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil || indexed.isTest(file, function) {
				continue
			}

			indexed.index(file, function)
		}
	}

	indexed.findAssertingHelpers()

	return indexed
}

// index records a helper under its name.
func (p *pkg) index(file *ast.File, function *ast.FuncDecl) {
	p.fileOf[function] = file

	if function.Recv == nil {
		p.funcs[function.Name.Name] = append(p.funcs[function.Name.Name], function)

		return
	}

	p.methods[function.Name.Name] = append(p.methods[function.Name.Name], function)
}

// findAssertingHelpers marks every helper that reaches a failure, directly or
// through another helper, repeating until nothing changes so a chain of any
// length, or a cycle, settles.
func (p *pkg) findAssertingHelpers() {
	for changed := true; changed; {
		changed = false

		for function, file := range p.fileOf {
			if p.asserting[function] {
				continue
			}

			if newScope(p, file, function).reaches(function.Body.List) {
				p.asserting[function], changed = true, true
			}
		}
	}
}

// anyAsserting reports whether one of the helpers of a name asserts.
func (p *pkg) anyAsserting(helpers []*ast.FuncDecl) bool {
	for _, helper := range helpers {
		if p.asserting[helper] {
			return true
		}
	}

	return false
}

// scope is what one top-level declaration can see: the file's imports, its own
// testing values, and the closures it stores.
type scope struct {
	pkg     *pkg
	imports map[string]bool
	values  map[string]valueKind
	// closures are the function literals stored under a name or a field path,
	// as in fail or deps.Post, and stored is every literal stored so: one
	// counts only when its name is called or handed on.
	closures map[string]*ast.FuncLit
	stored   map[*ast.FuncLit]bool
}

// newScope reads the testing values and closures anywhere in a declaration.
func newScope(p *pkg, file *ast.File, function *ast.FuncDecl) scope {
	visible := scope{
		pkg: p, imports: importNames(file), values: map[string]valueKind{},
		closures: map[string]*ast.FuncLit{}, stored: map[*ast.FuncLit]bool{},
	}
	testing := testingName(file)

	ast.Inspect(function, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			visible.addValues(node, testing)
		case *ast.AssignStmt:
			visible.addStored(node.Lhs, node.Rhs)
		case *ast.ValueSpec:
			names := make([]ast.Expr, len(node.Names))
			for index, name := range node.Names {
				names[index] = name
			}

			visible.addStored(names, node.Values)
		}

		return true
	})

	return visible
}

// methodOnValue reads a call to a method on a testing value: which kind of
// value, and which method.
func (s scope) methodOnValue(call *ast.CallExpr) (valueKind, string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return 0, "", false
	}

	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return 0, "", false
	}

	kind, isValue := s.values[receiver.Name]

	return kind, selector.Sel.Name, isValue
}

// addValues records a function's parameters that are testing values.
func (s scope) addValues(function *ast.FuncType, testing string) {
	for _, field := range function.Params.List {
		kind, ok := kindOf(field.Type, testing)
		if !ok {
			continue
		}

		for _, name := range field.Names {
			s.values[name.Name] = kind
		}
	}
}

// addStored records each function literal among values, under the name or
// field path it is stored in.
func (s scope) addStored(names, values []ast.Expr) {
	for index := range min(len(names), len(values)) {
		literal, ok := values[index].(*ast.FuncLit)
		if !ok {
			continue
		}

		s.stored[literal] = true

		if path, ok := pathOf(names[index]); ok {
			s.closures[path] = literal
		}
	}
}

// pathOf is the name or field path an expression spells, as in fail or
// deps.Post.
func pathOf(expr ast.Expr) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name, true
	case *ast.SelectorExpr:
		outer, ok := pathOf(expr.X)

		return outer + "." + expr.Sel.Name, ok
	default:
		return "", false
	}
}

// reaches reports whether any of the nodes can fail the test.
func (s scope) reaches(nodes []ast.Stmt) bool {
	return s.reachesVisiting(nodes, map[*ast.FuncLit]bool{})
}

// reachesVisiting is reaches, remembering the closures already followed so a
// closure that calls itself ends. A stored literal is not walked where it is
// stored: it counts where its name is called, or handed to a call or set in a
// composite literal, from where it may run.
func (s scope) reachesVisiting(nodes []ast.Stmt, visited map[*ast.FuncLit]bool) bool {
	for _, node := range nodes {
		if s.reachesFrom(node, visited) {
			return true
		}
	}

	return false
}

// reachesFrom reports whether anything in one statement can fail the test.
func (s scope) reachesFrom(statement ast.Stmt, visited map[*ast.FuncLit]bool) bool {
	found := false

	ast.Inspect(statement, func(node ast.Node) bool {
		if found {
			return false
		}

		switch node := node.(type) {
		case *ast.FuncLit:
			return !s.stored[node]
		case *ast.CallExpr:
			found = s.fails(node, visited) || s.handsOnClosure(node.Args, visited)
		case *ast.CompositeLit:
			found = s.handsOnClosure(fieldValues(node.Elts), visited)
		}

		return !found
	})

	return found
}

// handsOnClosure reports whether any of the values names a stored closure that
// can fail the test.
func (s scope) handsOnClosure(values []ast.Expr, visited map[*ast.FuncLit]bool) bool {
	for _, value := range values {
		if reached, _ := s.follow(value, visited); reached {
			return true
		}
	}

	return false
}

// follow reports whether the closure an expression names can fail the test,
// and whether it names one at all. A closure already followed reaches nothing
// more, so one that calls itself ends.
func (s scope) follow(expr ast.Expr, visited map[*ast.FuncLit]bool) (bool, bool) {
	path, ok := pathOf(expr)
	if !ok {
		return false, false
	}

	literal, isClosure := s.closures[path]
	if !isClosure || visited[literal] {
		return false, isClosure
	}

	visited[literal] = true

	return s.reachesVisiting(literal.Body.List, visited), true
}

// fieldValues are the values a composite literal sets, keyed or not.
func fieldValues(elements []ast.Expr) []ast.Expr {
	values := make([]ast.Expr, len(elements))

	for index, element := range elements {
		values[index] = element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			values[index] = pair.Value
		}
	}

	return values
}

// fails reports whether a call can fail the test: a failure method on a test
// value, a closure that reaches one, or an asserting helper handed the test.
func (s scope) fails(call *ast.CallExpr, visited map[*ast.FuncLit]bool) bool {
	if reached, isClosure := s.follow(call.Fun, visited); isClosure {
		return reached
	}

	switch function := call.Fun.(type) {
	case *ast.Ident:
		return s.passesTest(call) && s.pkg.anyAsserting(s.pkg.funcs[function.Name])
	case *ast.SelectorExpr:
		return s.selectorFails(call, function)
	default:
		return false
	}
}

// selectorFails reports whether x.name(...) can fail the test. A failure method
// on *testing.F does not count: inside a fuzz target it panics rather than
// failing the input.
func (s scope) selectorFails(call *ast.CallExpr, function *ast.SelectorExpr) bool {
	if receiver, ok := function.X.(*ast.Ident); ok {
		if kind, isValue := s.values[receiver.Name]; isValue {
			return kind != kindF && s.pkg.failureMethods[function.Sel.Name]
		}

		if s.imports[receiver.Name] {
			return false
		}
	}

	return s.passesTest(call) && s.pkg.anyAsserting(s.pkg.methods[function.Sel.Name])
}

// passesTest reports whether a call hands a helper the test, without which the
// helper cannot fail it.
func (s scope) passesTest(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		ident, ok := arg.(*ast.Ident)
		if !ok {
			continue
		}

		if kind, isValue := s.values[ident.Name]; isValue && kind != kindF {
			return true
		}
	}

	return false
}

// kindOf reads *testing.T, testing.TB or *testing.F, with testing under the name
// the file imports it as.
func kindOf(expr ast.Expr, testing string) (valueKind, bool) {
	if star, ok := expr.(*ast.StarExpr); ok {
		switch selectorIn(star.X, testing) {
		case "T":
			return kindT, true
		case "F":
			return kindF, true
		}

		return 0, false
	}

	return kindTB, selectorIn(expr, testing) == "TB"
}

// selectorIn is the name selected from pkg in expr, or empty.
func selectorIn(expr ast.Expr, pkg string) string {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	if ident, ok := selector.X.(*ast.Ident); !ok || ident.Name != pkg || pkg == "" {
		return ""
	}

	return selector.Sel.Name
}

// testingName is the name a file imports testing under, or empty.
func testingName(file *ast.File) string {
	for _, spec := range file.Imports {
		if importPath(spec) != testingPath {
			continue
		}

		if spec.Name != nil {
			return spec.Name.Name
		}

		return "testing"
	}

	return ""
}

// importNames is every name a file's imports are known by.
func importNames(file *ast.File) map[string]bool {
	names := map[string]bool{}

	for _, spec := range file.Imports {
		if spec.Name != nil {
			names[spec.Name.Name] = true

			continue
		}

		// The last element is the name a package is usually imported as.
		names[path.Base(importPath(spec))] = true
	}

	return names
}

// importPath is an import's path without its quotes. The parser has already
// checked it is a string literal.
func importPath(spec *ast.ImportSpec) string {
	return strings.Trim(spec.Path.Value, "\"`")
}
