// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package listsearch_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/listsearch"
)

func TestTheZeroSearchIsOffWithNoText(t *testing.T) {
	t.Parallel()

	// Arrange
	var search listsearch.Typed

	// Act
	got := viewOf(search)

	// Assert
	if got != (view{stage: listsearch.StageOff, text: ""}) {
		t.Errorf("the zero Typed is %+v; want off with no text", got)
	}
}

func TestABegunSearchIsTypedAfreshFromAnyStage(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		fromOff: off, fromTyping: typing, fromKept: kept,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			begun := search.Begun()

			// Assert
			if got := viewOf(begun); got != (view{stage: listsearch.StageTyping, text: ""}) {
				t.Errorf("Begun is %+v; want typing with no text", got)
			}
		})
	}
}

func TestTextTypedGoesOnTheEndOfASearchBeingTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	search := typing(t)

	// Act
	typed := search.Typed("é")

	// Assert
	if got := viewOf(typed); got != (view{stage: listsearch.StageTyping, text: "abé"}) {
		t.Errorf("Typed is %+v; want typing abé", got)
	}
}

func TestASearchNotBeingTypedTakesNoText(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		fromOff: off, fromKept: kept,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			typed := search.Typed("x")

			// Assert
			if typed != search {
				t.Errorf("Typed is %+v; want %+v unchanged", viewOf(typed), viewOf(search))
			}
		})
	}
}

func TestTrimmingTakesTheLastCharacterOffASearchBeingTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	search := typing(t).Typed("é")

	// Act
	trimmed := search.Trimmed()

	// Assert
	if got := viewOf(trimmed); got != (view{stage: listsearch.StageTyping, text: "ab"}) {
		t.Errorf("Trimmed is %+v; want typing ab, the whole é taken off", got)
	}
}

func TestTrimmingWithNothingToTakeChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		fromOff: off, "being typed with nothing typed": typingNothing, fromKept: kept,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			trimmed := search.Trimmed()

			// Assert
			if trimmed != search {
				t.Errorf("Trimmed is %+v; want %+v unchanged", viewOf(trimmed), viewOf(search))
			}
		})
	}
}

func TestAKeptSearchStillNarrowsByItsText(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		fromTyping: typing, "kept already": kept,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			got := viewOf(search.Kept())

			// Assert
			if got != (view{stage: listsearch.StageKept, text: "ab"}) {
				t.Errorf("Kept is %+v; want kept ab", got)
			}
		})
	}
}

func TestASearchKeptWithNoTextIsOff(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		"being typed with nothing typed": typingNothing, fromOff: off,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			got := viewOf(search.Kept())

			// Assert
			if got != (view{stage: listsearch.StageOff, text: ""}) {
				t.Errorf("Kept is %+v; want off with no text", got)
			}
		})
	}
}

func TestAClearedSearchIsOffFromAnyStage(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) listsearch.Typed{
		fromOff: off, fromTyping: typing, fromKept: kept,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			search := from(t)

			// Act
			got := viewOf(search.Cleared())

			// Assert
			if got != (view{stage: listsearch.StageOff, text: ""}) {
				t.Errorf("Cleared is %+v; want off with no text", got)
			}
		})
	}
}
