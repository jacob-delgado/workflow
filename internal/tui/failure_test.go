// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
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
	unanswered.edited = shortComment
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

// explainedError is a status Jira explained, as its client returns it: Jira's
// own reason, rejected, and marked with what the status says as well — a 404
// not-found, a 403 forbidden.
type explainedError struct {
	status error
	reason string
}

var _ error = explainedError{}

func (e explainedError) Error() string { return jira.ErrRejected.Error() + ": " + e.reason }

func (e explainedError) Unwrap() error { return jira.ErrRejected }

func (e explainedError) Is(target error) bool { return target == e.status }

func TestJirasOwnReasonLeads(t *testing.T) {
	t.Parallel()

	// Jira said why, so its reason leads rather than a guess at what the status
	// means: that the issue moved, or that the token may do nothing here.
	cases := map[string]struct {
		status error
		reason string
		guess  string
	}{
		"a missing issue": {
			status: jira.ErrNotFound, reason: "Issue does not exist or you do not have permission to see it.",
			guess: "No such issue",
		},
		"a token refused for this": {
			status: jira.ErrForbidden, reason: "You do not have permission to see this issue.",
			guess: "Jira refused the token",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			explained := newWorld()
			explained.detailErr = fmt.Errorf("reading PROJ-412: %w", explainedError{status: tt.status, reason: tt.reason})

			// Act
			view := explained.live(t, 200, 40).View().Content

			// Assert
			requireScreen(t, view, "✗ reading PROJ-412: jira rejected the request: "+tt.reason)
			refuseScreen(t, view, tt.guess)
		})
	}
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
		"the Review rail, when a program it waited on timed out": {
			start: withoutPull,
			prepare: func(w *world) {
				w.pullErr = fmt.Errorf("finding the pull request: %w", fmt.Errorf("gh: %w after 30s", proc.ErrTimedOut))
			},
			want: "✗ timed out",
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

func TestARefusedAnnouncementShowsTheServicesReason(t *testing.T) {
	t.Parallel()

	// Arrange
	archived := newWorld()
	archived.postErr = fmt.Errorf("%w: #dev is archived", messaging.ErrPostRefused)

	// Act
	view := typing(t, archived.live(t, 200, 40), "5", "p", keyEnter).View().Content

	// Assert
	// The service's refusal keeps its own verb-neutral words under the announce title.
	requireScreen(t, view, "Announce to Slack", "the message was refused: #dev is archived")
	refuseScreen(t, view, "post was refused")
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
			// A refused write leads with the scope, and guesses at no rate limit:
			// one that asks to wait is told apart before it reaches here.
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "rate limit")
		})
	}
}

// notOnPath is how a failure row tells a program workflow runs that is missing.
const notOnPath = "✗ A program workflow runs is not on PATH."

