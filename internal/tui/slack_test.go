// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNotInChannel stands in for Slack refusing a post.
var errNotInChannel = errors.New("the credential was not accepted: not_in_channel")

// announcement is what the world's pull request is announced as.
const announcement = "jacob opened a pull request: <" + pullURL + "|" + pullTitle + ">\n" +
	"<https://jira.example.com/browse/PROJ-412|PROJ-412> " + issueSummary

func TestTheSlackPanePreviewsTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "jacob opened a pull request:", "to     "+slackChannel, "CI     ● passed",
		"state  ○ nothing posted")
	requireScreen(t, footerLine(view), "p post to slack")
}

func TestTheSlackPaneAsksForAPullRequestFirst(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withoutPull().live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "Open a pull request first (4 Review)")
	refuseScreen(t, footerLine(view), "p post")
}

func TestPostingNowPostsTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	posting := newWorld()
	model := posting.live(t, 120, 40)

	// Act: open the post
	preview := typing(t, model, "5", "p")

	// Assert: it says where it goes and when
	requireScreen(t, preview.View().Content,
		"┏━ Post to Slack", "to  "+slackChannel, "enter post now", "w post when CI passes")

	// Act: post now
	posted := typing(t, preview, keyEnter)

	// Assert: the announcement is posted once
	requireScreen(t, posted.View().Content,
		"● posted to "+slackChannel)

	if calls := posting.asked("post "); len(calls) != 1 || calls[0] != "post "+announcement {
		t.Errorf("post calls = %q, want the announcement", calls)
	}

	// Act: move on from the notice
	pane := typing(t, posted, "j").View().Content

	// Assert: the pane says it was posted, and does not offer it again
	requireScreen(t, pane, "state  ● posted")
	refuseScreen(t, footerLine(pane), "p post")
}

func TestTheMessageCanBeEditedBeforeItPosts(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	editing.edited = "Ready for review: " + pullURL
	model := editing.live(t, 120, 40)

	// Act: edit the message
	edited := typing(t, model, "5", "p", "e")

	// Assert: the preview is the edited message
	requireScreen(t, edited.View().Content,
		"Ready for review: "+pullURL)

	// Act: post it
	typing(t, edited, keyEnter)

	// Assert: the edited message is what posted
	if calls := editing.asked("post "); len(calls) != 1 || calls[0] != "post Ready for review: "+pullURL {
		t.Errorf("post calls = %q, want the edited message", calls)
	}
}

func TestAnEmptiedMessageIsNotPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	emptied := newWorld()
	emptied.edited = "  "

	// Act
	view := typing(t, emptied.live(t, 120, 40), "5", "p", "e", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "nothing to post: the message was empty")

	if calls := emptied.asked("post "); len(calls) != 0 {
		t.Errorf("an empty message posted: %q", calls)
	}
}

func TestAFailedEditOfTheMessageSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	failedEdit := newWorld()
	failedEdit.editErr = errEditorFailed

	// Act
	view := typing(t, failedEdit.live(t, 120, 40), "5", "p", "e").View().Content

	// Assert
	requireScreen(t, view, "✗ the editor exited with an error")
}

func TestARefusedPostSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := newWorld()
	refusing.postErr = errNotInChannel
	model := refusing.live(t, 120, 40)

	// Act: post, and be refused
	refused := typing(t, model, "5", "p", keyEnter)

	// Assert: the post stays open with the reason
	requireScreen(t, refused.View().Content,
		"┏━ Post to Slack", "✗ the credential was not accepted: not_in_channel")

	// Act: close it
	closed := typing(t, refused, keyEsc)

	// Assert: the pane keeps the reason
	requireScreen(t, closed.View().Content,
		"state  ✗ the credential was not accepted")
}

func TestPostingWhenCIPassesWaitsForIt(t *testing.T) {
	t.Parallel()

	// Arrange
	waiting := newWorld()
	waiting.ci = []forge.CI{{State: forge.CIRunning}}

	// Act
	pending := typing(t, waiting.live(t, 120, 40), "5", "p", "w")

	// Assert
	requireScreen(t, pending.View().Content,
		"◐ will post to "+slackChannel+" once CI passes", "◐ Slack")

	if calls := waiting.asked("post "); len(calls) != 0 {
		t.Errorf("posted before CI passed: %q", calls)
	}
}

func TestAPostWaitingForCIGoesOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	passing := newWorld()
	passing.ciInterval = time.Millisecond
	passing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	// Act
	posted := typing(t, passing.live(t, 120, 40), "5", "p", "w")

	// Assert
	requireScreen(t, posted.View().Content,
		"● posted to "+slackChannel)

	if calls := passing.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want one once CI passed", calls)
	}
}

func TestAPostWaitingForCIIsDroppedWhenCIFails(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.ciInterval = time.Millisecond
	failing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIFailed}}

	// Act
	dropped := typing(t, failing.live(t, 120, 40), "5", "p", "w")

	// Assert
	requireScreen(t, dropped.View().Content,
		"✗ CI failed, so nothing was posted to Slack")

	if calls := failing.asked("post "); len(calls) != 0 {
		t.Errorf("posted though CI failed: %q", calls)
	}
}

func TestPostingNowReplacesThePostWaitingForCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// Running at start, running when w is pressed, passed when asked again.
	impatient := newWorld()
	impatient.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}
	waiting := typing(t, impatient.live(t, 120, 40), "5", "p", "w")

	// Act: post now instead
	postedNow := typing(t, waiting, "p", keyEnter)

	// Assert: it posted
	requireScreen(t, postedNow.View().Content,
		"● posted to "+slackChannel)

	if calls := impatient.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the one posted now", calls)
	}

	// Act: CI passes
	refreshed := typing(t, postedNow, "4", "r")

	// Assert: the waiting post does not go as well
	requireScreen(t, refreshed.View().Content,
		"● passed")

	if calls := impatient.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want no other once CI passed", calls)
	}
}

func TestNothingMorePostsWhileAPostIsOnItsWay(t *testing.T) {
	t.Parallel()

	// Arrange
	slow := newWorld()
	slow.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	slow.postGate = make(chan struct{})

	t.Cleanup(func() { close(slow.postGate) })

	model := slow.live(t, 120, 40)

	// Act: post when CI passes, which it has by the time w is pressed
	sending := typing(t, model, "5", "p", "w")

	// Assert: the post went, Slack has not answered, and it is not offered again
	requireScreen(t, sending.View().Content,
		"state  ◐ posting")
	refuseScreen(t, footerLine(sending.View().Content), "p post")

	if calls := slow.asked("post "); len(calls) != 1 {
		t.Fatalf("post calls = %q, want the one on its way", calls)
	}

	// Act: try to post again
	again := typing(t, sending, "p", keyEnter)

	// Assert: nothing opens, and nothing more is sent
	refuseScreen(t, again.View().Content, "Post to Slack")

	if calls := slow.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want only the one already on its way", calls)
	}
}

func TestPostingWhenCIHasAlreadyPassedPostsNow(t *testing.T) {
	t.Parallel()

	// Arrange
	passed := newWorld()

	// Act
	view := typing(t, passed.live(t, 120, 40), "5", "p", "w").View().Content

	// Assert
	requireScreen(t, view, "● posted to "+slackChannel)

	if calls := passed.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want one", calls)
	}
}

func TestADryRunPostsNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		key  string
		want string
	}{
		"posting now":            {key: keyEnter, want: "dry run: would post to " + slackChannel},
		"posting when CI passes": {key: "w", want: "dry run: would post to " + slackChannel + " once CI passes"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dry := newWorld()
			dry.ci = []forge.CI{{State: forge.CIRunning}}
			model := sized(t, dryInterface(dry), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, "5", "p", tt.key).View().Content

			// Assert
			requireScreen(t, view, tt.want)

			if calls := dry.asked("post "); len(calls) != 0 {
				t.Errorf("a dry run posted: %q", calls)
			}
		})
	}
}

func TestEscIsLabeledDiscardOnlyWhereTextIsLost(t *testing.T) {
	t.Parallel()

	const escDiscard = "esc discard"

	cases := map[string]struct {
		world *world
		keys  []string
		want  string
	}{
		"pull request composer discards":  {world: withoutPull(), keys: []string{"4", "n"}, want: escDiscard},
		"slack preview discards":          {world: newWorld(), keys: []string{"5", "p"}, want: escDiscard},
		"branch overlay discards":         {world: newWorld(), keys: []string{"2", "b"}, want: escDiscard},
		"commit composer keeps its draft": {world: newWorld(), keys: []string{"3", "c"}, want: "esc close"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := tt.world.live(t, 120, 40)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			requireScreen(t, footerLine(view), tt.want)
		})
	}
}

func TestTheSlackPaneNamesWhatItNeedsWhenUnset(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Slack = config.Slack{}
	model := sized(t, tui.New(cfg, nil, newWorld().deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "5").View().Content

	// Assert
	requireScreen(t, view, "Slack is not set up", "slack.webhook_url", "slack.token and slack.channel", ".workflow.json")
	refuseScreen(t, footerLine(view), "p post")
}

func TestADroppedPostLeavesALineInThePane(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.ciInterval = time.Millisecond
	failing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIFailed}}

	// Act
	dropped := typing(t, failing.live(t, 120, 40), "5", "p", "w")
	pane := typing(t, dropped, "j").View().Content

	// Assert
	requireScreen(t, pane, "not posted: CI failed at 16:00")
}

func TestQuittingWithAQueuedPostAsksFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	waiting := newWorld()
	waiting.ci = []forge.CI{{State: forge.CIRunning}}
	queued := typing(t, waiting.live(t, 120, 40), "5", "p", "w")

	// Act
	asked := typing(t, queued, "q")

	// Assert
	requireScreen(t, asked.View().Content,
		"A post is waiting for CI and will be lost")
	requireScreen(t, footerLine(asked.View().Content), "enter quit", "esc stay")
}
