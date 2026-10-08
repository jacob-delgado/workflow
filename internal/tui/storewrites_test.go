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

func TestAnAnnouncementTheStoreCannotKeepIsPostedAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	announcing := newWorld()
	announcing.storeErr = errStoreFull

	// Act
	posted := typing(t, announcing.live(t, 120, 40), "5", "p", keyEnter)

	// Assert
	requireScreen(t, posted.View().Content, "announced to", "could not be remembered", "disk is full")
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
