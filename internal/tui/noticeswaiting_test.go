// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

const (
	// issueListNotKept is the notice of an issue list the store lost.
	issueListNotKept = "the issue list was not kept for the next session: database or disk is full"
	// announcementNotConfirmed is the notice's note of an announcement the store
	// could not confirm.
	announcementNotConfirmed = "The store did not confirm it; it is not offered again. database or disk is full"
)

// What the store lost is said in the notice and nowhere else, so it is never
// dropped: it waits behind a failure no key has cleared yet, behind another
// loss not yet read, and while a search covers the notice row, and takes the
// notice, in turn, as keys clear what is before it.

// failedWithTwoLossesWaiting is the Tasks pane's stop failed, with the
// refresh's lost issue list and the announcement the store did not confirm
// waiting behind the failure, in that order.
func failedWithTwoLossesWaiting(t *testing.T) tui.Model {
	t.Helper()

	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	stopping, stopFailed := typing(t, concrete(t, refreshing), tasksPane).Update(keyMsg("s"))
	posting, posted := typing(t, concrete(t, stopping), "5", "p").Update(keyMsg(keyEnter))

	return drain(t, drain(t, drain(t, concrete(t, posting), stopFailed), refreshed), posted)
}

func TestWhatTheStoreLostWaitsInOrderBehindAnUnreadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh, the stop and the announcement are sent, each answer kept
	// back.
	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	stopping, stopFailed := typing(t, concrete(t, refreshing), tasksPane).Update(keyMsg("s"))
	posting, posted := typing(t, concrete(t, stopping), "5", "p").Update(keyMsg(keyEnter))

	// Act: the stop's failure lands, then the refresh's loss, then the post's,
	// with no key between them
	failed := drain(t, drain(t, drain(t, concrete(t, posting), stopFailed), refreshed), posted)

	// Assert: the failure keeps the notice
	requireFailureNotice(t, failed.View().Content, "taskwarrior refused the command: database is locked")

	// Act: a key clears the failure
	first := typing(t, failed, letterTyped)

	// Assert: the first loss takes the notice
	requireNotice(t, first.View().Content, issueListNotKept)

	// Act: the next key clears it
	second := typing(t, first, letterTyped)

	// Assert: the second loss takes the notice
	requireNotice(t, second.View().Content, announcementNotConfirmed)
}

func TestALossWaitingBehindAFailureOutlastsTheNoticeOfTheKeyThatClearsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh, the stop and the announcement are sent, each answer kept
	// back; the stop's failure lands, then the announcement's loss, with no
	// key between them.
	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	repo.storeErr = errStoreFull
	repo.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}}
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	stopping, stopFailed := typing(t, concrete(t, refreshing), tasksPane).Update(keyMsg("s"))
	posting, posted := typing(t, concrete(t, stopping), "5", "p").Update(keyMsg(keyEnter))
	waiting := drain(t, drain(t, concrete(t, posting), stopFailed), posted)

	// Act: a key clears the failure and says what it needs
	guided := typing(t, waiting, "3", "c")

	// Assert: the key's own guidance takes the notice
	requireNotice(t, guided.View().Content, "nothing is staged: stage a file, then commit")

	// Act: the refresh's loss lands
	landed := drain(t, guided, refreshed)

	// Assert: it waits behind the first, and the guidance keeps the notice
	requireNotice(t, landed.View().Content, "nothing is staged: stage a file, then commit")

	// Act: the next key clears the guidance
	first := typing(t, landed, letterTyped)

	// Assert: the first loss takes the notice
	requireNotice(t, first.View().Content, announcementNotConfirmed)

	// Act: the next key clears it
	second := typing(t, first, letterTyped)

	// Assert: the second loss takes the notice
	requireNotice(t, second.View().Content, issueListNotKept)

	// Act: the next key clears that
	cleared := typing(t, second, letterTyped)

	// Assert: nothing more was waiting
	refuseScreen(t, cleared.View().Content, issueListNotKept, announcementNotConfirmed)
}

func TestALossLandingOverAnUnreadLossWaitsBehindIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh and the announcement are sent, each answer kept back.
	repo := newWorld()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	posting, posted := typing(t, concrete(t, refreshing), "5", "p").Update(keyMsg(keyEnter))
	landed := drain(t, drain(t, concrete(t, posting), refreshed), posted)

	// Act
	next := typing(t, landed, letterTyped)

	// Assert
	requireNotice(t, next.View().Content, announcementNotConfirmed)
}

