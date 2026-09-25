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

// scope is what one top-level declaration can see: the file's imports, its own
// testing values, the closures it stores and the types it declares names with.
type scope struct {
	pkg     *pkg
	imports map[string]bool
	values  map[string]valueKind
	// closures are the function literals stored under a name or a field path,
	// as in fail or deps.Post, and stored is every literal stored so: one
	// counts only when its name is called or handed on.
	closures map[string]*ast.FuncLit
	stored   map[*ast.FuncLit]bool
	types    map[string]seenType
}

// newScope reads the testing values, closures and declared types anywhere in a
// declaration.
func newScope(p *pkg, file *ast.File, function *ast.FuncDecl) scope {
	visible := scope{
		pkg: p, imports: importNames(file), values: map[string]valueKind{},
		closures: map[string]*ast.FuncLit{}, stored: map[*ast.FuncLit]bool{}, types: map[string]seenType{},
	}
	testing := testingName(file)

	if function.Recv != nil {
		visible.declareFields(function.Recv)
	}

	ast.Inspect(function, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			visible.addValues(node, testing)
			visible.declareFields(node.Params)
		case *ast.AssignStmt:
			visible.addAssignment(node)
		case *ast.ValueSpec:
			visible.addSpec(node)
		case *ast.RangeStmt:
			visible.declareRange(node)
		}

		return true
	})

	return visible
}

// addAssignment records the closures an assignment stores, and the names a :=
// declares.
func (s scope) addAssignment(assignment *ast.AssignStmt) {
	s.addStored(assignment.Lhs, assignment.Rhs)

	if assignment.Tok == token.DEFINE {
		s.declareAssigned(assignment.Lhs, assignment.Rhs)
	}
}

// addSpec records the closures a var declaration stores, and the names it
// declares.
func (s scope) addSpec(spec *ast.ValueSpec) {
	names := make([]ast.Expr, len(spec.Names))
	for index, name := range spec.Names {
		names[index] = name
	}

	s.addStored(names, spec.Values)
	s.declareSpec(spec)
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
