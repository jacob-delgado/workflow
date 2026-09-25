// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
)

// state is how far through Arrange, Act and Assert a body has got.
type state int

const (
	start state = iota
	arranged
	acting
	asserted
)

// transition is where a marker leads from a state, or what it breaks.
type transition struct {
	next    state
	problem string
}

// transitions is the marker grammar: an optional Arrange, then one or more
// cycles, each an Act and its Assert or a single Act & Assert.
func transitions() map[state]map[markerKind]transition {
	return map[state]map[markerKind]transition{
		start: {
			arrange: {next: arranged}, act: {next: acting}, actAndAssert: {next: asserted},
			assert: {problem: "an Assert before any Act"},
		},
		arranged: {
			act: {next: acting}, actAndAssert: {next: asserted},
			arrange: {problem: "a second Arrange"}, assert: {problem: "an Assert before any Act"},
		},
		acting: {
			assert:       {next: asserted},
			arrange:      {problem: "an Arrange between an Act and its Assert"},
			act:          {problem: "an Act before the last one's Assert"},
			actAndAssert: {problem: "an Act & Assert before the last Act's Assert"},
		},
		asserted: {
			act: {next: acting}, actAndAssert: {next: asserted},
			arrange: {problem: "an Arrange after an Assert: setup for a later step belongs at the start of that step's Act"},
			assert:  {problem: "two Asserts in a row: an Assert follows its own Act"},
		},
	}
}

// unfinished is what a body that ends in a state leaves out.
func unfinished() map[state]string {
	//nolint:exhaustive // only the states that can be left dangling have a message; the rest are complete.
	return map[state]string{
		arranged: "an Arrange with no Act after it",
		acting:   "an Act with no Assert after it",
	}
}

// section is a marker and the top-level statements under it.
type section struct {
	marker     marker
	statements []ast.Stmt
}

// leaf checks a body that runs no subtests: its markers, their order and
// labels, and that each Assert reaches a failure.
func (b bodyCheck) leaf(body *ast.BlockStmt, reportAt token.Pos) []Violation {
	found := b.pkg.readComments(b.file, body, nil)

	if problems := b.malformedOrMisplaced(body, found); len(problems) > 0 {
		return problems
	}

	if len(found.markers) == 0 {
		return []Violation{b.violation(reportAt, MissingMarkers)}
	}

	sections, leading := split(body, found.markers)
	for _, statement := range leading {
		if !b.isParallel(statement) {
			return []Violation{b.violation(statement.Pos(), CodeBeforeMarker)}
		}
	}

	if problem, ok := b.order(found.markers); !ok {
		return []Violation{problem}
	}

	return append(b.labels(found.markers), b.contents(sections)...)
}

// malformedOrMisplaced reports markers written wrongly or put where no section
// can start. Either stops the rest of the check, which would only guess at what
// was meant.
func (b bodyCheck) malformedOrMisplaced(body *ast.BlockStmt, found comments) []Violation {
	var problems []Violation

	for _, pos := range found.malformed {
		problems = append(problems, b.violation(pos, MalformedMarker))
	}

	for _, marker := range found.markers {
		if b.pkg.misplaced(body, marker.pos) {
			problems = append(problems, b.violation(marker.pos, MarkerPlacement))
		}
	}

	return problems
}

// split divides a body's top-level statements among its markers, and returns
// those before the first.
func split(body *ast.BlockStmt, markers []marker) ([]section, []ast.Stmt) {
	sections := make([]section, len(markers))

	var leading []ast.Stmt

	for index := range markers {
		sections[index].marker = markers[index]
	}

	for _, statement := range body.List {
		owner := -1

		for index, marker := range markers {
			if marker.pos < statement.Pos() {
				owner = index
			}
		}

		if owner < 0 {
			leading = append(leading, statement)

			continue
		}

		sections[owner].statements = append(sections[owner].statements, statement)
	}

	return sections, leading
}

// isParallel reports whether a statement is t.Parallel() on a test value.
func (b bodyCheck) isParallel(statement ast.Stmt) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}

	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Parallel" {
		return false
	}

	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	kind, isValue := b.scope.values[receiver.Name]

	return isValue && kind == kindT
}

// order walks the markers through the grammar and reports the first one out of
// place, or a body that stops before its last Assert.
func (b bodyCheck) order(markers []marker) (Violation, bool) {
	current := start

	for _, marker := range markers {
		next := b.pkg.transitions[current][marker.kind]
		if next.problem != "" {
			return b.orderViolation(marker.pos, next.problem), false
		}

		current = next.next
	}

	if problem, ok := b.pkg.unfinished[current]; ok {
		return b.orderViolation(markers[len(markers)-1].pos, problem), false
	}

	return Violation{}, true
}

// orderViolation is a marker-order violation that says which order broke.
func (b bodyCheck) orderViolation(pos token.Pos, problem string) Violation {
	violation := b.violation(pos, MarkerOrder)
	violation.Message = problem

	return violation
}

// labels holds a flow of several cycles to labeling every step, and a single
// cycle or an Arrange to carrying none.
func (b bodyCheck) labels(markers []marker) []Violation {
	cycles := 0

	for _, marker := range markers {
		if marker.kind == act || marker.kind == actAndAssert {
			cycles++
		}
	}

	var problems []Violation

	for _, marker := range markers {
		switch {
		case marker.kind == arrange || cycles == 1:
			if marker.labeled {
				problems = append(problems, b.violation(marker.pos, LabelUnexpected))
			}
		case !marker.labeled:
			problems = append(problems, b.violation(marker.pos, LabelRequired))
		}
	}

	return problems
}

// contents reports sections with nothing in them, and Asserts that reach no
// failure.
func (b bodyCheck) contents(sections []section) []Violation {
	var problems []Violation

	for _, section := range sections {
		switch {
		case len(section.statements) == 0:
			problems = append(problems, b.violation(section.marker.pos, EmptySection))
		case section.marker.kind == assert || section.marker.kind == actAndAssert:
			problems = append(problems, b.unasserted(section)...)
		}
	}

	return problems
}

// unasserted reports an Assert that reaches no failure, at its marker, and one
// that reaches a failure only through a method the check cannot pick, at that
// method's call.
func (b bodyCheck) unasserted(section section) []Violation {
	found := b.scope.reaches(section.statements)

	if found.outcome == failing {
		return nil
	}

	if found.outcome == ambiguous {
		return []Violation{b.violation(found.at, AmbiguousHelper)}
	}

	return []Violation{b.violation(section.marker.pos, AssertWithoutFailure)}
}
