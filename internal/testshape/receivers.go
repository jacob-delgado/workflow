// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
)

// methodKey names a method by its receiver's type and its own name.
type methodKey struct {
	receiver string
	name     string
}

// seenType is a local type as far as syntax shows it: its name, or shown false
// when syntax does not show which type it is.
type seenType struct {
	name  string
	shown bool
}

// indexMethod records a method under its name, and under its receiver's type
// as well when that is a plain or pointer type name.
func (p *pkg) indexMethod(function *ast.FuncDecl) {
	name := function.Name.Name
	p.methodsNamed[name] = append(p.methodsNamed[name], function)

	for _, field := range function.Recv.List {
		if receiver := typeName(field.Type); receiver.shown {
			p.methods[methodKey{receiver: receiver.name, name: name}] = function
		}
	}
}

// agreed is what every one of the methods does, or ambiguous when they differ.
func (p *pkg) agreed(methods []*ast.FuncDecl) outcome {
	shared := silent

	for index, method := range methods {
		found := p.outcomes[method]
		if index > 0 && found != shared {
			return ambiguous
		}

		shared = found
	}

	return shared
}

// methodOutcome is what the method a selector calls does: the one its
// receiver's type declares, when syntax shows that type and it declares one of
// the name, and otherwise what every method of the name agrees on.
func (s scope) methodOutcome(selector *ast.SelectorExpr) outcome {
	name := selector.Sel.Name

	if receiver := s.typeOf(selector.X); receiver.shown {
		if method, declared := s.pkg.methods[methodKey{receiver: receiver.name, name: name}]; declared {
			return s.pkg.outcomes[method]
		}
	}

	return s.pkg.agreed(s.pkg.methodsNamed[name])
}

// typeOf is the local type a value is of, where syntax shows it: a composite
// literal or its address, new, a call of one of the package's functions, or a
// name declared with one of those or with a type.
func (s scope) typeOf(expr ast.Expr) seenType {
	switch expr := expr.(type) {
	case *ast.Ident:
		return s.types[expr.Name]
	case *ast.CompositeLit:
		return typeName(expr.Type)
	case *ast.UnaryExpr:
		return s.typeOf(expr.X)
	case *ast.CallExpr:
		return s.resultType(expr)
	default:
		return seenType{}
	}
}

// resultType is the type new(T) makes, or the one result of a call to one of
// the package's functions.
func (s scope) resultType(call *ast.CallExpr) seenType {
	function, ok := call.Fun.(*ast.Ident)
	if !ok {
		return seenType{}
	}

	if function.Name == "new" && len(call.Args) == 1 {
		return typeName(call.Args[0])
	}

	for _, declared := range s.pkg.funcs[function.Name] {
		if results := declared.Type.Results; results.NumFields() == 1 {
			return typeName(results.List[0].Type)
		}
	}

	return seenType{}
}

// typeName reads a plain or pointer type name, as in world or *world.
func typeName(expr ast.Expr) seenType {
	switch expr := expr.(type) {
	case *ast.Ident:
		return seenType{name: expr.Name, shown: true}
	case *ast.StarExpr:
		return typeName(expr.X)
	default:
		return seenType{}
	}
}

// declare records the type a name is declared with. A name declared again with
// another type shows none: which of the two a use sees is scoping this check
// does not follow.
func (s scope) declare(name string, declared seenType) {
	if earlier, ok := s.types[name]; ok && earlier != declared {
		declared = seenType{}
	}

	s.types[name] = declared
}

// declareAssigned records the names a := declares, each with its value's type.
func (s scope) declareAssigned(names, values []ast.Expr) {
	for index, name := range names {
		declared := seenType{}
		if len(values) == len(names) {
			declared = s.typeOf(values[index])
		}

		path, _ := pathOf(name)
		s.declare(path, declared)
	}
}

// declareSpec records the names a var declares, with the type it names or else
// each one's value's.
func (s scope) declareSpec(spec *ast.ValueSpec) {
	for index, name := range spec.Names {
		declared := seenType{}

		switch {
		case spec.Type != nil:
			declared = typeName(spec.Type)
		case len(spec.Values) == len(spec.Names):
			declared = s.typeOf(spec.Values[index])
		}

		s.declare(name.Name, declared)
	}
}

// declareRange records the names a range with := declares, whose types syntax
// does not show.
func (s scope) declareRange(loop *ast.RangeStmt) {
	if loop.Tok != token.DEFINE {
		return
	}

	for _, name := range []ast.Expr{loop.Key, loop.Value} {
		if ident, ok := name.(*ast.Ident); ok {
			s.declare(ident.Name, seenType{})
		}
	}
}

// declareFields records the names in a parameter or receiver list with their
// types.
func (s scope) declareFields(fields *ast.FieldList) {
	for _, field := range fields.List {
		declared := typeName(field.Type)

		for _, name := range field.Names {
			s.declare(name.Name, declared)
		}
	}
}
