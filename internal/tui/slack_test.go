// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errNotInChannel stands in for Slack refusing a post.
var errNotInChannel = errors.New("the credential was not accepted: not_in_channel")

// announcement is what the world's pull request is announced as.
const announcement = "jacob opened a pull request: <" + pullURL + "|" + pullTitle + ">\n" +
	"<https://jira.example.com/browse/PROJ-412|PROJ-412> " + issueSummary

func TestTheSlackPanePreviewsTheAnnouncement(t *testing.T) {
	t.Parallel()

	view := typing(t, newWorld().live(t, 120, 40), "5").View()
	requireScreen(t, view, "jacob opened a pull request:", "to     "+slackChannel, "CI     ● passed",
		"state  ○ nothing posted")
	requireScreen(t, footerLine(view), "p post to slack")

	none := withoutPull()
	view = typing(t, none.live(t, 120, 40), "5").View()
	requireScreen(t, view, "Open a pull request first (4 Review)")
	refuseScreen(t, footerLine(view), "p post")
}

func TestPostingNowPostsTheAnnouncement(t *testing.T) {
	t.Parallel()

	posting := newWorld()

	preview := typing(t, posting.live(t, 120, 40), "5", "p")
	requireScreen(t, preview.View(), "┏━ Post to Slack", "to  "+slackChannel, "enter post now", "w post when CI passes")

	posted := typing(t, preview, keyEnter)
	requireScreen(t, posted.View(), "● posted to "+slackChannel)
	requireScreen(t, typing(t, posted, "j").View(), "state  ● posted")

	if calls := posting.asked("post "); len(calls) != 1 || calls[0] != "post "+announcement {
		t.Errorf("post calls = %q, want the announcement", calls)
	}

	// Posted once, it is not offered again.
	refuseScreen(t, footerLine(typing(t, posted, "j").View()), "p post")
}

func TestTheMessageCanBeEditedBeforeItPosts(t *testing.T) {
	t.Parallel()

	editing := newWorld()
	editing.edited = "Ready for review: " + pullURL

	edited := typing(t, editing.live(t, 120, 40), "5", "p", "e")
	requireScreen(t, edited.View(), "Ready for review: "+pullURL)

	typing(t, edited, keyEnter)

	if calls := editing.asked("post "); len(calls) != 1 || calls[0] != "post Ready for review: "+pullURL {
		t.Errorf("post calls = %q", calls)
	}

	emptied := newWorld()
	emptied.edited = "  "
	requireScreen(t, typing(t, emptied.live(t, 120, 40), "5", "p", "e", keyEnter).View(),
		"nothing to post: the message was empty")

	failedEdit := newWorld()
	failedEdit.editErr = errEditorFailed
	requireScreen(t, typing(t, failedEdit.live(t, 120, 40), "5", "p", "e").View(), "✗ the editor exited with an error")
}

func TestARefusedPostSaysWhy(t *testing.T) {
	t.Parallel()

	refusing := newWorld()
	refusing.postErr = errNotInChannel

	refused := typing(t, refusing.live(t, 120, 40), "5", "p", keyEnter)
	requireScreen(t, refused.View(), "┏━ Post to Slack", "✗ the credential was not accepted: not_in_channel")
	requireScreen(t, typing(t, refused, "esc").View(), "state  ✗ the credential was not accepted")
}

func TestPostingWhenCIPassesWaitsForIt(t *testing.T) {
	t.Parallel()

	waiting := newWorld()
	waiting.ci = []forge.CI{{State: forge.CIRunning}}

	pending := typing(t, waiting.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, pending.View(), "◐ will post to "+slackChannel+" once CI passes", "◐ Slack")

	if calls := waiting.asked("post "); len(calls) != 0 {
		t.Fatalf("posted before CI passed: %q", calls)
	}

	passing := newWorld()
	passing.ciInterval = time.Millisecond
	passing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	posted := typing(t, passing.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, posted.View(), "● posted to "+slackChannel)

	if calls := passing.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want one once CI passed", calls)
	}
}

func TestAPostWaitingForCIIsDroppedWhenCIFails(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.ciInterval = time.Millisecond
	failing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIFailed}}

	dropped := typing(t, failing.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, dropped.View(), "✗ CI failed, so nothing was posted to Slack")

	if calls := failing.asked("post "); len(calls) != 0 {
		t.Errorf("posted though CI failed: %q", calls)
	}
}

func TestPostingNowReplacesThePostWaitingForCI(t *testing.T) {
	t.Parallel()

	// Running at start, running when w is pressed, passed when asked again.
	impatient := newWorld()
	impatient.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	waiting := typing(t, impatient.live(t, 120, 40), "5", "p", "w")
	postedNow := typing(t, waiting, "p", keyEnter)
	requireScreen(t, postedNow.View(), "● posted to "+slackChannel)

	requireScreen(t, typing(t, postedNow, "4", "r").View(), "● passed")

	if calls := impatient.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the one posted now and no other once CI passed", calls)
	}
}

func TestNothingMorePostsWhileAPostIsOnItsWay(t *testing.T) {
	t.Parallel()

	slow := newWorld()
	slow.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	slow.postGate = make(chan struct{})

	t.Cleanup(func() { close(slow.postGate) })

	// CI has passed by the time w is pressed, so the post goes, and Slack has
	// not answered yet.
	sending := typing(t, slow.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, sending.View(), "state  ◐ posting")
	refuseScreen(t, footerLine(sending.View()), "p post")
	refuseScreen(t, typing(t, sending, "p").View(), "Post to Slack")
}

func TestPostingWhenCIHasAlreadyPassedPostsNow(t *testing.T) {
	t.Parallel()

	passed := newWorld()
	requireScreen(t, typing(t, passed.live(t, 120, 40), "5", "p", "w").View(), "● posted to "+slackChannel)
}

func TestADryRunPostsNothing(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.ci = []forge.CI{{State: forge.CIRunning}}

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "5", "p", keyEnter).View(), "dry run: would post to "+slackChannel)
	requireScreen(t, typing(t, model, "5", "p", "w").View(), "dry run: would post to "+slackChannel+" once CI passes")

	if calls := dry.asked("post "); len(calls) != 0 {
		t.Errorf("a dry run posted: %q", calls)
	}
}
