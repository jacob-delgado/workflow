// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

// assertMarkerLine is the line of the Assert marker in every asserting source.
const assertMarkerLine = 9

// world is a helper type with an asserting method, expect, and one that is not,
// asked.
const world = `
type world struct{}

func (world) expect(t *testing.T, x int) {
	if x != 1 {
		t.Error("x")
	}
}

func (world) asked(x int) bool { return x == 1 }
`

// expectHelper is a helper function that asserts.
const expectHelper = `
func expect(t *testing.T, x int) {
	t.Helper()

	if x != 1 {
		t.Errorf("x = %d", x)
	}
}
`

// asserting is a test whose Act sets x and whose Assert is assert, followed by
// helpers. Its Assert marker is on line 9.
func asserting(assert string, helpers ...string) string {
	return test("\t// Act\n\tx := 1\n\n\t// Assert\n"+assert, helpers...)
}

func TestAnAssertThatReachesAFailurePasses(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"t.Error":         asserting("\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n"),
		"t.Errorf":        asserting("\tif x != 1 {\n\t\tt.Errorf(\"x = %d\", x)\n\t}\n"),
		"t.Fatal":         asserting("\tif x != 1 {\n\t\tt.Fatal(\"x\")\n\t}\n"),
		"t.Fatalf":        asserting("\tif x != 1 {\n\t\tt.Fatalf(\"x = %d\", x)\n\t}\n"),
		"t.Fail":          asserting("\tif x != 1 {\n\t\tt.Fail()\n\t}\n"),
		"t.FailNow":       asserting("\tif x != 1 {\n\t\tt.FailNow()\n\t}\n"),
		"inside a for":    asserting("\tfor range x {\n\t\tt.Error(\"x\")\n\t}\n"),
		"inside a select": asserting("\tselect {\n\tdefault:\n\t\tt.Error(\"x\")\n\t}\n"),
		"inside a go":     asserting("\tgo func() { t.Error(\"x\") }()\n"),
		"inside a closure": asserting("\teach(x, func(int) { t.Error(\"x\") })\n",
			"\nfunc each(n int, visit func(int)) { visit(n) }\n"),
		"through a helper": asserting("\texpect(t, x)\n", expectHelper),
		"through a TB": asserting("\texpectTB(t, x)\n",
			"\nfunc expectTB(tb testing.TB, x int) {\n\tif x != 1 {\n\t\ttb.Error(\"x\")\n\t}\n}\n"),
		"through a method": asserting("\tw := world{}\n\tw.expect(t, x)\n", world),
		"through a closure declared with var": asserting(
			"\tvar count int\n\tvar fail = func() { t.Fatal(\"x\") }\n\tif count == 0 && x != 1 {\n\t\tfail()\n\t}\n"),
		"through a method on a call's result": asserting("\tnewWorld().expect(t, x)\n", world,
			"\nfunc newWorld() world { return world{} }\n"),
		"with other values and fields around": asserting(
			"\tvar holder struct{ hook func() }\n\tholder.hook = func() {}\n\trespond(nil, x)\n\texpect(t, x)\n",
			expectHelper, "\nfunc respond(w *strings.Builder, x int) {}\n"),
		"through a closure": asserting("\tfail := func() { t.Fatal(\"x\") }\n\tif x != 1 {\n\t\tfail()\n\t}\n"),
		"through a closure stored in a field": asserting(
			"\tvar holder struct{ hook func() }\n\tholder.hook = func() { t.Error(\"x\") }\n\tholder.hook()\n"),
		"through a closure handed to a call": asserting("\tfail := func(int) { t.Error(\"x\") }\n\teach(x, fail)\n",
			"\nfunc each(n int, visit func(int)) { visit(n) }\n"),
		"through a closure set in a composite literal": asserting(
			"\tfail := func() { t.Error(\"x\") }\n\thooks := struct{ on func() }{on: fail}\n\t_ = hooks\n"),
		"through a closure listed in a slice literal": asserting(
			"\tfail := func() { t.Error(\"x\") }\n\thooks := []func(){fail}\n\t_ = hooks\n"),
		"through a chain": asserting("\touter(t, x)\n",
			"\nfunc outer(t *testing.T, x int) { inner(t, x) }\n",
			"\nfunc inner(t *testing.T, x int) {\n\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n}\n"),
		"with t named otherwise": header + "func TestX(tt *testing.T) {\n\t// Act\n\tx := 1\n\n\t// Assert\n" +
			"\tif x != 1 {\n\t\ttt.Error(\"x\")\n\t}\n}\n",
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := check(t, source)

			// Assert
			if len(got) != 0 {
				t.Errorf("violations = %v, want none", got)
			}
		})
	}
}

