// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The arguments t.Run and f.Fuzz take; the function is the last.
const (
	runArguments  = 2
	fuzzArguments = 1
)

// isTest reports whether a declaration is a test go test runs: TestX(*testing.T)
// or FuzzX(*testing.F), where X does not start with a lowercase letter.
// TestMain, benchmarks and examples take other parameters and are not.
func (p *pkg) isTest(file *ast.File, function *ast.FuncDecl) bool {
	if function.Recv != nil || function.Body == nil {
		return false
	}

	want, rest, ok := testPrefix(function.Name.Name)
	if !ok {
		return false
	}

	if first, _ := utf8.DecodeRuneInString(rest); rest != "" && unicode.IsLower(first) {
		return false
	}

	params := function.Type.Params.List
	if len(params) != 1 || len(params[0].Names) > 1 {
		return false
	}

	kind, isValue := kindOf(params[0].Type, testingName(file))

	return isValue && kind == want
}

// testPrefix splits a Test or Fuzz name, saying which parameter it needs.
func testPrefix(name string) (valueKind, string, bool) {
	if rest, ok := strings.CutPrefix(name, "Test"); ok {
		return kindT, rest, true
	}

	if rest, ok := strings.CutPrefix(name, "Fuzz"); ok {
		return kindF, rest, true
	}

	return 0, "", false
}

// bodyCheck checks the bodies of one top-level test.
type bodyCheck struct {
	pkg   *pkg
	file  *ast.File
	scope scope
	test  string
}

// checkTest checks a test and every subtest inside it.
func (p *pkg) checkTest(file *ast.File, function *ast.FuncDecl) []Violation {
	b := bodyCheck{pkg: p, file: file, scope: newScope(p, file, function), test: function.Name.Name}

	return b.body(function.Body, function.Name.Pos())
}

// violation is a rule broken at pos, with the rule's own message.
func (b bodyCheck) violation(pos token.Pos, rule Rule) Violation {
	return Violation{Position: b.pkg.fset.Position(pos), Test: b.test, Rule: rule, Message: b.pkg.messages[rule]}
}

// subtest is a t.Run or f.Fuzz call and the function it runs.
type subtest struct {
	function ast.Expr
}

// body checks a body as a leaf when it runs no subtests, and otherwise as the
// outside of a table whose subtests carry the markers, each in the scope the
// body's own leads into.
func (b bodyCheck) body(body *ast.BlockStmt, reportAt token.Pos) []Violation {
	subtests := b.subtests(body)
	literals := literalsOf(subtests)
	b.scope = b.scope.enter(body, literals)

	if len(subtests) == 0 {
		return b.leaf(body, reportAt)
	}

	return b.outer(body, subtests, literals)
}

// subtests are the t.Run and f.Fuzz calls in a body, not counting those inside
// another subtest.
func (b bodyCheck) subtests(body *ast.BlockStmt) []subtest {
	var found []subtest

	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		function, isSubtest := b.subtestFunction(call)
		if !isSubtest {
			return true
		}

		found = append(found, subtest{function: function})

		return false
	})

	return found
}

// subtestFunction is the function a t.Run or f.Fuzz call runs.
func (b bodyCheck) subtestFunction(call *ast.CallExpr) (ast.Expr, bool) {
	kind, method, isValue := b.scope.methodOnValue(call)

	switch {
	case !isValue:
		return nil, false
	case kind == kindT && method == "Run" && len(call.Args) == runArguments:
		return call.Args[runArguments-1], true
	case kind == kindF && method == "Fuzz" && len(call.Args) == fuzzArguments:
		return call.Args[fuzzArguments-1], true
	default:
		return nil, false
	}
}

// literalsOf are the function literals subtests run.
func literalsOf(subtests []subtest) []*ast.FuncLit {
	var literals []*ast.FuncLit

	for _, subtest := range subtests {
		if literal, ok := subtest.function.(*ast.FuncLit); ok {
			literals = append(literals, literal)
		}
	}

	return literals
}

// outer checks the outside of a table or fuzz test: no markers, and no
// assertion once the subtests start. Setup before them, and its guards, may
// fail the test. Then each subtest is checked as a body of its own.
func (b bodyCheck) outer(body *ast.BlockStmt, subtests []subtest, literals []*ast.FuncLit) []Violation {
	var problems []Violation

	for _, subtest := range subtests {
		if _, ok := subtest.function.(*ast.FuncLit); !ok {
			problems = append(problems, b.violation(subtest.function.Pos(), SubtestLiteral))
		}
	}

	found := b.pkg.readComments(b.file, body, literals)
	for _, marker := range found.markers {
		problems = append(problems, b.violation(marker.pos, TableMarker))
	}

	for _, pos := range found.malformed {
		problems = append(problems, b.violation(pos, TableMarker))
	}

	problems = append(problems, b.assertionsAmongSubtests(body, subtests, literals)...)

	for _, literal := range literals {
		problems = append(problems, b.body(literal.Body, literal.Pos())...)
	}

	return problems
}

// assertionsAmongSubtests reports failures outside the subtests from the first
// subtest on, in the loop after t.Run or after the loop, and calls there that
// may fail the test through a method the check cannot pick.
func (b bodyCheck) assertionsAmongSubtests(
	body *ast.BlockStmt, subtests []subtest, literals []*ast.FuncLit,
) []Violation {
	first := subtests[0].function.Pos()

	var problems []Violation

	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok && within(literal.Pos(), literals) {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok || call.Pos() < first {
			return true
		}

		found := b.scope.fails(call, map[*ast.FuncLit]bool{})
		if found == silent {
			return true
		}

		rule := TableAssertion
		if found == ambiguous {
			rule = AmbiguousHelper
		}

		problems = append(problems, b.violation(call.Pos(), rule))

		return false
	})

	return problems
}
