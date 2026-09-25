// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

// fuzz is a fixture file holding one FuzzX with body, and any helpers after it.
// The body's first line is line 6.
func fuzz(body string, helpers ...string) string {
	return header + "func FuzzX(f *testing.F) {\n" + body + "}\n" + strings.Join(helpers, "")
}

func TestSubtestsCarryTheMarkers(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"a table with markers inside each t.Run": {source: test(`	t.Parallel()

	cases := map[string]int{"one": 1}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := want

			// Assert
			if got != want {
				t.Error("got")
			}
		})
	}
`)},
		"a guard before the loop, and setup inside it": {source: test(`	server, err := start()
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []int{1} {
		got := server + want
		t.Run("case", func(t *testing.T) {
			// Act
			sum := got

			// Assert
			if sum != got {
				t.Error("sum")
			}
		})
	}
`, "\nfunc start() (int, error) { return 1, nil }\n")},
		"subtests nested three deep": {source: test(`	t.Run("outer", func(t *testing.T) {
		t.Run("middle", func(t *testing.T) {
			t.Run("inner", func(t *testing.T) {
				// Act & Assert
				if x := 1; x != 1 {
					t.Error("x")
				}
			})
		})
	})
`)},
		"Run on something that is not a test": {source: test(`	// Arrange
	seams := runner{}

	// Act
	got := seams.Run("pre-commit")

	// Assert
	if !got {
		t.Error("run")
	}
`, "\ntype runner struct{}\n\nfunc (runner) Run(string) bool { return true }\n")},
		"a marker in the outer body": {
			source: test(`	// Arrange
	cases := []int{1}

	for _, want := range cases {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}
`),
			want: []found{at(6, testshape.TableMarker)},
		},
		"a malformed marker in the outer body": {
			source: test(`	// arrange
	cases := []int{1}

	for _, want := range cases {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}
`),
			want: []found{at(6, testshape.TableMarker)},
		},
		"a marker in a middle subtest": {
			source: test(`	t.Run("outer", func(t *testing.T) {
		t.Run("middle", func(t *testing.T) {
			// Arrange
			x := 1

			t.Run("inner", func(t *testing.T) {
				// Act & Assert
				if y := x; y != 1 {
					t.Error("y")
				}
			})
		})
	})
`),
			want: []found{at(8, testshape.TableMarker)},
		},
		"a call that cannot fail after the subtests": {source: test(`	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})

		use(want)
	}
`, "\nfunc use(int) {}\n")},
		"an assertion after the loop": {
			source: test(`	total := 0

	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})

		total += want
	}

	if total != 1 {
		t.Error("total")
	}
`),
			want: []found{at(20, testshape.TableAssertion)},
		},
		"an assertion in the loop after t.Run": {
			source: test(`	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})

		if want != 1 {
			t.Error("want")
		}
	}
`),
			want: []found{at(15, testshape.TableAssertion)},
		},
		"an assertion after the loop through the method its receiver's type declares": {
			source: test(`	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}

	w := world{}
	w.check(t, 3)
`, world, checkers),
			want: []found{at(16, testshape.TableAssertion)},
		},
		"a call after the loop to a method its receiver's type declares that does not assert": {
			source: test(`	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}

	q := quiet{}
	q.check(t, 3)
`, world, checkers),
		},
		"a call after the loop to a method the check cannot pick, on a field": {
			source: test(`	var holder struct{ w world }

	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}

	holder.w.check(t, 3)
`, world, checkers),
			want: []found{at(17, testshape.AmbiguousHelper)},
		},
		"a call after the loop to a method the check cannot pick, on an interface": {
			source: test(`	var q checker = world{}

	for _, want := range []int{1} {
		t.Run("case", func(t *testing.T) {
			// Act & Assert
			if got := want; got != want {
				t.Error("got")
			}
		})
	}

	q.check(t, 1)
`, world, checkers, "\ntype checker interface{ check(*testing.T, int) }\n"),
			want: []found{at(17, testshape.AmbiguousHelper)},
		},
		"a subtest run from a function value": {
			source: test(`	check := func(t *testing.T) {
		t.Helper()
	}

	t.Run("case", check)
`),
			want: []found{at(10, testshape.SubtestLiteral)},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := check(t, tt.source)

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("violations = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFuzzTargetsCarryTheMarkers(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"seeds, and markers in the target": {source: fuzz(`	f.Add("seed")

	f.Fuzz(func(t *testing.T, s string) {
		// Act
		got := s

		// Assert
		if got != s {
			t.Error("got")
		}
	})
`)},
		"a marker outside the target": {
			source: fuzz(`	// Arrange
	f.Add("seed")

	f.Fuzz(func(t *testing.T, s string) {
		// Act & Assert
		if got := s; got != s {
			t.Error("got")
		}
	})
`),
			want: []found{at(6, testshape.TableMarker)},
		},
		"a target that is not a function literal": {
			source: fuzz("\tf.Fuzz(target)\n", "\nfunc target(t *testing.T, s string) {}\n"),
			want:   []found{at(6, testshape.SubtestLiteral)},
		},
		"a target that fails through f": {
			source: fuzz(`	f.Fuzz(func(t *testing.T, s string) {
		// Act & Assert
		if s == "" {
			f.Fatal("empty")
		}
	})
`),
			want: []found{at(7, testshape.AssertWithoutFailure)},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := check(t, tt.source)

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("violations = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOnlyTestsAndFuzzTargetsAreChecked(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"TestMain, benchmarks, examples, helpers and near misses": {source: header + `func TestMain(m *testing.M) { m.Run() }

func Testable(t *testing.T) {}

func BenchmarkX(b *testing.B) {}

func ExampleX() {}

func helper(t *testing.T) {}

type suite struct{}

func (suite) TestX(t *testing.T) {}

func TestWrongParameter(n int) {}

func TestTwoParameters(t *testing.T, n int) {}

func TestTwoNames(t, u *testing.T) {}
`},
		"a test importing testing under another name": {
			source: "package fixture_test\n\nimport tst \"testing\"\n\nfunc TestX(t *tst.T) {\n\t_ = 1\n}\n",
			want:   []found{at(5, testshape.MissingMarkers)},
		},
		// A test's parameter is recognized through the file's own import of testing.
		"a file that does not import testing": {
			source: "package fixture_test\n\nfunc TestX(t *testing.T) {\n\t_ = 1\n}\n",
		},
		"a test with an underscore after Test": {
			source: header + "func Test_x(t *testing.T) {\n\t_ = 1\n}\n",
			want:   []found{at(5, testshape.MissingMarkers)},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := check(t, tt.source)

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("violations = %v, want %v", got, tt.want)
			}
		})
	}
}