func TestAnAssertThatReachesNoFailureIsReported(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		line   int
	}{
		"t.Log":      {source: asserting("\tif x != 1 {\n\t\tt.Log(\"x\")\n\t}\n"), line: assertMarkerLine},
		"t.Skip":     {source: asserting("\tif x != 1 {\n\t\tt.Skip(\"x\")\n\t}\n"), line: assertMarkerLine},
		"t.Helper":   {source: asserting("\tt.Helper()\n\t_ = x\n"), line: assertMarkerLine},
		"panic":      {source: asserting("\tif x != 1 {\n\t\tpanic(\"x\")\n\t}\n"), line: assertMarkerLine},
		"fmt.Errorf": {source: asserting("\t_ = fmt.Errorf(\"x = %d\", x)\n"), line: assertMarkerLine},
		"a helper that never fails": {
			source: asserting("\tquiet(t, x)\n", "\nfunc quiet(t *testing.T, x int) { t.Log(x) }\n"),
			line:   assertMarkerLine,
		},
		"mutual recursion": {
			source: asserting("\tping(t, x)\n",
				"\nfunc ping(t *testing.T, n int) {\n\tif n > 0 {\n\t\tpong(t, n-1)\n\t}\n}\n",
				"\nfunc pong(t *testing.T, n int) {\n\tif n > 0 {\n\t\tping(t, n-1)\n\t}\n}\n"),
			line: assertMarkerLine,
		},
		"a method that does not assert": {
			source: asserting("\tw := world{}\n\t_ = w.asked(x)\n", world),
			line:   assertMarkerLine,
		},
		"an asserting method not given t": {
			source: asserting("\tw := world{}\n\tw.expect(nil, x)\n", world),
			line:   assertMarkerLine,
		},
		"an asserting helper not given t": {source: asserting("\texpect(nil, x)\n", expectHelper), line: assertMarkerLine},
		"an asserting method never called": {
			source: asserting("\tw := world{}\n\tverify := w.expect\n\t_ = verify\n", world),
			line:   assertMarkerLine,
		},
		"a closure that calls itself": {
			source: asserting("\tvar loop func()\n\tloop = func() { loop() }\n\tloop()\n"),
			line:   assertMarkerLine,
		},
		"a closure that does not fail": {
			source: asserting("\tnote := func() { t.Log(\"x\") }\n\tnote()\n"),
			line:   assertMarkerLine,
		},
		"a closure stored and never called": {
			source: asserting("\tverify := func() { t.Error(\"x\") }\n\t_ = verify\n"),
			line:   assertMarkerLine,
		},
		"a closure stored by index": {
			source: asserting("\thooks := make([]func(), 1)\n\thooks[0] = func() { t.Error(\"x\") }\n\t_ = hooks\n"),
			line:   assertMarkerLine,
		},
		"a closure stored in a field and never called": {
			source: asserting("\tvar holder struct{ hook func() }\n\tholder.hook = func() { t.Error(\"x\") }\n\t_ = holder\n"),
			line:   assertMarkerLine,
		},
		"an imported function named like a helper": {
			source: "package fixture_test\n\nimport (\n\tcheck \"example.com/tools\"\n\n\t\"testing\"\n)\n\n" +
				"func TestX(t *testing.T) {\n\t// Act\n\tx := 1\n\n\t// Assert\n\tcheck.expect(t, x)\n}\n" + world,
			line: 13,
		},
		"an unnamed import named like a helper": {
			source: "package fixture_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/check\"\n)\n\n" +
				"func TestX(t *testing.T) {\n\t// Act\n\tx := 1\n\n\t// Assert\n\tcheck.expect(t, x)\n}\n" + world,
			line: 13,
		},
		"a failure only in the Arrange": {
			source: test(`	// Arrange
	x, err := 1, error(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	y := x

	// Assert
	_ = y
`),
			line: 15,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := check(t, tt.source)

			// Assert
			want := []found{at(tt.line, testshape.AssertWithoutFailure)}
			if !slices.Equal(got, want) {
				t.Errorf("violations = %v, want %v", got, want)
			}
		})
	}
}

func TestHelpersResolveAcrossThePackagesFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	usesHelper := asserting("\texpect(t, x)\n")
	definesHelper := header + "func TestY(t *testing.T) {\n\t_ = 1\n}\n" + expectHelper

	// Act
	got := check(t, usesHelper, definesHelper)

	// Assert
	want := []found{{file: "fixture1_test.go", line: 5, rule: testshape.MissingMarkers}}
	if !slices.Equal(got, want) {
		t.Errorf("violations = %v, want %v", got, want)
	}
}
