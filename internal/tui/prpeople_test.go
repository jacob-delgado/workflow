// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheComposerOpensWithReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	// Past where it goes, two tabs reach the reviewers field, then each field
	// is filled in turn.
	keys := []string{
		"4", "n", keyEnter, keyTab, keyTab, pasted("ana, ben"), keyTab, pasted("cass"), keyTab, pasted("bug"), keyEnter,
	}

	// Act
	typing(t, model, keys...)

	// Assert
	calls := opening.asked("open ")
	if len(calls) != 1 || !strings.Contains(calls[0], "reviewers=ana,ben assignees=cass labels=bug") {
		t.Errorf("open calls = %q, want the reviewers, assignees and labels", calls)
	}
}

// reviewersField is how the composer's reviewers field starts on screen, once
// it asks what the pull request says.
const reviewersField = "> "

// ownedBy is the world before a pull request is opened, every path of its
// base owned by owners.
func ownedBy(owners ...string) *world {
	w := withoutPull()
	w.codeOwners = owners

	return w
}

// openedBeforeTheOwnersAnswer opens the composer on the interface and returns
// it with the read of the code owners not yet answered, so a test can act
// first and deliver the answer after.
func openedBeforeTheOwnersAnswer(t *testing.T, live tui.Model) (tui.Model, tea.Cmd) {
	t.Helper()

	opened, read := typing(t, live, "4").Update(keyMsg("n"))

	return concrete(t, opened), read
}

func TestTheComposerPreFillsTheCodeOwnersAsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ownedBy("ana", "acme/control-plane", "ben")
	model := opening.live(t, 120, 40)

	// Act
	composer := typing(t, model, "4", "n", keyEnter)

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
	typing(t, model, "4", "n", keyEnter, keyEnter)

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
	typing(t, model, "4", "n", keyEnter, keyEnter)

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
			typing(t, model, "4", "n", keyEnter, keyEnter)

			// Assert
			if calls := opening.asked("open "); len(calls) != 1 || strings.Contains(calls[0], "reviewers=") {
				t.Errorf("open calls = %q, want one naming no reviewer", calls)
			}
		})
	}
}

func TestOwnersAnsweringAfterTheComposerClosedOpenNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheOwnersAnswer(t, ownedBy("ana").live(t, 120, 40))
	closed := typing(t, opened, keyEsc)

	// Act
	view := drain(t, closed, read).View().Content

	// Assert
	refuseScreen(t, view, "┏━ Open pull request")
}

func TestOwnersAnsweringAnEarlierComposerLeaveTheReopenedOneReading(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first composer's reads have answered, though not yet been handed
	// over, by the time it is closed and another opens, its own owners held.
	held := newHold(t)
	first, read := openedBeforeTheOwnersAnswer(t, heldLive(t, held, ownedBy("ana"), heldOwners))
	earlier := answersOf(t, first, read)
	closed := typing(t, first, keyEsc)

	held.armed.Store(true)
	reopened := holding(t, held, closed, "n")

	// Act
	answered := reopened
	for _, msg := range earlier {
		next, follow := held.update(t, answered, msg)
		answered = drainPast(t, held, next, follow)
	}

	// Assert
	held.requireReached(t)
	requireScreen(t, answered.View().Content, "reading what the pull request starts from")
	refuseScreen(t, answered.View().Content, "ana")
}

// answersOf runs cmd and every command it leads to, keeping the messages they
// answer with rather than handing them to model, so a test can hand them to
// it later.
func answersOf(t *testing.T, model tui.Model, cmd tea.Cmd) []tea.Msg {
	t.Helper()

	deadline := time.NewTimer(failsafe)
	defer deadline.Stop()

	var answers []tea.Msg

	settle(t, model, cmd, driver{
		run: func(next tea.Cmd) (tea.Msg, bool) { return await(t, next, deadline.C), true },
		deliver: func(model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
			answers = append(answers, msg)

			return model, nil
		},
	})

	return answers
}

func TestAKeptDraftsReviewersWinOverTheOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	// The proposed owner is erased by hand, and the composer closed, which
	// keeps the draft with no reviewers.
	opening := ownedBy("ana")
	model := opening.live(t, 120, 40)
	erased := typing(t, model, "4", "n", keyEnter, keyTab, keyTab, keyClearLine, keyEsc)

	// Act
	typing(t, erased, "n", keyEnter, keyEnter)

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
	opening.reviewerErr = fmt.Errorf("%w (ana): %w", forge.ErrSomePeopleNotAdded, forge.ErrNoUser)
	model := opening.live(t, 200, 40)

	keys := []string{"4", "n", keyEnter, keyTab, keyTab, pasted("ana"), keyEnter}

	// Act
	opened := typing(t, model, keys...)

	// Assert
	requireScreen(t, opened.View().Content, "opened #42",
		"could not add every reviewer, assignee or label: the forge has no such user")
	refuseScreen(t, opened.View().Content, "┏━ Open pull request")
}

func TestShiftTabOnTheTitleGoesBackToWhereThePullRequestGoes(t *testing.T) {
	t.Parallel()

	// Arrange
	saying := typing(t, withoutPull().live(t, 120, 40), "4", "n", keyEnter)

	// Act
	composer := typing(t, saying, keyShiftTab)

	// Assert
	requireScreen(t, composer.View().Content, "Base", "> main")
	refuseScreen(t, composer.View().Content, "Reviewers")
}

func TestADraftClosedBeforeTheOwnersAnsweredIsFilledOnReopening(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheOwnersAnswer(t, ownedBy("ana").live(t, 120, 40))
	closed := drain(t, typing(t, opened, "x", keyEsc), read)

	// Act
	view := typing(t, closed, "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, reviewersField+"ana")
}
