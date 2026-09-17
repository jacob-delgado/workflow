// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

// header opens every fixture file. A body passed to test starts on line 6.
const header = "package fixture_test\n\nimport \"testing\"\n\n"

// firstFile is the name check gives the first source it parses.
const firstFile = "fixture0_test.go"

// found is a violation reduced to what a case states: where, and which rule.
type found struct {
	file string
	line int
	rule testshape.Rule
}

// at is a violation in the first fixture file.
func at(line int, rule testshape.Rule) found {
	return found{file: firstFile, line: line, rule: rule}
}

// test is a fixture file holding one TestX with body, and any helpers after it.
// The body's first line is line 6.
func test(body string, helpers ...string) string {
	return header + "func TestX(t *testing.T) {\n" + body + "}\n" + strings.Join(helpers, "")
}

// check parses each source as a test file of one package and reports what
// Check finds in them.
func check(t *testing.T, sources ...string) []found {
	t.Helper()

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(sources))

	for index, source := range sources {
		name := fmt.Sprintf("fixture%d_test.go", index)

		file, err := parser.ParseFile(fset, name, source, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v\n%s", name, err, source)
		}

		files = append(files, file)
	}

	violations := testshape.Check(fset, files)
	got := make([]found, 0, len(violations))

	for _, violation := range violations {
		got = append(got, found{file: violation.Position.Filename, line: violation.Position.Line, rule: violation.Rule})
	}

	return got
}
