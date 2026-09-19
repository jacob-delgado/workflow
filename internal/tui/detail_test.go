// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestCollapsedIssuesEnterOpensTheSelectedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 79, 24)

	// Act
	view := typing(t, model, keyEnter).View()

	// Assert
	requireScreen(t, view, "Tokens reach the log.")
}

func TestCollapsedIssuesShowTheSetupStepWhenNothingToChoose(t *testing.T) {
	t.Parallel()

	// Arrange
	model := sized(t, tui.New(config.Config{}, config.ErrNotFound, tui.Deps{}), 79, 24)

	// Act
	view := model.View()

	// Assert
	requireScreen(t, view, "config init")
}

// Errors the fakes answer with.
var (
	errNotVisible   = errors.New("jira rejected the request: Issue Does Not Exist")
	errEditorFailed = errors.New("the editor exited with an error")
)

func TestTheDetailShowsTheIssueInFull(t *testing.T) {
	t.Parallel()

	// Act
	view := newWorld().live(t, 120, 40).View()

	// Assert
	requireScreen(t, view, issueKey+" "+issueSummary, "Bug · In Progress", "reported by Ana Lopez",
		"Tokens reach the log.", "Comments 1 of 1", "Ana Lopez · 2h ago", "Repro'd on 8.2.1")
}

func TestCommentsShownLimitsHowManyTheDetailDraws(t *testing.T) {
	t.Parallel()

	// Arrange
	fewer := newWorld()
	fewer.cfg.UI.CommentsShown = 2
	fewer.detail.Comments = []jira.Comment{
		{Author: reporter, Body: "oldest", Created: testNow().Add(-3 * time.Hour)},
		{Author: reporter, Body: "middle", Created: testNow().Add(-2 * time.Hour)},
		{Author: reporter, Body: "newest", Created: testNow().Add(-1 * time.Hour)},
	}
	fewer.detail.CommentTotal = 3

	// Act
	view := fewer.live(t, 120, 40).View()

	// Assert
	// Only the two most recent are drawn, and the heading says so.
	requireScreen(t, view, "Comments 2 of 3", "middle", "newest")
	refuseScreen(t, view, "oldest")
}

func TestTheDetailSaysWhatIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	quiet := newWorld()
	quiet.detail = jira.IssueDetail{Issue: jira.Issue{Key: issueKey}, Reporter: reporter, CommentTotal: 0}

	// Act
	view := quiet.live(t, 120, 40).View()

	// Assert
	requireScreen(t, view, "no description")
	refuseScreen(t, view, "Comments")
}

func TestADetailThatCannotBeReadSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.detailErr = errNotVisible

	// Act
	view := failing.live(t, 120, 40).View()

	// Assert
	requireScreen(t, view, "✗ jira rejected the request: Issue Does Not Exist")
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

			// Arrange
			aged := newWorld()
			aged.detail.Comments[0].Created = tt.created

			// Act
			view := aged.live(t, 120, 40).View()

			// Assert
			requireScreen(t, view, "Ana Lopez · "+tt.want)
		})
	}
}

func TestTheDetailFollowsTheSelectionOnceItRests(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	screen := moving.live(t, 120, 40)
	moving.detail = jira.IssueDetail{Issue: jira.Issue{Key: secondIssue}, Reporter: "Fred", Description: "Retries."}

	// Act: select the next issue
	updated, rest := screen.Update(keyMsg("j"))
	moved := concrete(t, updated)

	// Assert: the header is the new issue, and the full read is on its way
	requireScreen(t, moved.View(), "PROJ-388 Add retries", "loading the description and comments…")

	// Act: the selection rests
	rested := drain(t, moved, rest)

	// Assert: the new issue is read once, and shown
	requireScreen(t, rested.View(), "Retries.", "reported by Fred")

	if reads := moving.asked("issue PROJ-388"); len(reads) != 1 {
		t.Errorf("read PROJ-388 %d times, want once", len(reads))
	}
}

