// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// Errors the fakes answer with.
var (
	errNotVisible   = errors.New("jira rejected the request: Issue Does Not Exist")
	errEditorFailed = errors.New("the editor exited with an error")
)

func TestTheDetailShowsTheIssueInFull(t *testing.T) {
	t.Parallel()

	view := newWorld().live(t, 120, 40).View()

	requireScreen(t, view, issueKey+" "+issueSummary, "Bug · In Progress", "reported by Ana Lopez",
		"Tokens reach the log.", "Comments 1 of 1", "Ana Lopez · 2h ago", "Repro'd on 8.2.1")
}

func TestTheDetailSaysWhatIsMissing(t *testing.T) {
	t.Parallel()

	quiet := newWorld()
	quiet.detail = jira.IssueDetail{Issue: jira.Issue{Key: issueKey}, Reporter: reporter, CommentTotal: 0}
	view := quiet.live(t, 120, 40).View()

	requireScreen(t, view, "no description")
	refuseScreen(t, view, "Comments")

	failing := newWorld()
	failing.detailErr = errNotVisible
	requireScreen(t, failing.live(t, 120, 40).View(), "✗ jira rejected the request: Issue Does Not Exist")
}

func TestCommentAgesReadAsBrieflyAsTheyCan(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		created time.Time
		want    string
	}{
		"moments":         {created: testNow().Add(-20 * time.Second), want: "just now"},
		"minutes":         {created: testNow().Add(-5 * time.Minute), want: "5m ago"},
		"days":            {created: testNow().Add(-72 * time.Hour), want: "3d ago"},
		"long ago":        {created: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), want: "2026-01-02"},
		"an unknown date": {created: time.Time{}, want: "some time ago"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			aged := newWorld()
			aged.detail.Comments[0].Created = tt.created

			requireScreen(t, aged.live(t, 120, 40).View(), "Ana Lopez · "+tt.want)
		})
	}
}

func TestTheDetailFollowsTheSelectionOnceItRests(t *testing.T) {
	t.Parallel()

	moving := newWorld()
	screen := moving.live(t, 120, 40)
	moving.detail = jira.IssueDetail{Issue: jira.Issue{Key: "PROJ-388"}, Reporter: "Fred", Description: "Retries."}

	// Before the pause, the header is the new issue and the full read is on its
	// way; after it, the new issue's description is shown.
	updated, cmd := screen.Update(keyMsg("j"))
	requireScreen(t, concrete(t, updated).View(), "PROJ-388 Add retries", "loading the description and comments…")

	requireScreen(t, drain(t, concrete(t, updated), cmd).View(), "Retries.", "reported by Fred")

	if reads := moving.asked("issue PROJ-388"); len(reads) != 1 {
		t.Errorf("read PROJ-388 %d times, want once", len(reads))
	}
}

func TestADetailForAnIssueNoLongerSelectedIsNotShown(t *testing.T) {
	t.Parallel()

	stale := newWorld()
	screen := stale.live(t, 120, 40)

	// Move away and back before either read finishes: only the issue selected
	// when an answer arrives may use it.
	away, readAway := screen.Update(keyMsg("j"))
	back, readBack := concrete(t, away).Update(keyMsg("k"))

	settled := drain(t, drain(t, concrete(t, back), readAway), readBack)
	requireScreen(t, settled.View(), issueKey+" "+issueSummary, "Tokens reach the log.")
}

func TestWorkInProgressIsWhereTheInterfaceOpens(t *testing.T) {
	t.Parallel()

	// The branch names the second issue in the list, so that is selected.
	resuming := newWorld()
	resuming.branch.Name = "feat/PROJ-388-add-retries"

	requireScreen(t, resuming.live(t, 120, 40).View(), "▸ ○ PROJ-388 Add retries")

	// A branch naming an issue not in the list leaves the selection alone.
	elsewhere := newWorld()
	elsewhere.branch.Name = "fix/OTHER-1-thing"

	requireScreen(t, elsewhere.live(t, 120, 40).View(), "▸ ◐ PROJ-412")
}

