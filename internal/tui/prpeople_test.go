// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheComposerOpensWithReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	// Two tabs reach the reviewers field, then each field is filled in turn.
	keys := slices.Concat(
		[]string{"4", "n", keyTab, keyTab}, letters("ana, ben"),
		[]string{keyTab}, letters("cass"),
		[]string{keyTab}, letters("bug"), []string{keyEnter},
	)

	// Act
	typing(t, model, keys...)

	// Assert
	calls := opening.asked("open ")
	if len(calls) != 1 || !strings.Contains(calls[0], "reviewers=ana,ben assignees=cass labels=bug") {
		t.Errorf("open calls = %q, want the reviewers, assignees and labels", calls)
	}
}

// reviewersField is how the composer's reviewers field starts on screen.
const reviewersField = "reviewers > "

// ownedBy is the world before a pull request is opened, every path of its
// base owned by owners.
func ownedBy(owners ...string) *world {
	w := withoutPull()
	w.codeOwners = owners

	return w
}

// openedBeforeTheOwnersAnswer opens the composer on the world and returns it
// with the read of the code owners not yet answered, so a test can act first
// and deliver the answer after.
func openedBeforeTheOwnersAnswer(t *testing.T, w *world) (tui.Model, tea.Cmd) {
	t.Helper()

	opened, read := typing(t, w.live(t, 120, 40), "4").Update(keyMsg("n"))

	return concrete(t, opened), read
}

func TestTheComposerPreFillsTheCodeOwnersAsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ownedBy("ana", "acme/control-plane", "ben")
	model := opening.live(t, 120, 40)

	// Act
	composer := typing(t, model, "4", "n")

	// Assert
	requireScreen(t, composer.View().Content, reviewersField+"ana, ben, acme/control-plane")

	if calls := opening.asked("code-owners main"); len(calls) != 1 {
		t.Errorf("CODEOWNERS reads = %q, want one, at the base", calls)
	}
}

func TestThePreFilledReviewersAreRequestedPeopleAndTeams(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ownedBy("ana", "acme/control-plane")
	model := opening.live(t, 120, 40)

	// Act
	typing(t, model, "4", "n", keyEnter)

	// Assert
	calls := opening.asked("open ")
	if len(calls) != 1 || !strings.Contains(calls[0], "reviewers=ana teams=acme/control-plane") {
		t.Errorf("open calls = %q, want ana as a reviewer and acme/control-plane as a team", calls)
	}
}

func TestTheComposerDoesNotProposeTheAuthorAsAReviewer(t *testing.T) {
	t.Parallel()

	// Arrange
	// The world's forge credential is jacob's, named here as CODEOWNERS might.
	opening := ownedBy("Jacob", "ana")
	model := opening.live(t, 120, 40)

	// Act
	typing(t, model, "4", "n", keyEnter)

	// Assert
	if calls := opening.asked("open "); len(calls) != 1 || !strings.Contains(calls[0], "draft=no reviewers=ana\n") {
		t.Errorf("open calls = %q, want ana alone as a reviewer", calls)
	}
}

// errCodeOwnersUnreadable is a CODEOWNERS file the repository names but the
// user cannot read.
var errCodeOwnersUnreadable = errors.New("reading CODEOWNERS: permission denied")

func TestAComposerWhoseOwnersCannotBeReadProposesNoReviewers(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*world){
		"CODEOWNERS unreadable":  func(w *world) { w.codeOwnersErr = errCodeOwnersUnreadable },
		"the changes unreadable": func(w *world) { w.changedPathsErr = errCodeOwnersUnreadable },
		"no repository to read":  func(w *world) { w.noCodeOwners = true },
	}

	for name, fail := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Owners come back with the failure, so a composer that read past it
			// would propose them.
			opening := ownedBy("ana", "ben")
			fail(opening)
			model := opening.live(t, 120, 40)

			// Act
			typing(t, model, "4", "n", keyEnter)

			// Assert
			if calls := opening.asked("open "); len(calls) != 1 || strings.Contains(calls[0], "reviewers=") {
				t.Errorf("open calls = %q, want one naming no reviewer", calls)
			}
		})
	}
}

func TestReviewersTypedBeforeTheOwnersAnswerAreKept(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheOwnersAnswer(t, ownedBy("ana"))
	typed := typing(t, opened, keyTab, keyTab, "z")

	// Act
	view := drain(t, typed, read).View().Content

	// Assert
	requireScreen(t, view, reviewersField+"z")
	refuseScreen(t, view, "ana")
}

func TestReviewersBeingSentAreNotReplacedByTheOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	// Enter sends no reviewers; the pull request it opens is left unanswered,
	// so the composer is still sending when the owners answer.
	opened, read := openedBeforeTheOwnersAnswer(t, ownedBy("ana"))
	sending, _ := opened.Update(keyMsg(keyEnter))

	// Act
	view := drain(t, concrete(t, sending), read).View().Content

	// Assert
	requireScreen(t, view, reviewersField)
	refuseScreen(t, view, "ana")
}

func TestOwnersAnsweringAfterTheComposerClosedOpenNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheOwnersAnswer(t, ownedBy("ana"))
	closed := typing(t, opened, keyEsc)

	// Act
	view := drain(t, closed, read).View().Content

	// Assert
	refuseScreen(t, view, reviewersField)
}

func TestOwnersAnsweringForAnotherBranchLeaveItsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	// Before the owners answer, the composer is closed and the branch is
	// switched in a shell to one whose base no CODEOWNERS covers.
	opening := ownedBy("ana")
	opened, read := openedBeforeTheOwnersAnswer(t, opening)
	closed := typing(t, opened, keyEsc)
	opening.branch.Name = issuelessBranch
	opening.codeOwners = nil
	reopened := typing(t, closed, "r", "n")

	// Act
	view := drain(t, reopened, read).View().Content

	// Assert
	requireScreen(t, view, reviewersField)
	refuseScreen(t, view, "ana")
}

func TestAKeptDraftsReviewersWinOverTheOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	// The proposed owner is erased by hand, and the composer closed, which
	// keeps the draft with no reviewers.
	opening := ownedBy("ana")
	model := opening.live(t, 120, 40)
	erased := typing(t, model, "4", "n", keyTab, keyTab, keyBackspace, keyBackspace, keyBackspace, keyEsc)

	// Act
	typing(t, erased, "n", keyEnter)

	// Assert
	if calls := opening.asked("open "); len(calls) != 1 || strings.Contains(calls[0], "reviewers=") {
		t.Errorf("open calls = %q, want one naming no reviewer", calls)
	}
}

func TestAPullThatOpensButCannotAddReviewersIsNotLost(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull opens, but the forge knows no reviewer by that name.
	opening := withoutPull()
	opening.reviewerErr = fmt.Errorf("%w (ana): %w", forge.ErrSomeReviewersNotAdded, forge.ErrNoUser)
	model := opening.live(t, 200, 40)

	keys := append([]string{"4", "n", keyTab, keyTab}, letters("ana")...)

	// Act
	opened := typing(t, model, append(keys, keyEnter)...)

	// Assert
	requireScreen(t, opened.View().Content, "opened #42",
		"could not add every reviewer, assignee or label: the forge has no such user")
	refuseScreen(t, opened.View().Content, "┏━ Open pull request")
}

func TestShiftTabStepsBackwardThroughTheComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	// Act
	composer := typing(t, model, "4", "n", keyShiftTab)

	// Assert
	requireScreen(t, composer.View().Content, "▸ labels")
}
