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
// testing values, and the closures it assigns to names.
type scope struct {
	pkg      *pkg
	imports  map[string]bool
	values   map[string]valueKind
	closures map[string]*ast.FuncLit
}

// newScope reads the testing values and closures anywhere in a declaration.
func newScope(p *pkg, file *ast.File, function *ast.FuncDecl) scope {
	visible := scope{
		pkg: p, imports: importNames(file), values: map[string]valueKind{}, closures: map[string]*ast.FuncLit{},
	}
	testing := testingName(file)

	ast.Inspect(function, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			visible.addValues(node, testing)
		case *ast.AssignStmt:
			visible.addAssignedClosures(node.Lhs, node.Rhs)
		case *ast.ValueSpec:
			for index, name := range node.Names {
				if index < len(node.Values) {
					visible.addClosure(name, node.Values[index])
				}
			}
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

// addAssignedClosures records each name assigned a function literal.
func (s scope) addAssignedClosures(names, values []ast.Expr) {
	if len(names) != len(values) {
		return
	}

	for index, name := range names {
		if ident, ok := name.(*ast.Ident); ok {
			s.addClosure(ident, values[index])
		}
	}
}

// addClosure records name as a closure when value is a function literal.
func (s scope) addClosure(name *ast.Ident, value ast.Expr) {
	if literal, ok := value.(*ast.FuncLit); ok {
		s.closures[name.Name] = literal
	}
}

// reaches reports whether any of the nodes can fail the test.
func (s scope) reaches(nodes []ast.Stmt) bool {
	return s.reachesVisiting(nodes, map[*ast.FuncLit]bool{})
}

// reachesVisiting is reaches, remembering the closures already followed so a
// closure that calls itself ends.
func (s scope) reachesVisiting(nodes []ast.Stmt, visited map[*ast.FuncLit]bool) bool {
	for _, node := range nodes {
		found := false

		ast.Inspect(node, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if found || (ok && s.fails(call, visited)) {
				found = true

				return false
			}

			return true
		})

		if found {
			return true
		}
	}

	return false
}

// fails reports whether a call can fail the test: a failure method on a test
// value, a closure that reaches one, or an asserting helper handed the test.
func (s scope) fails(call *ast.CallExpr, visited map[*ast.FuncLit]bool) bool {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		if literal, ok := s.closures[function.Name]; ok && !visited[literal] {
			visited[literal] = true

			return s.reachesVisiting(literal.Body.List, visited)
		}

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
