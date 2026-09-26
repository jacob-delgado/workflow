// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNotInChannel stands in for Slack refusing a post.
var errNotInChannel = errors.New("the credential was not accepted: not_in_channel")

// announcement is what the world's pull request is announced as, and
// mergedAnnouncement and redCIAnnouncement are the same pull at its other
// moments: the issue line is shared, only the lead sentence changes.
const (
	announcement = "jacob opened a pull request: <" + pullURL + "|" + pullTitle + ">\n" +
		"<https://jira.example.com/browse/PROJ-412|PROJ-412> " + issueSummary
	mergedAnnouncement = "jacob merged a pull request: <" + pullURL + "|" + pullTitle + ">\n" +
		"<https://jira.example.com/browse/PROJ-412|PROJ-412> " + issueSummary
	redCIAnnouncement = "CI is red on the pull request: <" + pullURL + "|" + pullTitle + ">\n" +
		"<https://jira.example.com/browse/PROJ-412|PROJ-412> " + issueSummary
)

func TestTheSlackPaneAnnouncesAMerge(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act: open the post
	preview := typing(t, model, "5", "p")

	// Assert: it is a merge announcement, with no CI to wait on
	requireScreen(t, preview.View().Content, "jacob merged a pull request", "enter announce now")
	refuseScreen(t, footerLine(preview.View().Content), "when CI passes")

	// Act: post it
	posted := typing(t, preview, keyEnter)

	// Assert: the merge announcement is posted once
	requireScreen(t, posted.View().Content, "● announced to "+slackChannel)

	if calls := merged.asked("post "); len(calls) != 1 || calls[0] != "post "+mergedAnnouncement {
		t.Errorf("post calls = %q, want the merge announcement", calls)
	}
}

func TestTheSlackPaneAnnouncesRedCI(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	model := failing.live(t, 120, 40)

	// Act
	preview := typing(t, model, "5", "p")

	// Assert
	requireScreen(t, preview.View().Content, "CI is red on the pull request", "enter announce now")
	refuseScreen(t, footerLine(preview.View().Content), "when CI passes")
}

func TestAMergeIsAnnouncedOnlyOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act: post the merge
	posted := typing(t, model, "5", "p", keyEnter)

	// Assert: it is posted once
	if calls := merged.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the merge posted once", calls)
	}

	// Act: move on from the notice, then try to post the merge again
	after := typing(t, posted, "j", "p", keyEnter)

	// Assert: the merge is neither re-offered nor posted a second time
	requireScreen(t, after.View().Content, "state  ● announced")
	refuseScreen(t, footerLine(after.View().Content), "p announce")

	if calls := merged.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the merge announced once, not re-offered", calls)
	}
}

func TestPostWhenCIPassesIsInertOnAMerge(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merge has no CI to wait for, so w does nothing rather than queue the post.
	merged := mergedBranch()

	// Act
	after := typing(t, merged.live(t, 120, 40), "5", "p", "w")

	// Assert
	refuseScreen(t, after.View().Content, "once CI passes")

	if calls := merged.asked("post "); len(calls) != 0 {
		t.Errorf("a merge queued or sent a post waiting for CI: %q", calls)
	}
}

func TestAMergeIsAnnouncableAfterTheOpening(t *testing.T) {
	t.Parallel()

	// Arrange
	// The opening is announced; then the pull request merges under the reader.
	posting := newWorld()
	model := typing(t, posting.live(t, 120, 40), "5", "p", keyEnter)
	posting.pull.State = forge.StateMerged

	// Act
	afterMerge := typing(t, model, "4", "r", "5")

	// Assert
	// The merge can still be announced — the opening does not block it.
	requireScreen(t, afterMerge.View().Content, "jacob merged a pull request")
	requireScreen(t, footerLine(afterMerge.View().Content), "p announce to slack")
}

func TestTheSlackPanePreviewsTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "jacob opened a pull request:", "to     "+slackChannel, "CI     ● passed",
		"state  ○ nothing announced")
	requireScreen(t, footerLine(view), "p announce to slack")
}

// teamsMessaging is a Teams webhook, so every surface of it names Teams rather
// than a hardcoded Slack.
func teamsMessaging() config.Messaging {
	return config.Messaging{Kind: "teams", WebhookURL: "https://example.com/hook"}
}

func TestTheMessagingPaneNamesTheServiceInUse(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Teams webhook is configured, so the pane and its footer name Teams
	// rather than a hardcoded Slack.
	teams := newWorld()
	teams.cfg.Messaging = teamsMessaging()

	// Act
	view := typing(t, teams.live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "5 Teams")
	requireScreen(t, footerLine(view), "p announce to teams")
}

func TestTheHelpNamesTheMessagingService(t *testing.T) {
	t.Parallel()

	// Arrange
	teams := newWorld()
	teams.cfg.Messaging = teamsMessaging()

	// Act
	helpView := typing(t, teams.live(t, 120, 20), "?").View().Content

	// Assert
	requireScreen(t, helpView, "Review and Teams")
}

