// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package workdirs_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// entries are directories by name, none of them a repository.
func entries(names ...string) []workdirs.Entry {
	listed := make([]workdirs.Entry, 0, len(names))
	for _, name := range names {
		listed = append(listed, workdirs.Entry{Name: name, Repository: false})
	}

	return listed
}

// matchedNames are the names of what matched, in order.
func matchedNames(matches []workdirs.Match) []string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.Entry.Name)
	}

	return names
}

func TestNamesStartingWithWhatIsTypedRankBeforeThoseHoldingItsLetters(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := entries("lib", "two-words", "workflow", "notes", "Workday-notes")

	// Act
	got := workdirs.Matching(listed, "wo")

	// Assert
	want := []string{"workflow", "Workday-notes", "two-words"}
	if names := matchedNames(got); !slices.Equal(names, want) {
		t.Fatalf("matched %q, want %q", names, want)
	}

	if !got[0].Prefix || !got[1].Prefix || got[2].Prefix {
		t.Errorf("matches %+v; want the first two to start with wo, the last not", got)
	}
}

func TestAMatchSaysWhichLettersMatched(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := entries("café-two")

	// Act
	got := workdirs.Matching(listed, "ÉW")

	// Assert
	// Each letter's place is counted in letters, so é is the fourth.
	if len(got) != 1 || !slices.Equal(got[0].Letters, []int{3, 6}) {
		t.Errorf("matches %+v, want é and w at 3 and 6", got)
	}
}

func TestEveryNameMatchesNothingTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := entries("web", "zoo")

	// Act
	got := workdirs.Matching(listed, "")

	// Assert
	if names := matchedNames(got); !slices.Equal(names, []string{"web", "zoo"}) || !got[0].Prefix || !got[1].Prefix {
		t.Errorf("matches %+v, want both, as given, each starting with nothing", got)
	}
}
