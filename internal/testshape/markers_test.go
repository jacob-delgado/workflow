// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

func TestMarkersAreReadExactly(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"trailing whitespace after a marker": test(
			"\t// Act  \n\tx := 1\n\n\t// Assert\t\n\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n"),
		"prose that starts with a marker's word": test(`	// Act
	x := 1

	// Assert
	// Assert nothing leaks: acting on x is enough.
	if x != 1 {
		t.Error("x")
	}
`),
		"a marker inside a string literal": test(`	// Act
	x := "// Assert"

	// Assert
	if x == "" {
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

func TestMalformedMarkersAreReported(t *testing.T) {
	t.Parallel()

	// Each writes the Assert of an otherwise sound test wrongly, on line 9. A
	// malformed marker is reported alone: the rest of the body is checked once
	// it is fixed.
	cases := map[string]string{
		"lowercase":            "// assert",
		"no space after //":    "//Assert",
		"two spaces after //":  "//  Assert",
		"a colon and no label": "// Assert:",
		"shouting":             "// ACT & ASSERT",
		"and for &":            "// Act and Assert",
		"a block comment":      "/* Assert */",
	}

	for name, marker := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			source := test("\t// Act\n\tx := 1\n\n\t" + marker + "\n\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n")

			// Act
			got := check(t, source)

			// Assert
			want := []found{at(9, testshape.MalformedMarker)}
			if !slices.Equal(got, want) {
				t.Errorf("violations = %v, want %v", got, want)
			}
		})
	}
}

func TestMarkersBelongBetweenTopLevelStatements(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source string
		want   []found
	}{
		"a marker trailing a statement": {
			source: test(`	// Act
	x := 1 // Assert
	if x != 1 {
		t.Error("x")
	}
`),
			want: []found{at(7, testshape.MarkerPlacement)},
		},
		"a marker after an opening brace": {
			source: test(`	// Act
	x := 1

	if x != 1 { // Assert
		t.Error("x")
	}
`),
			want: []found{at(9, testshape.MarkerPlacement)},
		},
		"a marker on the line that opens the body": {
			source: header + "func TestX(t *testing.T) { // Act\n\tx := 1\n\n" +
				"\t// Assert\n\tif x != 1 {\n\t\tt.Error(\"x\")\n\t}\n}\n",
			want: []found{at(5, testshape.MarkerPlacement)},
		},
		"a marker inside an if": {
			source: test(`	// Act
	x := 1

	if x != 1 {
		// Assert
		t.Error("x")
	}
`),
			want: []found{at(10, testshape.MarkerPlacement)},
		},
		"a marker inside a for": {
			source: test(`	// Act
	x := 1

	for range x {
		// Assert
		t.Error("x")
	}
`),
			want: []found{at(10, testshape.MarkerPlacement)},
		},
		"a marker inside a select": {
			source: test(`	// Act
	done := make(chan struct{})

	select {
	case <-done:
		// Assert
		t.Error("done")
	default:
	}
`),
			want: []found{at(11, testshape.MarkerPlacement)},
		},
		"a marker inside a closure that is not a subtest": {
			source: test(`	// Act
	check := func() {
		// Assert
		t.Error("x")
	}

	// Assert
	check()
`),
			want: []found{at(8, testshape.MarkerPlacement)},
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
