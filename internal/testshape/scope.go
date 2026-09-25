// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
	"maps"
	"path"
	"slices"
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

// scope is what a body in one top-level declaration can see: the file's
// imports, the declaration's testing values, and the closures stored and the
// types names are declared with in the body and around it. A subtest's body
// sees what surrounds it, but nothing a sibling subtest declares.
type scope struct {
	pkg     *pkg
	imports map[string]bool
	values  map[string]valueKind
	// closures are the function literals stored under a name or a field path,
	// as in fail or deps.Post, and stored is every literal in the declaration
	// stored so: one counts only when its name is called or handed on.
	closures map[string]*ast.FuncLit
	stored   map[*ast.FuncLit]bool
	types    map[string]seenType
}

// newScope reads the testing values and stored literals anywhere in a
// declaration, and the types of its receiver and parameters. What its body
// declares, enter adds.
func newScope(p *pkg, file *ast.File, function *ast.FuncDecl) scope {
	visible := scope{
		pkg: p, imports: importNames(file), values: map[string]valueKind{},
		closures: map[string]*ast.FuncLit{}, stored: map[*ast.FuncLit]bool{}, types: map[string]seenType{},
	}
	testing := testingName(file)

	if function.Recv != nil {
		visible.declareFields(function.Recv)
	}

	visible.declareFields(function.Type.Params)

	ast.Inspect(function, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			visible.addValues(node, testing)
		case *ast.AssignStmt:
			visible.markStored(node.Rhs)
		case *ast.ValueSpec:
			visible.markStored(node.Values)
		}

		return true
	})

	return visible
}

// helperScope is what a helper's whole body sees.
func helperScope(p *pkg, file *ast.File, function *ast.FuncDecl) scope {
	return newScope(p, file, function).enter(function.Body, nil)
}

// enter is the scope inside a body: what s sees, with the closures and types
// the body declares over it, outside the subtests given, which enter scopes of
// their own.
func (s scope) enter(body *ast.BlockStmt, subtests []*ast.FuncLit) scope {
	inner := s
	inner.closures = maps.Clone(s.closures)
	inner.types = maps.Clone(s.types)

	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return !slices.Contains(subtests, node)
		case *ast.FuncType:
			inner.declareFields(node.Params)
		case *ast.AssignStmt:
			inner.addAssignment(node)
		case *ast.ValueSpec:
			inner.addSpec(node)
		case *ast.RangeStmt:
			inner.declareRange(node)
		}

		return true
	})

	return inner
}

// markStored records the function literals among the values an assignment or
// var declaration stores.
func (s scope) markStored(values []ast.Expr) {
	for _, value := range values {
		if literal, ok := value.(*ast.FuncLit); ok {
			s.stored[literal] = true
		}
	}
}

// addAssignment records the closures an assignment stores, and the names a :=
// declares.
func (s scope) addAssignment(assignment *ast.AssignStmt) {
	s.addClosures(assignment.Lhs, assignment.Rhs)

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

	s.addClosures(names, spec.Values)
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

// addClosures records each function literal among values under the name or
// field path it is stored in.
func (s scope) addClosures(names, values []ast.Expr) {
	for index := range min(len(names), len(values)) {
		literal, isLiteral := values[index].(*ast.FuncLit)
		path, named := pathOf(names[index])

		if isLiteral && named {
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
