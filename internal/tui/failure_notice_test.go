// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// requireFailureNotice fails the test unless the notice row, the one above the
// footer, shows want after the failure mark, with the mark in the failure color.
func requireFailureNotice(t *testing.T, view, want string) {
	t.Helper()

	lines := strings.Split(view, "\n")
	notice := lines[len(lines)-2]

	if !strings.Contains(ansi.Strip(notice), failGlyph+" "+want) || !strings.Contains(notice, redOpen()+failGlyph) {
		t.Errorf("the notice does not show %q after a red %q:\n%s", want, failGlyph, ansi.Strip(view))
	}
}

// requireNotice fails the test unless the notice row, the one above the footer,
// shows want.
func requireNotice(t *testing.T, view, want string) {
	t.Helper()

	lines := strings.Split(view, "\n")

	if notice := ansi.Strip(lines[len(lines)-2]); !strings.Contains(notice, want) {
		t.Errorf("the notice does not show %q:\n%s", want, ansi.Strip(view))
	}
}

func TestARefusalNoticeWearsTheFailureStyle(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
		want    string
	}{
		"a failure with no job to re-run": {
			prepare: func(w *world) {
				w.ci, w.nothingToRerun = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}, true
			},
			keys: []string{"4", "R", keyEnter},
			want: "nothing to re-run: this failure has no job to restart",
		},
		"a re-run the forge rejects": {
			prepare: func(w *world) {
				w.ci, w.rerunErr = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}, forge.ErrRejected
			},
			keys: []string{"4", "R", keyEnter},
			want: "re-run failed: the forge rejected the request",
		},
		"a queued post that may have gone out once CI passed": {
			prepare: func(w *world) {
				w.ciInterval = time.Millisecond
				w.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}
				w.postErr = fmt.Errorf("%w: i/o timeout", messaging.ErrUnreachable)
			},
			keys: []string{"5", "p", "w"},
			want: "dropped the announcement to Slack waiting for #42: " +
				"The messaging service did not say whether it took the post",
		},
		"a link with no browser to open it": {
			prepare: func(w *world) {
				w.openURLErr = fmt.Errorf("opening https://jira.example/browse/PROJ-412: %w",
					fmt.Errorf("%w: xdg-open", proc.ErrNotFound))
			},
			keys: []string{"o"},
			want: "A program workflow runs is not on PATH.",
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
			requireFailureNotice(t, view, tt.want)
		})
	}
}

func TestAGuidanceNoticeStaysPlain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
		want    string
	}{
		"nothing staged to commit": {
			prepare: func(w *world) { w.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}} },
			keys:    []string{"3", "c"},
			want:    "nothing is staged: stage a file, then commit",
		},
		"a comment saved empty": {
			prepare: func(w *world) { w.edited = "   " },
			keys:    []string{"c", keyCtrlO, keyEnter},
			want:    "nothing to post: the comment was empty",
		},
		"a message emptied": {
			prepare: func(w *world) { w.edited = "  " },
			keys:    []string{"5", "p", "e", keyEnter},
			want:    "nothing to announce: the message was empty",
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
			// Red means something broke; being told what a key needs is not that.
			row := rowShowing(view, tt.want)
			if row == "" || strings.Contains(row, failGlyph) || strings.Contains(row, redOpen()) {
				t.Errorf("want %q on a plain row with no failure mark, got %q:\n%s", tt.want, row, ansi.Strip(view))
			}
		})
	}
}

func TestADroppedPostIsNoticedAsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	switching := newWorld()
	switching.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, switching.live(t, 200, 40), "5", "p", "w")

	anotherPull(switching)

	// Act
	view := typing(t, waiting, "r").View().Content

	// Assert
	requireFailureNotice(t, view, strings.TrimPrefix(droppedPost, failGlyph+" "))
}

func TestAShortTerminalsFooterDrawsAFailureInRed(t *testing.T) {
	t.Parallel()

	// Arrange
	// Too short for a notice row of its own, so the footer stands in for it.
	faked := newWorld()
	faked.stageErr = fmt.Errorf("staging: %w", fmt.Errorf("%w: git", proc.ErrNotFound))

	// Act
	view := typing(t, faked.live(t, 120, 3), "3", keySpace).View().Content

	// Assert
	footer := footerLine(view)
	if lines := strings.Split(view, "\n"); !strings.Contains(footer, "✗ A program workflow runs is not on PATH.") ||
		!strings.Contains(lines[len(lines)-1], redOpen()+failGlyph) {
		t.Errorf("the footer does not stand in with a red failure:\n%s", ansi.Strip(view))
	}
}

// taskwarriorLocked is the reason a task's stop fails: Taskwarrior refused the write.
func taskwarriorLocked() error {
	return fmt.Errorf("%w: database is locked", taskwarrior.ErrRefused)
}

// noOpener is the reason opening the selected task's issue fails: no browser
// opener on PATH.
func noOpener() error {
	return fmt.Errorf("opening the issue: %w", fmt.Errorf("%w: xdg-open", proc.ErrNotFound))
}

func TestAFailureNoticeOutlastsAPlainOneThatLandsBeforeAKey(t *testing.T) {
	t.Parallel()

	// Arrange
	// The stop is sent and its answer kept back, the open then fails, and the
	// stop's plain result lands after the failure with no key between them.
	repo := withTasks()
	repo.openURLErr = noOpener()
	stopping, stopped := typing(t, repo.live(t, 200, 40), tasksPane).Update(keyMsg("s"))
	failed := typing(t, concrete(t, stopping), "o")

	// Act
	view := drain(t, failed, stopped).View().Content

	// Assert
	requireFailureNotice(t, view, "A program workflow runs is not on PATH.")
}

func TestALaterFailureNoticeReplacesAnEarlierOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.openURLErr = noOpener()
	repo.tasks.writeErr = taskwarriorLocked()
	stopping, stopped := typing(t, repo.live(t, 200, 40), tasksPane).Update(keyMsg("s"))
	failed := typing(t, concrete(t, stopping), "o")

	// Act
	view := drain(t, failed, stopped).View().Content

	// Assert
	requireFailureNotice(t, view, "taskwarrior refused the command: database is locked")
}
