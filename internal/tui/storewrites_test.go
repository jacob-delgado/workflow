// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// What the store could not keep is said, beside what was done: an
// announcement posted, a commit made, an issue list loaded all happened, and
// only the next session's memory of them is lost.

import (
	"errors"
	"testing"
)

// errStoreFull is a store that could not write.
var errStoreFull = errors.New("database or disk is full")

func TestAnAnnouncementTheStoreCannotConfirmIsPostedAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	announcing := newWorld()
	announcing.storeErr = errStoreFull

	// Act
	posted := typing(t, announcing.live(t, 120, 40), "5", "p", keyEnter)

	// Assert
	requireScreen(t, posted.View().Content, "announced to",
		"The store did not confirm it; it is not offered again. database or disk is full")
}

func TestACommitWhoseScopeTheStoreCannotKeepSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	composing.storeErr = errStoreFull

	keys := append([]string{"3", "c", keyShiftTab}, letters("config")...)
	keys = append(append(keys, keyTab), letters("redact tokens")...)
	keys = append(keys, keyEnter)

	// Act
	committed := typing(t, composing.live(t, 120, 40), keys...)

	// Assert
	requireScreen(t, committed.View().Content, "committed", "scope was not remembered", "disk is full")
}

func TestAnIssueListTheStoreCannotKeepSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	searching := newWorld()
	searching.storeErr = errStoreFull

	// Act
	listed := searching.live(t, 120, 40)

	// Assert
	requireScreen(t, listed.View().Content, "issue list was not kept", "disk is full")
}

// What the store lost is said in the notice and nowhere else, so it takes the
// notice even from a failure no key has cleared yet.

func TestAnAnnouncementTheStoreCannotConfirmSaysSoOverAnUnreadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// The stop is sent and its failure kept back, the announcement is sent and
	// its answer kept back too; the stop's failure lands, then the post's
	// answer, with no key between them.
	repo := withTasks()
	repo.tasks.writeErr = taskwarriorLocked()
	repo.storeErr = errStoreFull
	stopping, stopFailed := typing(t, repo.live(t, 200, 40), tasksPane).Update(keyMsg("s"))
	posting, posted := typing(t, concrete(t, stopping), "5", "p").Update(keyMsg(keyEnter))
	failed := drain(t, concrete(t, posting), stopFailed)

	// Act
	view := drain(t, failed, posted).View().Content

	// Assert
	requireNotice(t, view, "The store did not confirm it; it is not offered again. database or disk is full")
}

func TestAnIssueListTheStoreCannotKeepSaysSoOverAnUnreadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh is sent and its answer kept back, the open then fails, and
	// the store's loss lands after the failure with no key between them.
	repo := withTasks()
	repo.openURLErr = noOpener()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 200, 40).Update(keyMsg("r"))
	failed := typing(t, concrete(t, refreshing), tasksPane, "o")

	// Act
	view := drain(t, failed, refreshed).View().Content

	// Assert
	requireNotice(t, view, "the issue list was not kept for the next session: database or disk is full")
}