func TestResumingNeverOverridesAChoice(t *testing.T) {
	t.Parallel()

	// No branch names an issue at start; the user chooses one; then the branch
	// is read again and turns out to name a different issue.
	chosen := newWorld()
	chosen.branch = gitrepo.Branch{Name: baseName, Base: baseRef}

	screen := typing(t, chosen.live(t, 120, 40), "j")
	chosen.branch = gitrepo.Branch{Name: featureName, Base: baseRef}

	reloaded := typing(t, screen, "2", "r", "1")
	requireScreen(t, reloaded.View(), "▸ ○ PROJ-388")
}

func TestCommentingPreviewsBeforeItPosts(t *testing.T) {
	t.Parallel()

	commenting := newWorld()
	commenting.edited = "Patch up shortly"

	screen := typing(t, commenting.live(t, 120, 40), "c")
	requireScreen(t, screen.View(), "┏━ Comment on PROJ-412", "Patch up shortly", "enter post", "e edit", "esc discard")

	if posted := commenting.asked("comment"); len(posted) != 0 {
		t.Fatalf("posted before the preview was confirmed: %q", posted)
	}

	posted := typing(t, screen, keyEnter)
	requireScreen(t, posted.View(), "● commented on PROJ-412")
	refuseScreen(t, posted.View(), "┏━ Comment on")

	if calls := commenting.asked("comment PROJ-412 Patch up shortly"); len(calls) != 1 {
		t.Errorf("comment calls = %q, want one", commenting.asked("comment"))
	}

	// The issue is read again to show the new comment.
	if reads := commenting.asked("issue " + issueKey); len(reads) < 2 {
		t.Errorf("read the issue %d times, want it read again after commenting", len(reads))
	}
}

func TestACommentCanBeEditedAgainOrDiscarded(t *testing.T) {
	t.Parallel()

	commenting := newWorld()
	commenting.edited = "first draft"

	screen := typing(t, commenting.live(t, 120, 40), "c")
	commenting.edited = "second draft"

	edited := typing(t, screen, "e")
	requireScreen(t, edited.View(), "second draft")

	if edits := commenting.asked("edit first draft"); len(edits) != 1 {
		t.Errorf("the second edit did not start from the first draft: %q", commenting.asked("edit"))
	}

	discarded := typing(t, edited, "esc")
	requireScreen(t, discarded.View(), "comment discarded")

	if posted := commenting.asked("comment"); len(posted) != 0 {
		t.Errorf("a discarded comment was posted: %q", posted)
	}
}

func TestACommentThatCannotBePostedSaysWhy(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
		want    string
	}{
		"empty": {
			prepare: func(w *world) { w.edited = "   " }, keys: []string{"c"},
			want: "nothing to post: the comment was empty",
		},
		"editor failed": {
			prepare: func(w *world) { w.editErr = errEditorFailed }, keys: []string{"c"},
			want: "✗ the editor exited with an error",
		},
		// Refused, the preview stays open with Jira's reason.
		"refused": {
			prepare: func(w *world) { w.edited = "hello"; w.commentErr = errNotVisible }, keys: []string{"c", keyEnter},
			want: "✗ jira rejected the request",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			failing := newWorld()
			tt.prepare(failing)

			requireScreen(t, typing(t, failing.live(t, 120, 40), tt.keys...).View(), tt.want)
		})
	}
}

func TestNothingInterruptsACommentBeingPosted(t *testing.T) {
	t.Parallel()

	commenting := newWorld()
	commenting.edited = "hello"

	screen := typing(t, commenting.live(t, 120, 40), "c")
	sending, _ := pressed(t, screen, keyEnter)

	requireScreen(t, sending.View(), "posting…")

	if footer := footerLine(sending.View()); strings.Contains(footer, "esc") {
		t.Errorf("the footer offers leaving while posting: %q", footer)
	}

	requireScreen(t, press(t, sending, "esc", "e").View(), "┏━ Comment on")
}
