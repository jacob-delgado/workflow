// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// wordyWorld is the world with an issue whose description runs well past any
// pane, ending THE END.
func wordyWorld() *world {
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"

	return wordy
}

// pagesDown is the scroll-down key pressed count times.
func pagesDown(count int) []string {
	return slices.Repeat([]string{"pgdown"}, count)
}

// changedFiles is count files named prefix0.go onward, each modified and none
// staged, so a list of them outgrows the detail pane.
func changedFiles(prefix string, count int) []gitrepo.Change {
	changes := make([]gitrepo.Change, count)
	for index := range changes {
		changes[index] = gitrepo.Change{Path: prefix + strconv.Itoa(index) + ".go", Staged: ' ', Unstaged: 'M'}
	}

	return changes
}

// selectingRow is the keys that focus a pane by its number and move its
// selection down to row index.
func selectingRow(paneKey string, index int) []string {
	return append([]string{paneKey}, slices.Repeat([]string{"j"}, index)...)
}

// Over-scrolling must not strand the offset past the end: one scroll-up has to
// move the view, not spend itself undoing scrolls the content never had room
// for.
func TestOverScrollingStillMovesOnTheWayBack(t *testing.T) {
	t.Parallel()

	// Arrange
	overScrolled := typing(t, wordyWorld().live(t, 120, 30), pagesDown(12)...)

	// Act
	back := typing(t, overScrolled, "pgup")

	// Assert
	requireScreen(t, overScrolled.View().Content, "THE END")
	refuseScreen(t, back.View().Content, "THE END")
}

// A taller terminal leaves an offset that was the end past the new end. The
// first scroll-up must move the view from where it is drawn, not spend itself
// undoing lines the taller pane no longer needs to hide.
func TestAScrollLeftPastTheEndByATallerTerminalMovesOnTheFirstKey(t *testing.T) {
	t.Parallel()

	// Arrange
	atTheEnd := typing(t, wordyWorld().live(t, 120, 30), pagesDown(12)...)
	taller := sized(t, atTheEnd, 120, 60)

	// Act
	back := typing(t, taller, "pgup")

	// Assert
	requireScreen(t, atTheEnd.View().Content, "THE END")
	refuseScreen(t, taller.View().Content, detailTop)
	requireScreen(t, back.View().Content, detailTop)
}

// A pane first seen after another was scrolled deep starts at its own top, not
// at the other pane's depth.
func TestAnotherPaneStartsAtItsOwnTop(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := wordyWorld()
	wordy.changes = changedFiles("file", 60)
	deep := typing(t, wordy.live(t, 120, 30), pagesDown(6)...)

	// Act
	view := typing(t, deep, "3").View().Content

	// Assert
	requireScreen(t, view, "▸ ○ modified   file0.go")
}

func TestMovingToAnotherIssueStartsItsDetailAtTheTop(t *testing.T) {
	t.Parallel()

	// Arrange
	deep := typing(t, wordyWorld().live(t, 120, 30), pagesDown(6)...)

	// Act
	view := typing(t, deep, "j").View().Content

	// Assert
	requireScreen(t, view, "│ "+secondIssue+" Add retries")
}

// Reading the shown issue again — after assigning it, here — refreshes what it
// says without moving the reader back to its top.
func TestReloadingTheShownIssueKeepsItsScroll(t *testing.T) {
	t.Parallel()

	// Arrange
	deep := typing(t, wordyWorld().live(t, 120, 30), pagesDown(12)...)

	// Act
	view := typing(t, deep, append(append([]string{"a"}, letters("fred")...), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "assigned "+issueKey+" to fred", "THE END")
	refuseScreen(t, view, detailTop)
}

func TestRefreshingTheBranchKeepsItsScroll(t *testing.T) {
	t.Parallel()

	// Arrange
	// A terminal this short, and too narrow for the rail, shows the Branch pane's
	// detail alone and not all of it, so one scroll puts its name out of sight.
	scrolled := typing(t, newWorld().live(t, 70, 8), "2", "J")
	if strings.Contains(plain(scrolled.View().Content), featureName) {
		t.Fatalf("the Branch pane did not scroll:\n%s", plain(scrolled.View().Content))
	}

	// Act
	view := typing(t, scrolled, "r").View().Content

	// Assert
	requireScreen(t, view, "commits   1 not on the base")
	refuseScreen(t, view, featureName)
}

func TestARefreshKeepsTheSelectedReviewInSight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.reviews = manyReviews(20)
	scrolled := typing(t, repo.live(t, 120, 16), selectingRow("6", 19)...)

	// Act
	view := typing(t, scrolled, "r").View().Content

	// Assert
	requireScreen(t, view, "▸ · #1 queued change (1)")
}

// A reload that shrinks the tree while the Commits pane is out of sight must
// still leave a click on it landing on the row it appears to.
func TestClickingAfterTheTreeShrankWhileAwayStagesTheVisibleFile(t *testing.T) {
	t.Parallel()

	// Arrange
	shrinking := newWorld()
	shrinking.changes = changedFiles("old", 20)
	away := typing(t, shrinking.live(t, 120, 20), append(selectingRow("3", 19), "2")...)

	shrinking.changes = changedFiles("new", 3)
	back := typing(t, away, "r", "3")

	// Act
	// Row 3 of the detail is the second file, new1.go; stage it after the click.
	typing(t, click(t, back, 60, 3), keySpace)

	// Assert
	if got := shrinking.asked("stage new1.go"); len(got) != 1 {
		t.Errorf("stage calls = %v, want the clicked visible file new1.go", shrinking.asked("stage"))
	}
}
