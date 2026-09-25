// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package testshape checks that every test marks its Arrange, Act and Assert,
// and that each Assert reaches a failure. CLAUDE.md's TDD process section holds
// the rules; cmd/testshape runs this over a package's test files.
//
// It reads syntax alone, without type checking: test values are recognized by
// their declared types, and helpers by name within the package. That is enough
// here, because thelper fixes what a test's parameter is called, and it keeps
// the check to the standard library. A helper method is picked by its
// receiver's type where syntax shows that type (a composite literal, new, a
// call of one of the package's functions, or a name declared with one of those
// or with a type), and otherwise by name while every method of the name agrees
// on whether it asserts; where they disagree, the call is reported. A function
// literal stored under a name counts where that name is called, handed to a
// call or set in a composite literal, and a subtest sees the closures declared
// around it but none a sibling subtest declares.
package testshape

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// Rule names what a test body got wrong.
type Rule string

// The rules a test body is held to.
const (
	MissingMarkers       Rule = "missing-markers"
	CodeBeforeMarker     Rule = "code-before-marker"
	MalformedMarker      Rule = "malformed-marker"
	MarkerPlacement      Rule = "marker-placement"
	MarkerOrder          Rule = "marker-order"
	EmptySection         Rule = "empty-section"
	LabelRequired        Rule = "label-required"
	LabelUnexpected      Rule = "label-unexpected"
	AssertWithoutFailure Rule = "assert-without-failure"
	AmbiguousHelper      Rule = "ambiguous-helper"
	SubtestLiteral       Rule = "subtest-literal"
	TableMarker          Rule = "table-marker"
	TableAssertion       Rule = "table-assertion"
)

// Violation is one broken rule, and how to fix it.
type Violation struct {
	Position token.Position
	// Test is the top-level test the body belongs to, including for a subtest.
	Test    string
	Rule    Rule
	Message string
}

// String reads like a compiler error, so editors can jump to it.
var _ fmt.Stringer = Violation{}

func (v Violation) String() string {
	return fmt.Sprintf("%s: %s: [%s] %s", v.Position, v.Test, v.Rule, v.Message)
}

// Check reports every violation in the test files of one package, in file
// order. Helpers may live in any of the files.
func Check(fset *token.FileSet, files []*ast.File) []Violation {
	pkg := newPackage(fset, files)

	var violations []Violation

	for _, file := range files {
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if ok && pkg.isTest(file, function) {
				violations = append(violations, pkg.checkTest(file, function)...)
			}
		}
	}

	sortViolations(violations)

	return violations
}

// sortViolations orders violations by file, then by where in it they are.
func sortViolations(violations []Violation) {
	slices.SortStableFunc(violations, func(first, second Violation) int {
		if first.Position.Filename != second.Position.Filename {
			return strings.Compare(first.Position.Filename, second.Position.Filename)
		}

		return first.Position.Offset - second.Position.Offset
	})
}

// messages says how to fix each rule. An order violation says which order it
// broke instead.
func messages() map[Rule]string {
	//nolint:exhaustive // only the rules with a fixed message are here; the rest build theirs at the call site.
	return map[Rule]string{
		MissingMarkers:   "no // Arrange, // Act or // Assert markers",
		CodeBeforeMarker: "code before the first marker; only t.Parallel() may come before it",
		MalformedMarker: "not a marker as written: use // Arrange, // Act, // Assert or // Act & Assert, " +
			"with an optional \": label\"",
		MarkerPlacement: "a marker goes on a line of its own, between the body's top-level statements",
		EmptySection:    "nothing under this marker; leave out a part that would be empty",
		LabelRequired:   "a flow of several steps labels every Act and Assert, as in // Act: open the preview",
		LabelUnexpected: "a single Act and Assert carries no label, and an Arrange never does; " +
			"the test name is the label",
		AssertWithoutFailure: "this Assert reaches no t.Error or t.Fatal, directly or through a helper, " +
			"so it asserts nothing",
		AmbiguousHelper: "this call may reach a failure only through a method whose receiver's type is not " +
			"visible, and methods of that name on different types disagree on asserting: " +
			"declare the receiver with its type, as in q := T{} or var q T, or rename one of the methods",
		SubtestLiteral: "t.Run and f.Fuzz need a function literal, so its body can carry the markers",
		TableMarker:    "this test runs subtests: the markers go inside each t.Run or f.Fuzz closure",
		TableAssertion: "an assertion outside the subtests; move it into a case, or into a test of its own",
	}
}