func TestADetailForAnIssueNoLongerSelectedIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newWorld().deps()
	deps.Jira.Issue = func(key jira.Key) (jira.IssueDetail, error) {
		if key == secondIssue {
			return jira.IssueDetail{Issue: jira.Issue{Key: key}, Reporter: "Fred", Description: "Retries."}, nil
		}

		return newWorld().detail, nil
	}

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	screen := drain(t, model, model.Init())

	// Act
	// PROJ-388 is read, then the selection moves back to PROJ-412 and that is
	// read too; PROJ-388's answer arrives last.
	away, restAway := pressed(t, screen, "j")
	readingAway, readAway := finish(t, away, restAway)
	back, restBack := pressed(t, readingAway, "k")
	readingBack, readBack := finish(t, back, restBack)
	current, _ := finish(t, readingBack, readBack)
	settled, _ := finish(t, current, readAway)

	// Assert
	requireScreen(t, settled.View(), issueKey+" "+issueSummary, "Tokens reach the log.")
	refuseScreen(t, settled.View(), "Retries.", "reported by Fred")
}

func TestARestForAnIssueNoLongerSelectedReadsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	restless := newWorld()
	restless.issues = append(restless.issues, jira.Issue{Key: "PROJ-500", Summary: "Trim logs", StatusCategory: "new"})
	screen := restless.live(t, 120, 40)
	passing, restOnSecond := pressed(t, screen, "j")
	third, _ := pressed(t, passing, "j")

	// Act
	// The selection has moved on to the third issue when the second's rest ends.
	_, read := finish(t, third, restOnSecond)

	// Assert
	// Holding j down reads nothing until the selection rests where it stops.
	if read != nil {
		t.Errorf("a rest for an issue the selection had left started a read")
	}

	if reads := append(restless.asked("issue "+secondIssue), restless.asked("issue PROJ-500")...); len(reads) != 0 {
		t.Errorf("read %q before the selection rested on it", reads)
	}
}

func TestMovingBackToTheIssueShownWaitsForNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	screen := newWorld().live(t, 120, 40)
	away, _ := pressed(t, screen, "j")

	// Act
	back, rest := pressed(t, away, "k")

	// Assert
	if rest != nil {
		t.Error("moving back to the issue already shown waited to read it again")
	}

	requireScreen(t, back.View(), issueKey+" "+issueSummary, "Tokens reach the log.")
}

