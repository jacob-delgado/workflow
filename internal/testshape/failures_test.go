// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"slices"
	"strings"
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

// checkers declares three methods on world and on quiet: check, which only
// world's asserts, note, which neither's does, and must, which both do.
const checkers = `
type quiet struct{}

func (world) check(t *testing.T, x int) {
	if x != 1 {
		t.Error("x")
	}
}

func (quiet) check(t *testing.T, x int) { t.Log(x) }

func (world) note(t *testing.T, x int) { t.Log(x) }

func (quiet) note(t *testing.T, x int) { t.Log(x) }

func (world) must(t *testing.T, x int) {
	if x != 1 {
		t.Error("x")
	}
}

func (quiet) must(t *testing.T, x int) {
	if x != 1 {
		t.Error("x")
	}
}
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
		"through the method its receiver's type declares": asserting("\tw := world{}\n\tw.check(t, x)\n", world, checkers),
		"through a method on a pointer":                   asserting("\tw := &world{}\n\tw.check(t, x)\n", world, checkers),
		"through a method on new":                         asserting("\tw := new(world)\n\tw.check(t, x)\n", world, checkers),
		"through a method on a variable of a declared type": asserting("\tvar w world\n\tw.check(t, x)\n",
			world, checkers),
		"through a method on a variable declared with a value": asserting("\tvar w = world{}\n\tw.check(t, x)\n",
			world, checkers),
		"through a method on a typed parameter": asserting("\tinspect(t, &world{}, x)\n", world, checkers,
			"\nfunc inspect(t *testing.T, w *world, x int) { w.check(t, x) }\n"),
		"through a method on its own receiver": asserting("\tworld{}.verify(t, x)\n", world, checkers,
			"\nfunc (w world) verify(t *testing.T, x int) { w.check(t, x) }\n"),
		"through a method on a call's result of its type": asserting("\tnewWorld().check(t, x)\n", world, checkers,
			"\nfunc newWorld() world { return world{} }\n"),
		"through a method only one type of its name declares": asserting(
			"\tvar holder struct{ w world }\n\tholder.w.expect(t, x)\n", world, checkers),
		"through a method every type of its name asserts": asserting(
			"\tvar holder struct{ q quiet }\n\tholder.q.must(t, x)\n", world, checkers),
		"with a method the check cannot pick beside a failure": asserting(
			"\tvar holder struct{ q quiet }\n\tholder.q.check(t, x)\n\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n",
			world, checkers),
		"through a closure stored in a field": asserting(
			"\tvar holder struct{ hook func() }\n\tholder.hook = func() { t.Error(\"x\") }\n\tholder.hook()\n"),
		"through a closure handed to a call": asserting("\tfail := func(int) { t.Error(\"x\") }\n\teach(x, fail)\n",
			"\nfunc each(n int, visit func(int)) { visit(n) }\n"),
		"through a closure set in a composite literal": asserting(
			"\tfail := func() { t.Error(\"x\") }\n\thooks := struct{ on func() }{on: fail}\n\t_ = hooks\n"),
		"through a closure listed in a slice literal": asserting(
			"\tfail := func() { t.Error(\"x\") }\n\thooks := []func(){fail}\n\t_ = hooks\n"),
		"through a closure declared around the subtest": test(`	fail := func() { t.Error("x") }

	t.Run("x", func(t *testing.T) {
		// Act
		x := 1

		// Assert
		if x != 1 {
			fail()
		}
	})
`),
		"through a receiver a sibling subtest declares otherwise": test(`	t.Run("world", func(t *testing.T) {
		// Arrange
		w := world{}

		// Act
		x := 1

		// Assert
		w.check(t, x)
	})

	t.Run("quiet", func(t *testing.T) {
		// Arrange
		w := quiet{}

		// Act
		x := 1

		// Assert
		w.note(t, x)
		if x != 1 {
			t.Error("x")
		}
	})
`,
			world, checkers),
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
		"the method its receiver's type declares, that does not assert": {
			source: asserting("\tq := quiet{}\n\tq.check(t, x)\n", world, checkers),
			line:   assertMarkerLine,
		},
		"a method no type of its name asserts": {
			source: asserting("\tvar holder struct{ q quiet }\n\tholder.q.note(t, x)\n", world, checkers),
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
		"a closure only a sibling subtest declares asserting": {
			source: test(`	t.Run("quiet", func(t *testing.T) {
		// Arrange
		fail := func() {}

		// Act
		x := 1

		// Assert
		if x != 1 {
			fail()
		}
	})

	t.Run("loud", func(t *testing.T) {
		// Arrange
		fail := func() { t.Error("x") }

		// Act
		x := 1

		// Assert
		if x != 1 {
			fail()
		}
	})
`),
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

func TestAMethodTheCheckCannotPickIsReportedAtItsCall(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"on a field":                     "\tvar holder struct{ q quiet }\n\tholder.q.check(t, x)\n",
		"from a call of two results":     "\tq, err := two()\n\t_ = err\n\tq.check(t, x)\n",
		"from a var of two results":      "\tvar q, err = two()\n\t_ = err\n\tq.check(t, x)\n",
		"of a generic type":              "\tq := box[int]{}\n\tq.check(t, x)\n",
		"from new without a type":        "\tq := new()\n\tq.check(t, x)\n",
		"from a function with no result": "\tq := nothing()\n\tq.check(t, x)\n",
		"from another package's call":    "\tq := tools.New()\n\tq.check(t, x)\n",
		"of an interface type":           "\tvar q checker = quiet{}\n\tq.check(t, x)\n",
		"promoted from an embedded type": "\tq := fixture{}\n\tq.check(t, x)\n",
		"declared again with another type": "\tif x == 1 {\n\t\tq := world{}\n\t\t_ = q\n\t}\n" +
			"\tq := quiet{}\n\tq.check(t, x)\n",
		"a range variable over an earlier one": "\tq := world{}\n\t_ = q\n" +
			"\tfor _, q := range []quiet{{}} { q.check(t, x) }\n",
		"through a helper that calls one": "\tinspect(t, keeper{}, x)\n",
	}
	declarations := "\ntype checker interface{ check(*testing.T, int) }\n" +
		"\ntype fixture struct{ world }\n" +
		"\nfunc two() (quiet, error) { return quiet{}, nil }\n" +
		"\ntype box[T any] struct{}\n\nfunc (box[T]) check(t *testing.T, x int) { t.Log(x) }\n" +
		"\nfunc nothing() {}\n" +
		"\ntype keeper struct{ q quiet }\n\nfunc inspect(t *testing.T, k keeper, x int) { k.q.check(t, x) }\n"

	for name, assert := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			source := asserting(assert, world, checkers, declarations)
			call := assertMarkerLine + strings.Count(assert, "\n")

			// Act
			got := check(t, source)

			// Assert
			want := []found{at(call, testshape.AmbiguousHelper)}
			if !slices.Equal(got, want) {
				t.Errorf("violations = %v, want %v", got, want)
			}
		})
	}
}
