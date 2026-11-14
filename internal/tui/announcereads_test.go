// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The messaging pane reads what the store announced at start, on a refresh,
// and again after a post, a held post sent and local data removed, each of
// which changes it. It shows what the read it began last found.

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestARefreshBegunBeforeAPostDoesNotOfferItAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads what was announced, slowly, before the pull request
	// is announced; the post's own read answers first.
	posting := newWorld()
	refreshing, late := withheldKeys(t, typing(t, posting.live(t, 120, 40), "5"), posting.counting("history"), "r")
	posted := typing(t, refreshing, "p", keyEnter, "j")

	// Act
	view := handedOver(t, posted, late).View().Content

	// Assert
	requireScreen(t, view, "state  ● announced")
	refuseScreen(t, footerLine(view), "p announce")
}

func TestARefreshBegunBeforeLocalDataIsRemovedDoesNotBringItsClaimsBack(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request was announced in an earlier session; a refresh reads
	// that, slowly, and the cache, which holds the claim, is removed
	// meanwhile.
	removing := newWorld()
	removing.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}
	refreshing, late := withheldKeys(t, typing(t, removing.live(t, 120, 40), "5"), removing.counting("history"), "r")
	removed := typing(t, refreshing, reposKey, localDataKey, "c", keyEnter, "5")

	// Act
	view := handedOver(t, removed, late).View().Content

	// Assert
	requireScreen(t, view, "nothing announced")
	refuseScreen(t, view, "state  ● announced")
}

func TestThePostsReadOfWhatWasAnnouncedMarksTheTitleUntilItAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHold(t)
	pane := typing(t, heldLive(t, held, newWorld(), func(deps *tui.Deps, held *hold) {
		read := deps.Store.Announced
		deps.Store.Announced = func(ctx context.Context) ([]loop.AnnounceRecord, error) {
			held.wait()

			return read(ctx)
		}
	}), "5")
	held.armed.Store(true)

	// Act
	view := holding(t, held, pane, "p", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Slack ◐")
}