func TestWorkInProgressIsWhereTheInterfaceOpens(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch string
		want   string
	}{
		"a branch naming an issue in the list": {branch: "feat/PROJ-388-add-retries", want: "▸ ○ PROJ-388 Add retries"},
		// A branch naming an issue not in the list leaves the selection alone.
		"a branch naming an issue not in the list": {branch: "fix/OTHER-1-thing", want: "▸ ◐ PROJ-412"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			resuming := newWorld()
			resuming.branch.Name = tt.branch

			// Act
			view := resuming.live(t, 120, 40).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestResumingNeverOverridesAChoice(t *testing.T) {
	t.Parallel()

	// Arrange
	// No branch names an issue at start; the user chooses one; then the branch
	// is read again and turns out to name a different issue.
	chosen := newWorld()
	chosen.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	screen := typing(t, chosen.live(t, 120, 40), "j")
	chosen.branch = gitrepo.Branch{Name: featureName, Base: baseRef}

	// Act
	reloaded := typing(t, screen, "2", "r", "1").View()

	// Assert
	// The new branch is shown, so it was read; the choice stands regardless.
	requireScreen(t, reloaded, featureName, "▸ ○ PROJ-388")
}

func TestCommentingPreviewsBeforeItPosts(t *testing.T) {
	t.Parallel()

	// Arrange
	commenting := newWorld()
	commenting.edited = "Patch up shortly"
	model := commenting.live(t, 120, 40)

	// Act: write the comment
	screen := typing(t, model, "c")

	// Assert: it is previewed, and not yet posted
	requireScreen(t, screen.View(), "┏━ Comment on PROJ-412", "Patch up shortly", "enter post", "e edit", "esc discard")

	if posted := commenting.asked("comment"); len(posted) != 0 {
		t.Errorf("posted before the preview was confirmed: %q", posted)
	}

	// Act: post it
	posted := typing(t, screen, keyEnter)

	// Assert: it posted once, the preview closed, and the issue was read again
	requireScreen(t, posted.View(), "● commented on PROJ-412")
	refuseScreen(t, posted.View(), "┏━ Comment on")

	if calls := commenting.asked("comment PROJ-412 Patch up shortly"); len(calls) != 1 {
		t.Errorf("comment calls = %q, want one", commenting.asked("comment"))
	}

	// The second read is what shows the new comment.
	if reads := commenting.asked("issue " + issueKey); len(reads) != 2 {
		t.Errorf("read the issue %d times, want once at start and again after commenting", len(reads))
	}
}

func TestACommentCanBeEditedAgainOrDiscarded(t *testing.T) {
	t.Parallel()

	// Arrange
	commenting := newWorld()
	commenting.edited = "first draft"
	screen := typing(t, commenting.live(t, 120, 40), "c")
	commenting.edited = "second draft"

	// Act: edit it again
	edited := typing(t, screen, "e")

	// Assert: the second edit started from the first draft
	requireScreen(t, edited.View(), "second draft")

	if edits := commenting.asked("edit first draft"); len(edits) != 1 {
		t.Errorf("the second edit did not start from the first draft: %q", commenting.asked("edit"))
	}

	// Act: discard it
	discarded := typing(t, edited, keyEsc)

	// Assert: nothing posted
	requireScreen(t, discarded.View(), "comment discarded")

	if posted := commenting.asked("comment"); len(posted) != 0 {
		t.Errorf("a discarded comment was posted: %q", posted)
	}
}

func TestACommentThatCannotBePostedSaysWhy(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		edited     string
		editErr    error
		commentErr error
		keys       []string
		want       string
	}{
		"empty":         {edited: "   ", keys: []string{"c"}, want: "nothing to post: the comment was empty"},
		"editor failed": {editErr: errEditorFailed, keys: []string{"c"}, want: "✗ the editor exited with an error"},
		// Refused, the preview stays open with Jira's reason.
		"refused": {
			edited: greeting, commentErr: errNotVisible, keys: []string{"c", keyEnter},
			want: "✗ jira rejected the request",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failing := newWorld()
			failing.edited, failing.editErr, failing.commentErr = tt.edited, tt.editErr, tt.commentErr

			// Act
			view := typing(t, failing.live(t, 120, 40), tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestNothingInterruptsACommentBeingPosted(t *testing.T) {
	t.Parallel()

	for _, key := range []string{keyEsc, "e"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			commenting := newWorld()
			commenting.edited = "hello"
			sending, _ := pressed(t, typing(t, commenting.live(t, 120, 40), "c"), keyEnter)

			// Act
			after, cmd := pressed(t, sending, key)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command while the comment is posted", key)
			}

			if after.View() != sending.View() {
				t.Errorf("%s changed the screen while the comment is posted:\n%s", key, after.View())
			}
		})
	}
}

func TestTheFooterOffersNoWayOutWhileACommentIsPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	commenting := newWorld()
	commenting.edited = "hello"
	screen := typing(t, commenting.live(t, 120, 40), "c")

	// Act
	sending, _ := pressed(t, screen, keyEnter)

	// Assert
	requireScreen(t, sending.View(), "posting…")
	refuseScreen(t, footerLine(sending.View()), keyEsc)
}

func TestAFailedReEditKeepsThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	commenting := newWorld()
	commenting.edited = "keep me"
	screen := typing(t, commenting.live(t, 120, 40), "c")
	commenting.edited, commenting.editErr = "lost in the editor", errEditorFailed

	// Act
	failed := typing(t, screen, "e")

	// Assert
	requireScreen(t, failed.View(), "┏━ Comment on PROJ-412", "keep me", "✗ the editor exited with an error")
	refuseScreen(t, failed.View(), "lost in the editor")
}

func TestRRetriesAFailedDetailLoad(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.detailErr = errNotVisible
	model := failing.live(t, 120, 40)
	before := len(failing.asked("issue " + issueKey))

	// Act
	typing(t, model, "r")

	// Assert
	if after := len(failing.asked("issue " + issueKey)); after != before+1 {
		t.Errorf("asked Jira for the issue %d times, want one more than the %d before r", after, before)
	}
}
