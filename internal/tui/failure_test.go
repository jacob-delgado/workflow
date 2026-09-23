// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAKnownErrorReadsAsASentenceAndAnUnknownOneAsRawText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err  error
		want string
	}{
		"a known sentinel becomes a sentence": {
			err:  jira.ErrUnreachable,
			want: "Jira did not answer in time",
		},
		"an unknown error keeps its raw text": {
			err:  errUnreadable,
			want: "permission denied",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failing := newWorld()
			failing.detailErr = tt.err

			// Act
			view := failing.live(t, 120, 40).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestAPinnedRefusalSpeaksTheSentence(t *testing.T) {
	t.Parallel()

	// Arrange
	unanswered := newWorld()
	unanswered.edited = "a short comment"
	unanswered.commentErr = fmt.Errorf("posting the comment: %w", jira.ErrUnreachable)

	// Act
	view := typing(t, unanswered.live(t, 200, 40), "c", keyEnter).View().Content

	// Assert
	// The sentence leads, with the way out; the raw chain stays beneath it. It
	// names no key: an overlay owns the keyboard, and its footer offers the retry.
	requireScreen(t, view, "✗ Jira did not answer in time. Check the VPN, then try again.",
		"posting the comment: could not reach the server")
	refuseScreen(t, view, "press `r`")
}

// explainedNotFoundError is a 404 Jira explained, as its client returns it: Jira's
// own reason, rejected, and marked not-found as well.
type explainedNotFoundError struct{ reason string }

var _ error = explainedNotFoundError{}

func (e explainedNotFoundError) Error() string { return jira.ErrRejected.Error() + ": " + e.reason }

func (e explainedNotFoundError) Unwrap() error { return jira.ErrRejected }

func (e explainedNotFoundError) Is(target error) bool { return target == jira.ErrNotFound }

func TestJirasOwnReasonForANotFoundLeads(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := newWorld()
	missing.detailErr = fmt.Errorf("reading PROJ-412: %w",
		explainedNotFoundError{reason: "Issue does not exist or you do not have permission to see it."})

	// Act
	view := missing.live(t, 200, 40).View().Content

	// Assert
	// Jira said why, so its reason leads rather than a guess that the issue moved.
	requireScreen(t, view, "✗ reading PROJ-412: jira rejected the request: Issue does not exist")
	refuseScreen(t, view, "No such issue")
}

func TestARailSpeaksInBrief(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		start   func() *world
		prepare func(*world)
		keys    []string
		want    string
	}{
		"the Review rail": {
			start:   withoutPull,
			prepare: func(w *world) { w.pullErr = fmt.Errorf("finding the pull request: %w", forge.ErrNoRepository) },
			want:    "✗ the forge cannot see the repo",
		},
		"the Reviews rail": {
			start:   newWorld,
			prepare: func(w *world) { w.reviewsErr = fmt.Errorf("listing review requests: %w", forge.ErrUnauthorized) },
			keys:    []string{"6"},
			want:    "✗ the forge token is not valid",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := tt.start()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestTheMessagingRefusalNamesNoOneService(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := newWorld()
	refused.postErr = fmt.Errorf("%w: invalid_token", messaging.ErrRejected)

	// Act
	view := typing(t, refused.live(t, 200, 40), "5", "p", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "The messaging service refused the announcement: check the bot is in the channel "+
		"or the webhook is current.")
	refuseScreen(t, view, "Slack refused")
}

// lacksWriteScope is how a refused forge write names its likeliest fix.
const lacksWriteScope = "the token may lack the write scope"

func TestARefusedForgeWriteLeadsWithTheWriteScope(t *testing.T) {
	t.Parallel()

	refused := fmt.Errorf("writing: %w", forge.ErrRefused)

	cases := map[string]struct {
		start   func() *world
		prepare func(*world)
		keys    []string
		want    string
	}{
		"opening a pull request": {
			start:   withoutPull,
			prepare: func(w *world) { w.openErr = refused },
			keys:    []string{"4", "n", keyEnter},
			want:    lacksWriteScope,
		},
		"editing a pull request": {
			start:   newWorld,
			prepare: func(w *world) { w.editPullErr = refused },
			keys:    []string{"4", "e", keyEnter},
			want:    lacksWriteScope,
		},
		"closing an issue a forge tracks": {
			start:   newWorld,
			prepare: func(w *world) { w.moves, w.transitionErr = workflowMoves(), refused },
			keys:    []string{"t", keyEnter},
			want:    lacksWriteScope,
		},
		"adding the reviewers to a pull request that opened": {
			start:   withoutPull,
			prepare: func(w *world) { w.reviewerErr = refused },
			keys:    append(append([]string{"4", "n", keyTab, keyTab}, letters("ana")...), keyEnter),
			want:    "could not add every reviewer, assignee or label: the token may lack write scope",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := tt.start()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 200, 40), tt.keys...).View().Content

			// Assert
			// A refused write leads with the scope; waiting out a rate limit is the
			// lesser likelihood, not the headline.
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "which may be rate limiting. Wait a minute")
		})
	}
}