func TestAFailureRowSpeaksTheSentence(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
		want    string
	}{
		"a refused status change, in full on the picker's row": {
			prepare: func(w *world) {
				w.moves, w.transitionErr = workflowMoves(), fmt.Errorf("moving the issue: %w", jira.ErrForbidden)
			},
			keys: []string{"t", keyEnter},
			want: "✗ Jira refused the token for this.",
		},
		"a program not on PATH, in full on the run's row": {
			prepare: func(w *world) { w.commitErr = fmt.Errorf("%w: git", proc.ErrNotFound) },
			keys:    commitKeys("x"),
			want:    notOnPath,
		},
		"a refused CI read, in brief on the pane's summary row": {
			prepare: func(w *world) { w.ciErr = fmt.Errorf("checking CI: %w", forge.ErrRefused) },
			keys:    []string{"4"},
			want:    "CI     ✗ the forge refused the request",
		},
		"an unreachable service, in brief on the Messaging rail": {
			prepare: func(w *world) { w.postErr = fmt.Errorf("%w: dial tcp: i/o timeout", messaging.ErrUnreachable) },
			keys:    []string{"5", "p", keyEnter, keyEsc},
			want:    "✗ could not reach messaging",
		},
		"a refused assignment, in full on the form's row": {
			prepare: func(w *world) { w.assignErr = fmt.Errorf("assigning PROJ-412: %w", jira.ErrForbidden) },
			keys:    append(append([]string{"a"}, letters("fred")...), keyEnter),
			want:    "✗ Jira refused the token for this.",
		},
		"an unknown assignee, in Jira's own words on the form's row": {
			prepare: func(w *world) {
				w.assignErr = fmt.Errorf("assigning PROJ-412: %w",
					explainedError{status: jira.ErrNotFound, reason: "User 'fredd' does not exist."})
			},
			keys: append(append([]string{"a"}, letters("fredd")...), keyEnter),
			want: "✗ assigning PROJ-412: jira rejected the request: User 'fredd' does not exist.",
		},
		"branches that would not list, in full on the switcher's row": {
			prepare: func(w *world) {
				w.branchesErr = fmt.Errorf("listing branches: %w", fmt.Errorf("%w: git", proc.ErrNotFound))
			},
			keys: []string{"2", "s"},
			want: notOnPath,
		},
		"branches git gave up listing, in full on the switcher's row": {
			prepare: func(w *world) {
				w.branchesErr = fmt.Errorf("listing branches: %w", fmt.Errorf("git: %w after 30s", proc.ErrTimedOut))
			},
			keys: []string{"2", "s"},
			want: "✗ A program workflow runs did not answer in time and was stopped.",
		},
		"a switch git could not run, in full on the switcher's row": {
			prepare: func(w *world) {
				w.changes, w.branches = nil, []string{featureName, otherTaskBranch}
				w.checkoutErr = fmt.Errorf("switching to %s: %w", otherTaskBranch, fmt.Errorf("%w: git", proc.ErrNotFound))
			},
			keys: []string{"2", "s", keyEnter},
			want: notOnPath,
		},
		"a diff outside a repository, in full on the diff's row": {
			prepare: func(w *world) { w.diffErr = fmt.Errorf("%w: /repo", gitrepo.ErrNotARepository) },
			keys:    []string{"3"},
			want:    "✗ This is not inside a git repository.",
		},
		"a check with no browser to open it, in full on the checks' row": {
			prepare: func(w *world) {
				w.ci = []forge.CI{{
					State: forge.CIFailed, Total: 1, Done: 1, Failed: 1,
					Checks: []forge.Check{{Name: "vet", State: forge.CIFailed, URL: "https://ci.example/vet"}},
				}}
				w.openURLErr = fmt.Errorf("opening the check: %w", fmt.Errorf("%w: xdg-open", proc.ErrNotFound))
			},
			keys: []string{"4", "c", keyEnter},
			want: notOnPath,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 200, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestADetailTellsTheFailureItsSummaryRowShortens(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
		want    []string
	}{
		"a refused CI read, on the Review detail": {
			prepare: func(w *world) {
				w.ciErr = fmt.Errorf("checking CI: %w: Resource not accessible by integration", forge.ErrRefused)
			},
			keys: []string{"4"},
			want: []string{
				"CI     ✗ the forge refused the request",
				"✗ The forge refused the request: the token may lack a permission this needs.",
				"Resource not accessible by integration",
			},
		},
		"a refused announcement, on the Messaging detail once its preview is gone": {
			prepare: func(w *world) { w.postErr = fmt.Errorf("%w: invalid_token", messaging.ErrRejected) },
			keys:    []string{"5", "p", keyEnter, keyEsc},
			want: []string{
				"state  ✗ the announcement was refused",
				"✗ The messaging service refused the announcement:",
				"invalid_token",
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 200, 40), tt.keys...).View().Content

			// Assert
			// The summary row keeps the brief; the way out and the service's own
			// words stay on screen beneath it, not only in a notice the next key clears.
			requireScreen(t, view, tt.want...)
		})
	}
}

// rowShowing is the row of a screen that shows want, with its color escapes
// kept, or empty when no row does.
func rowShowing(view, want string) string {
	for row := range strings.SplitSeq(view, "\n") {
		if strings.Contains(ansi.Strip(row), want) {
			return row
		}
	}

	return ""
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestTheConfigurationScreenShowsItsErrorAsAFailure(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	model := sized(t, tui.New(config.Config{}, fmt.Errorf("%w: unexpected end of JSON input", config.ErrInvalid),
		tui.Deps{}), 120, 40)

	// Act
	view := model.View().Content

	// Assert
	requireScreen(t, view, "configuration error", "start over with `workflow config init --force`")

	if row := rowShowing(view, failGlyph+" invalid .workflow.json"); !strings.Contains(row, redOpen()+failGlyph) {
		t.Errorf("the configuration error is not a red failure row:\n%s", ansi.Strip(view))
	}
}

// Two problems a configuration can have at once.
var (
	errBadTemplate   = errors.New("invalid branch template: must contain {key}")
	errNegativeLimit = errors.New("invalid commit default: subject_limit cannot be negative: -1")
)

func TestTheConfigurationScreenGivesEachProblemItsOwnRow(t *testing.T) {
	t.Parallel()

	// Arrange
	// config.Load joins every problem it finds, one to a line.
	invalid := fmt.Errorf("%w: %w", config.ErrInvalid, errors.Join(errBadTemplate, errNegativeLimit))
	model := sized(t, tui.New(config.Config{}, invalid, tui.Deps{}), 80, 40)

	// Act
	view := model.View().Content

	// Assert
	requireScreen(t, view, "invalid branch template", "invalid commit default")

	if rowShowing(view, "invalid branch template") == rowShowing(view, "invalid commit default") {
		t.Errorf("both problems share one row:\n%s", ansi.Strip(view))
	}
}