func TestALossTheClearingKeyShowedOutlastsThatKeysLaterAnswer(t *testing.T) {
	t.Parallel()

	for _, answer := range []struct {
		name     string
		writeErr error
	}{
		{name: "a plain result", writeErr: nil},
		{name: "a failure", writeErr: taskwarriorLocked()},
	} {
		t.Run(answer.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The refresh is sent and its answer kept back; the open fails and the
			// refresh's loss waits behind it. The stop clears the failure, the loss
			// takes the notice, and the stop's answer lands over it.
			repo := withTasks()
			repo.openURLErr = noOpener()
			repo.tasks.writeErr = answer.writeErr
			repo.storeErr = errStoreFull
			refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
			waiting := drain(t, typing(t, concrete(t, refreshing), tasksPane, "o"), refreshed)
			stopping, stopped := waiting.Update(keyMsg("s"))
			answered := drain(t, concrete(t, stopping), stopped)

			// Act
			next := typing(t, answered, letterTyped)

			// Assert
			requireNotice(t, next.View().Content, issueListNotKept)
		})
	}
}

func TestLossesWaitingAreNotUsedUpByKeysTypedIntoASearch(t *testing.T) {
	t.Parallel()

	// Arrange
	searched := typing(t, failedWithTwoLossesWaiting(t), tasksPane, "/", "a", "b")

	// Act
	closed := typing(t, searched, keyEsc)

	// Assert
	requireNotice(t, closed.View().Content, issueListNotKept)
}

func TestALossLandingWhileASearchCoversTheNoticeWaitsForItToClose(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh is sent and its answer kept back, and the search opened;
	// the refresh's loss lands while the search covers the notice row.
	repo := newWorld()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	searching := drain(t, typing(t, concrete(t, refreshing), "/"), refreshed)
	searched := typing(t, searching, "a")

	// Act
	closed := typing(t, searched, keyEsc)

	// Assert
	requireNotice(t, closed.View().Content, issueListNotKept)
}

func TestAWaitingNoticeShowsOnceAClickLeavesTheFilteredIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh is sent and its answer kept back, and the issues filtered;
	// the refresh's loss lands while the filter covers the notice row.
	repo := newWorld()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 120, 40).Update(keyMsg("r"))
	filtered := drain(t, typing(t, concrete(t, refreshing), "/", "a", keyEnter), refreshed)
	commits := screenRow(t, filtered.View().Content, "3 Commits")

	// Act
	clicked := click(t, filtered, 5, commits+1)

	// Assert
	requireNotice(t, clicked.View().Content, issueListNotKept)
}

// A failure that lands while a search covers the notice row waits for it, as
// what the store lost does, so the keys typed into the search do not clear it
// unseen; once the search ends it takes the row first, as it would have had
// no search covered it.

// stopFailedUnderASearch is the Tasks pane's stop sent and its answer kept
// back, a search typed over the Issues, and the stop's failure landed under
// it.
func stopFailedUnderASearch(t *testing.T, search ...string) tui.Model {
	t.Helper()

	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	stopping, stopFailed := typing(t, repo.live(t, 200, 40), tasksPane).Update(keyMsg("s"))

	return drain(t, typing(t, concrete(t, stopping), append([]string{"1", "/"}, search...)...), stopFailed)
}

func TestAFailureLandingUnderASearchShowsOnceTheSearchCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	searched := typing(t, stopFailedUnderASearch(t, "a"), "b")

	// Act
	closed := typing(t, searched, keyEsc)

	// Assert
	requireFailureNotice(t, closed.View().Content, "taskwarrior refused the command: database is locked")
}

func TestAFailureUnderASearchShowsBeforeALossThatLandedFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh and the stop are sent, each answer kept back, and a search
	// typed over the Issues; the refresh's loss lands under it, then the
	// stop's failure.
	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	stopping, stopFailed := typing(t, concrete(t, refreshing), tasksPane).Update(keyMsg("s"))
	searched := drain(t, drain(t, typing(t, concrete(t, stopping), "1", "/", "a"), refreshed), stopFailed)

	// Act
	closed := typing(t, searched, keyEsc)

	// Assert
	requireFailureNotice(t, closed.View().Content, "taskwarrior refused the command: database is locked")
}

func TestAFailureUnderAFilterTakesTheRowFromAReportItHid(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issues are filtered, m reports the mouse on under the filter, and
	// the stop's failure lands under it too.
	filtered := stopFailedUnderASearch(t, "a", keyEnter, "m")

	// Act
	left := typing(t, filtered, keyTab)

	// Assert
	requireFailureNotice(t, left.View().Content, "taskwarrior refused the command: database is locked")
}
