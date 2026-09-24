// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
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

// announcementLead is the first line of the Slack pane's announcement of the
// branch's pull request.
const announcementLead = "jacob opened a pull request:"

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

// The done-when of a scroll kept per pane: leaving a pane and coming back finds
// its detail where it was left.
func TestLeavingAndReturningToAPaneKeepsItsScroll(t *testing.T) {
	t.Parallel()

	// Arrange
	scrolled := typing(t, wordyWorld().live(t, 120, 30), pagesDown(6)...)

	// Act
	view := typing(t, scrolled, keyTab, keyShiftTab).View().Content

	// Assert
	requireScreen(t, view, "THE END")
	refuseScreen(t, view, detailTop)
}

func TestReturningToTheCommitsPaneKeepsTheSelectedFileInSight(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"after reading another pane": {"1"},
		// The Branch pane's refresh reads the work tree again too.
		"after a reload while away": {"2", "r"},
	}

	for name, away := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			crowded := newWorld()
			crowded.changes = changedFiles("file", 60)
			left := typing(t, crowded.live(t, 120, 20), append(selectingRow("3", 40), away...)...)

			// Act
			view := typing(t, left, "3").View().Content

			// Assert
			requireScreen(t, view, "▸ ○ modified   file40.go")
		})
	}
}

// A taller terminal draws a list scrolled to its selection from higher up than
// the offset it was scrolled to, since the whole of it now nearly fits. A click
// must pick the row drawn where it lands, not the row the old offset names.
func TestClickingAReviewAfterATallerTerminalOpensTheRowDrawnThere(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.reviews = manyReviews(20)
	taller := sized(t, typing(t, repo.live(t, 120, 16), selectingRow("6", 19)...), 120, 24)
	top := topReviewNumber(t, taller.View().Content)

	// Act
	typing(t, click(t, taller, 40, 2), "o")

	// Assert
	if got := repo.asked("browse https://example.com/pr/" + strconv.Itoa(top)); len(got) != 1 {
		t.Errorf("browse calls = %v, want the top request drawn, #%d", repo.asked("browse"), top)
	}
}

func TestClickingAFileAfterATallerTerminalStagesTheFileDrawnThere(t *testing.T) {
	t.Parallel()

	// Arrange
	crowded := newWorld()
	crowded.changes = changedFiles("file", 60)
	taller := sized(t, typing(t, crowded.live(t, 120, 20), selectingRow("3", 40)...), 120, 60)
	drawn := regexp.MustCompile(`file\d+\.go`).FindString(strings.Split(plain(taller.View().Content), "\n")[3])

	// Act
	typing(t, click(t, taller, 60, 3), keySpace)

	// Assert
	if got := crowded.asked("stage " + drawn); drawn == "" || len(got) != 1 {
		t.Errorf("stage calls = %v, want the file drawn on the clicked row, %q", crowded.asked("stage"), drawn)
	}
}

// A click on the detail's border lands on no line of the list, even where a
// file is scrolled out of sight just past it, as a click on the rail's does.
func TestClickingTheDetailsBorderPicksNoFile(t *testing.T) {
	t.Parallel()

	cases := map[string]int{"the top border": 1, "the bottom border": 18}

	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			crowded := newWorld()
			crowded.changes = changedFiles("file", 60)
			scrolled := typing(t, crowded.live(t, 120, 20), selectingRow("3", 40)...)

			// Act
			typing(t, click(t, scrolled, 60, row), keySpace)

			// Assert
			if got := crowded.asked("stage "); len(got) != 1 || got[0] != "stage file40.go" {
				t.Errorf("stage calls = %v, want only the file still selected, file40.go", got)
			}
		})
	}
}

func TestClickingTheDetailsBorderPicksNoReview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.reviews = manyReviews(20)
	scrolled := typing(t, repo.live(t, 120, 16), selectingRow("6", 19)...)

	// Act
	typing(t, click(t, scrolled, 40, 1), "o")

	// Assert
	if got := repo.asked("browse "); len(got) != 1 || got[0] != "browse https://example.com/pr/1" {
		t.Errorf("browse calls = %v, want only the request still selected, #1", got)
	}
}

// Another branch checked out while its panes were scrolled starts each of them
// at the top, since its name, its request and the announcement of it are new.
func TestAnotherBranchStartsItsPanesAtTheTop(t *testing.T) {
	t.Parallel()

	const otherBranch, otherTitle = "feat/" + secondIssue + "-add-retries", "feat: add retries"

	cases := map[string]struct {
		pane   string
		reload []string
		top    string
	}{
		"the Branch pane, refreshed in place":     {pane: "2", reload: []string{"r"}, top: otherBranch},
		"the Branch pane, refreshed from Commits": {pane: "2", reload: []string{"3", "r", "2"}, top: otherBranch},
		"the Review pane":                         {pane: "4", reload: []string{"3", "r", "4"}, top: "#43 " + otherTitle},
		"the Slack pane":                          {pane: "5", reload: []string{"3", "r", "5"}, top: announcementLead},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			switching := newWorld()
			scrolled := typing(t, switching.live(t, 70, 8), tt.pane, "J")
			switching.branch.Name, switching.branch.Upstream = otherBranch, "origin/"+otherBranch
			switching.pull.Number, switching.pull.Title = 43, otherTitle

			// Act
			view := typing(t, scrolled, tt.reload...).View().Content

			// Assert
			requireScreen(t, view, tt.top)
		})
	}
}

// A new request found for the same branch is new content too, so the Review
// pane shows it from its top.
func TestANewRequestStartsTheReviewPaneAtTheTop(t *testing.T) {
	t.Parallel()

	// Arrange
	reopened := newWorld()
	scrolled := typing(t, reopened.live(t, 70, 8), "4", "J")
	reopened.pull.Number = 43

	// Act
	view := typing(t, scrolled, "r").View().Content

	// Assert
	requireScreen(t, view, "#43 "+pullTitle)
}

// Each pane scrolls an offset of its own: scrolling one leaves another at its
// top, and the one scrolled is still scrolled on return.
func TestEachPaneKeepsItsOwnScroll(t *testing.T) {
	t.Parallel()

	tops := map[string]string{"2": featureName, "4": "#42 " + pullTitle, "5": announcementLead}
	cases := map[string]struct{ from, to string }{
		"Branch, then Review": {from: "2", to: "4"},
		"Review, then Slack":  {from: "4", to: "5"},
		"Slack, then Branch":  {from: "5", to: "2"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			scrolled := typing(t, newWorld().live(t, 70, 8), tt.from, "J")

			// Act
			visiting := typing(t, scrolled, tt.to)
			back := typing(t, visiting, tt.from)

			// Assert
			requireScreen(t, visiting.View().Content, tops[tt.to])
			refuseScreen(t, back.View().Content, tops[tt.from])
		})
	}
}

// A reload of the queue that lands while the Reviews pane is out of sight still
// follows its selection, so the selected request is in sight on return.
func TestAReloadWhileAwayKeepsTheSelectedReviewInSight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.reviews = manyReviews(20)
	selected := typing(t, repo.live(t, 120, 16), selectingRow("6", 19)...)
	refreshing, answer := selected.Update(keyMsg("r"))
	away := typing(t, concrete(t, refreshing), "1")

	// Act
	view := typing(t, drain(t, away, answer), "6").View().Content

	// Assert
	requireScreen(t, view, "▸ · #1 queued change (1)")
}
