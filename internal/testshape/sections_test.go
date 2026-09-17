// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

func TestSectionsFollowArrangeActAssert(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"arrange, act and assert below t.Parallel": test(`	t.Parallel()

	// Arrange
	x := 1

	// Act
	y := x + 1

	// Assert
	if y != 2 {
		t.Errorf("y = %d", y)
	}
`),
		"act and assert with nothing to arrange": test(`	// Act
	x := 1

	// Assert
	if x != 1 {
		t.Error("x")
	}
`),
		"an act folded into its assertion": test(`	// Act & Assert
	if x := 1; x != 1 {
		t.Error("x")
	}
`),
		"arrange, then an act folded into its assertion": test(`	// Arrange
	x := 1

	// Act & Assert
	if y := x + 1; y != 2 {
		t.Error("y")
	}
`),
		"a labeled flow": test(`	// Arrange
	x := 0

	// Act: add one
	x++

	// Assert: it is one
	if x != 1 {
		t.Error("x")
	}

	// Act & Assert: adding again makes two
	if x++; x != 2 {
		t.Error("x")
	}
`),
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

func TestSectionsOutOfOrderAreReported(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"no markers at all": {
			source: test(`	x := 1
	_ = x
`),
			want: []found{at(5, testshape.MissingMarkers)},
		},
		"code before the first marker": {
			source: test(`	x := 1

	// Act
	y := x

	// Assert
	if y != 1 {
		t.Error("y")
	}
`),
			want: []found{at(6, testshape.CodeBeforeMarker)},
		},
		"an Assert before any Act": {
			source: test(`	// Assert
	if true {
		t.Error("x")
	}
`),
			want: []found{at(6, testshape.MarkerOrder)},
		},
		"a second Arrange": {
			source: test(`	// Arrange
	x := 1

	// Arrange
	y := x

	// Act & Assert
	if y != 1 {
		t.Error("y")
	}
`),
			want: []found{at(9, testshape.MarkerOrder)},
		},
		"an Arrange inside a cycle": {
			source: test(`	// Act
	x := 1

	// Arrange
	y := x

	// Assert
	if y != 1 {
		t.Error("y")
	}
`),
			want: []found{at(9, testshape.MarkerOrder)},
		},
		"two Acts in a row": {
			source: test(`	// Act
	x := 1

	// Act
	y := x

	// Assert
	if y != 1 {
		t.Error("y")
	}
`),
			want: []found{at(9, testshape.MarkerOrder)},
		},
		"an Act & Assert where the Assert belongs": {
			source: test(`	// Act
	x := 1

	// Act & Assert
	if x != 1 {
		t.Error("x")
	}
`),
			want: []found{at(9, testshape.MarkerOrder)},
		},
		"an Arrange after an Assert": {
			source: test(`	// Act: first
	x := 1

	// Assert: first
	if x != 1 {
		t.Error("x")
	}

	// Arrange
	y := 2

	// Act: second
	y++

	// Assert: second
	if y != 3 {
		t.Error("y")
	}
`),
			want: []found{at(14, testshape.MarkerOrder)},
		},
		"two Asserts in a row": {
			source: test(`	// Act
	x := 1

	// Assert
	if x != 1 {
		t.Error("x")
	}

	// Assert
	if x == 2 {
		t.Error("x")
	}
`),
			want: []found{at(14, testshape.MarkerOrder)},
		},
		"an Arrange and nothing more": {
			source: test(`	// Arrange
	x := 1
	_ = x
`),
			want: []found{at(6, testshape.MarkerOrder)},
		},
		"an Act without its Assert": {
			source: test(`	// Arrange
	x := 1

	// Act
	_ = x
`),
			want: []found{at(9, testshape.MarkerOrder)},
		},
		"two tests in one file, reported in order": {
			source: header + "func TestA(t *testing.T) {\n\t_ = 1\n}\n\nfunc TestB(t *testing.T) {\n\t_ = 2\n}\n",
			want:   []found{at(5, testshape.MissingMarkers), at(9, testshape.MissingMarkers)},
		},
		"an order error hides the label and failure problems after it": {
			source: test(`	// Assert: nothing checked
	x := 1

	// Act
	_ = x
`),
			want: []found{at(6, testshape.MarkerOrder)},
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

func TestOnlyTParallelMayComeBeforeTheMarkers(t *testing.T) {
	t.Parallel()

	// Each case is the statement before the markers of an otherwise sound test.
	cases := map[string]string{
		"an assignment":                  "x := 1",
		"a receive":                      "<-done",
		"a call on t with arguments":     "t.Log(\"starting\")",
		"another method on t":            "t.Helper()",
		"a plain function call":          "setUp()",
		"Parallel on something else":     "w.Parallel()",
		"Parallel on a call's result":    "newWorld().Parallel()",
		"Parallel with a stray argument": "t.Parallel(1)",
	}

	for name, statement := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			source := test("\t" + statement + "\n\n\t// Act & Assert\n\tif x := 1; x != 1 {\n\t\tt.Error(\"x\")\n\t}\n")

			// Act
			got := check(t, source)

			// Assert
			want := []found{at(6, testshape.CodeBeforeMarker)}
			if !slices.Equal(got, want) {
				t.Errorf("violations = %v, want %v", got, want)
			}
		})
	}
}

func TestEmptySectionsAndLabelsAreReported(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"an empty section in the middle": {
			source: test(`	// Arrange

	// Act
	x := 1

	// Assert
	if x != 1 {
		t.Error("x")
	}
`),
			want: []found{at(6, testshape.EmptySection)},
		},
		"an empty section at the end": {
			source: test(`	// Act
	x := 1
	_ = x

	// Assert
`),
			want: []found{at(10, testshape.EmptySection)},
		},
		"a flow missing an Act label": {
			source: test(`	// Act
	x := 1

	// Assert: it is one
	if x != 1 {
		t.Error("x")
	}

	// Act: again
	x++

	// Assert: it is two
	if x != 2 {
		t.Error("x")
	}
`),
			want: []found{at(6, testshape.LabelRequired)},
		},
		"a flow missing an Assert label": {
			source: test(`	// Act: one
	x := 1

	// Assert
	if x != 1 {
		t.Error("x")
	}

	// Act: again
	x++

	// Assert: it is two
	if x != 2 {
		t.Error("x")
	}
`),
			want: []found{at(9, testshape.LabelRequired)},
		},
		"a flow missing an Act & Assert label": {
			source: test(`	// Act: one
	x := 1

	// Assert: it is one
	if x != 1 {
		t.Error("x")
	}

	// Act & Assert
	if x++; x != 2 {
		t.Error("x")
	}
`),
			want: []found{at(14, testshape.LabelRequired)},
		},
		"a single cycle with a label": {
			source: test(`	// Act: add
	x := 1

	// Assert
	if x != 1 {
		t.Error("x")
	}
`),
			want: []found{at(6, testshape.LabelUnexpected)},
		},
		"a labeled Arrange": {
			source: test(`	// Arrange: a counter
	x := 0

	// Act & Assert
	if x != 0 {
		t.Error("x")
	}
`),
			want: []found{at(6, testshape.LabelUnexpected)},
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
