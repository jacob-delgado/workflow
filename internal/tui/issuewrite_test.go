// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

var errForbidden = errors.New("forbidden")

// The two issue-write actions, named once for the table cases and the recorded
// call prefixes that check them.
const (
	assignCase  = "assign"
	logWorkCase = "log work"
)

// typeInto opens a write form with an open key, types text, and returns the
// model without confirming.
func typeInto(t *testing.T, repo *world, openKey, text string) tui.Model {
	t.Helper()

	keys := append([]string{openKey}, letters(text)...)

	return typing(t, repo.live(t, 120, 40), keys...)
}

func TestAssigningPostsTheAssigneeAfterAPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act: open the assign form and type a username
	form := typeInto(t, repo, "a", "fred")

	// Assert: the form is up and nothing is sent before confirming
	requireScreen(t, form.View().Content, "Assign "+issueKey)

	if got := repo.asked(assignCase); len(got) != 0 {
		t.Errorf("assigned before confirming: %v", got)
	}

	// Act: confirm
	view := typing(t, form, keyEnter).View().Content

	// Assert: it posts the assignee, says so, and re-reads the issue to show it
	if got := repo.asked("assign " + issueKey + " fred"); len(got) != 1 {
		t.Errorf("assign calls = %v, want one for the typed username", got)
	}

	requireScreen(t, view, "assigned "+issueKey+" to fred")

	if reads := repo.asked("issue " + issueKey); len(reads) != 2 {
		t.Errorf("issue reads = %d, want two: the initial load and the refresh after assigning", len(reads))
	}
}

func TestLoggingWorkPostsTheDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, typeInto(t, repo, "w", "2h"), keyEnter).View().Content

	// Assert
	if got := repo.asked("worklog " + issueKey + " 2h "); len(got) != 1 {
		t.Errorf("worklog calls = %v, want one for the typed duration", got)
	}

	requireScreen(t, view, "logged 2h on "+issueKey)

	if reads := repo.asked("issue " + issueKey); len(reads) != 2 {
		t.Errorf("issue reads = %d, want two: the initial load and the refresh after logging work", len(reads))
	}
}

func TestAnIssueWriteHeldBackInADryRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		openKey string
		text    string
		want    string
		prefix  string
	}{
		assignCase:  {openKey: "a", text: "fred", want: "would assign " + issueKey + " to fred", prefix: "assign"},
		logWorkCase: {openKey: "w", text: "2h", want: "would log 2h on " + issueKey, prefix: "worklog"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			base := sized(t, dryInterface(repo), 120, 40)
			model := drain(t, base, base.Init())

			keys := append([]string{testCase.openKey}, letters(testCase.text)...)

			// Act
			view := typing(t, model, append(keys, keyEnter)...).View().Content

			// Assert
			requireScreen(t, view, "dry run: "+testCase.want)

			if got := repo.asked(testCase.prefix); len(got) != 0 {
				t.Errorf("a dry run wrote: %v", got)
			}
		})
	}
}

func TestAnEmptyIssueWriteIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), "a", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "needs a value")

	if got := repo.asked(assignCase); len(got) != 0 {
		t.Errorf("an empty form assigned: %v", got)
	}
}

func TestAFailedIssueWriteKeepsTheFormOpenWithTheReason(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		openKey string
		text    string
		title   string
		fail    func(*world)
	}{
		assignCase:  {openKey: "a", text: "fred", title: "Assign", fail: func(w *world) { w.assignErr = errForbidden }},
		logWorkCase: {openKey: "w", text: "2h", title: "Log work", fail: func(w *world) { w.worklogErr = errForbidden }},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			testCase.fail(repo)

			// Act
			view := typing(t, typeInto(t, repo, testCase.openKey, testCase.text), keyEnter).View().Content

			// Assert
			requireScreen(t, view, testCase.title+" "+issueKey, "forbidden")
		})
	}
}

func TestCancelingAnIssueWriteSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, typeInto(t, repo, "a", "fred"), keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Assign "+issueKey)

	if got := repo.asked(assignCase); len(got) != 0 {
		t.Errorf("a canceled form assigned: %v", got)
	}
}

func TestAnIssueWriteNeedsItsSeam(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		openKey string
		title   string
		nilOut  func(*tui.Deps)
	}{
		assignCase:  {openKey: "a", title: "Assign", nilOut: func(d *tui.Deps) { d.Jira.Assign = nil }},
		logWorkCase: {openKey: "w", title: "Log work", nilOut: func(d *tui.Deps) { d.Jira.AddWorklog = nil }},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			without := repo.deps()
			testCase.nilOut(&without)
			base := sized(t, tui.New(repo.cfg, nil, without), 120, 40)
			noSeam := drain(t, base, base.Init())

			// Act
			withForm := typing(t, repo.live(t, 120, 40), testCase.openKey).View().Content
			withoutForm := typing(t, noSeam, testCase.openKey).View().Content

			// Assert
			requireScreen(t, withForm, testCase.title+" "+issueKey)
			refuseScreen(t, withoutForm, testCase.title+" "+issueKey)
		})
	}
}

func TestTheHelpListsAssignAndLogWork(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 50), "?").View().Content

	// Assert
	requireScreen(t, view, "assign", "log work")
}