func TestTheSlackPaneAsksForAPullRequestFirst(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withoutPull().live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "Open a pull request first (4 Review)")
	refuseScreen(t, footerLine(view), "p announce")
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
		"┏━ Announce to Slack", "to  "+slackChannel, "enter announce now", "w when CI passes")

	// Act: post now
	posted := typing(t, preview, keyEnter)

	// Assert: the announcement is posted once
	requireScreen(t, posted.View().Content,
		"● announced to "+slackChannel)

	if calls := posting.asked("post "); len(calls) != 1 || calls[0] != "post "+announcement {
		t.Errorf("post calls = %q, want the announcement", calls)
	}

	// Act: move on from the notice
	pane := typing(t, posted, "j").View().Content

	// Assert: the pane says it was posted, and does not offer it again
	requireScreen(t, pane, "state  ● announced")
	refuseScreen(t, footerLine(pane), "p announce")
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
	requireScreen(t, view, "nothing to announce: the message was empty")

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
		"┏━ Announce to Slack", "✗ the credential was not accepted: not_in_channel")

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
		"◐ will announce to "+slackChannel+" once CI passes", "◐ Slack")

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
		"● announced to "+slackChannel)

	if calls := passing.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want one once CI passed", calls)
	}
}

func TestAPostWaitingForCIIsDroppedWhenCIFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI is running when the post is queued — a "ready for review" moment — and
	// fails on the next check. The interval is past the harness's horizon so the
	// initial poll does not fast-forward CI to failed before the post is queued.
	failing := newWorld()
	failing.ciInterval = 2 * time.Second
	failing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIFailed}}

	// Act
	dropped := typing(t, failing.live(t, 120, 40), "5", "p", "w")

	// Assert
	requireScreen(t, dropped.View().Content,
		"✗ CI failed, so nothing was announced to Slack")

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
		"● announced to "+slackChannel)

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
	slow.postParked, slow.postRelease = make(chan struct{}, 1), make(chan struct{})

	t.Cleanup(func() { close(slow.postRelease) })

	previewing := typing(t, slow.live(t, 120, 40), "5", "p")

	// Act: post when CI passes, which it has by the time w is pressed; the post
	// goes, and Slack has not answered when the screen is read
	checking, check := pressed(t, previewing, "w")
	sending, post := finish(t, checking, check)

	go post()

	select {
	case <-slow.postParked:
	case <-time.After(failsafe):
		t.Fatalf("the post had not reached Slack after %v: w did not send it", failsafe)
	}

	// Assert: the post went, Slack has not answered, and it is not offered again
	requireScreen(t, sending.View().Content,
		"state  ◐ announcing")
	refuseScreen(t, footerLine(sending.View().Content), "p announce")

	if calls := slow.asked("post "); len(calls) != 1 {
		t.Fatalf("post calls = %q, want the one on its way", calls)
	}

	// Act: try to post again
	again := typing(t, sending, "p", keyEnter)

	// Assert: nothing opens, and nothing more is sent
	refuseScreen(t, again.View().Content, "Announce to Slack")

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
	requireScreen(t, view, "● announced to "+slackChannel)

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
		"announcing now":            {key: keyEnter, want: "dry run: would announce to " + slackChannel},
		"announcing when CI passes": {key: "w", want: "dry run: would announce to " + slackChannel + " once CI passes"},
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
		"messaging preview discards":      {world: newWorld(), keys: []string{"5", "p"}, want: escDiscard},
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
	cfg.Messaging = config.Messaging{}
	model := sized(t, tui.New(cfg, nil, newWorld().deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "5").View().Content

	// Assert
	requireScreen(t, view, "Slack is not set up", "messaging.webhook_url",
		"messaging.token and", "messaging.channel", ".workflow.json")
	refuseScreen(t, footerLine(view), "p announce")
}

func TestADroppedPostLeavesALineInThePane(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI is running when the post is queued, then fails; the interval is past the
	// harness's horizon so the queue happens before CI settles.
	failing := newWorld()
	failing.ciInterval = 2 * time.Second
	failing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIFailed}}

	// Act
	dropped := typing(t, failing.live(t, 120, 40), "5", "p", "w")
	pane := typing(t, dropped, "j").View().Content

	// Assert
	requireScreen(t, pane, "not announced: CI failed at 16:00")
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
		"An announcement is waiting for CI and will be lost")
	requireScreen(t, footerLine(asked.View().Content), "enter quit", "esc stay")
}

func TestAPreviouslyAnnouncedPullOpensAsPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	// The store remembers pull request 42 was announced at its ready moment in an
	// earlier session, so the pane opens showing it posted, not offering it.
	announcing := newWorld()
	announcing.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}

	// Act
	view := typing(t, announcing.live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "state  ● announced")
}

func TestAnnouncingRemembersItInTheStore(t *testing.T) {
	t.Parallel()

	// Arrange
	announcing := newWorld()

	// Act
	// Post the ready-for-review announcement (moment 0).
	typing(t, announcing.live(t, 120, 40), "5", "p", keyEnter)

	// Assert
	if calls := announcing.asked("announce"); len(calls) != 1 || calls[0] != "announce 42 0" {
		t.Errorf("recorded announce = %q, want the pull and moment remembered", calls)
	}
}
