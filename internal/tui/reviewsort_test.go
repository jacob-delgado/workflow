// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

func TestSListsTheReviewsNewestFirst(t *testing.T) {
	t.Parallel()

	// Act
	view := plain(typing(t, withReviews().live(t, 120, 40), "6", "s").View().Content)

	// Assert
	requireScreen(t, view, "newest first")

	if newer, older := strings.Index(view, "#7 "), strings.Index(view, "#12 "); newer > older {
		t.Errorf("the newer request is listed below the older:\n%s", view)
	}
}

func TestSortingByRepositoryHeadsTheRequestsOfEachRepository(t *testing.T) {
	t.Parallel()

	// Act
	view := plain(typing(t, withReviews().live(t, 120, 40), "6", "s", "s").View().Content)

	// Assert
	requireScreen(t, view, "by repository")

	other, older := headingLine(view, "example/other"), lineWith(view, "#12 ")
	repo, newer := headingLine(view, "example/repo"), lineWith(view, "#7 ")

	if other < 0 || repo < 0 || other > older || older > repo || repo > newer {
		t.Errorf("want example/other's heading, #12, example/repo's heading, #7, in that order:\n%s", view)
	}
}

func TestMovingThroughReposPassesOverTheirHeadings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "s", "s", downAction, "o")

	// Assert
	if opened := repo.asked("browse"); len(opened) != 1 || !strings.Contains(opened[0], reviewNewerURL) {
		t.Errorf("opened %q, want the second request, #7, past the heading between them", opened)
	}
}

// lineWith is the number of the first line holding text, or -1.
func lineWith(view, text string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return index
		}
	}

	return -1
}

// headingLine is the number of the line naming repository on its own, as a
// heading does and a request's row, which carries its number, does not; or -1.
func headingLine(view, repository string) int {
	for index, line := range strings.Split(view, "\n") {
		if strings.Contains(line, repository) && !strings.Contains(line, "#") {
			return index
		}
	}

	return -1
}
