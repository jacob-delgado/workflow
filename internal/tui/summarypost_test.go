// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// tuesdayHeading is the Summary's first line for the period it opens on, and
// tuesdayBold the same as Slack shows it.
const (
	tuesdayHeading = "# 2026-09-15"
	tuesdayBold    = "*2026-09-15*"
	postSummaryKey = "p"
)

func TestTheSummaryIsPostedOnlyOnceItsPreviewIsConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	model := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act: open the preview
	preview := typing(t, model, postSummaryKey)

	// Assert: it shows the summary and where it goes, and nothing is posted yet
	requireScreen(t, preview.View().Content, "┏━ Post to Slack", tuesdayHeading, "committed abc1234",
		"to  "+devChannel, "enter post", "e edit", "esc discard")

	if calls := busy.asked("post "); len(calls) != 0 {
		t.Errorf("posted before the preview was confirmed: %q", calls)
	}

	// Act: post it
	posted := typing(t, preview, keyEnter)

	// Assert: it is posted once, as Slack shows it, and said so
	requireScreen(t, posted.View().Content, "● posted to "+devChannel)

	calls := busy.asked("post ")
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "post "+tuesdayBold) || busy.channelPostedTo() != devChannel {
		t.Errorf("posted %q to %q, want the summary once, headed as Slack shows it, to %s",
			calls, busy.channelPostedTo(), devChannel)
	}
}

func TestTheSummarysPreviewIsDiscardedUnposted(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Post to Slack")

	if calls := busy.asked("post "); len(calls) != 0 {
		t.Errorf("a discarded preview posted: %q", calls)
	}
}

func TestTheSummaryPostsToTheChannelCycledTo(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.cfg.Messaging.Channels = []string{teamChannel}

	// Act: cycle to the next channel
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, "right")

	// Assert: the preview names it
	requireScreen(t, view.View().Content, "to  "+teamChannel, "change channel")

	// Act: post
	typing(t, view, keyEnter)

	// Assert: it went there
	if got := busy.channelPostedTo(); got != teamChannel {
		t.Errorf("posted to %q, want the channel cycled to, %s", got, teamChannel)
	}
}

func TestTheSummaryIsPostedAsItWasEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.edited = "# Tuesday\n\n- shipped the fix"

	// Act
	edited := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, "e", keyEnter)

	// Assert
	if calls := busy.asked("post "); len(calls) != 1 || calls[0] != "post *Tuesday*\n\n• shipped the fix" {
		t.Errorf("posted %q, want the edit, as Slack shows it", calls)
	}

	if edits := busy.asked("edit "); len(edits) != 1 || !strings.HasPrefix(edits[0], "edit "+tuesdayHeading) {
		t.Errorf("the editor was handed %q, want the summary's Markdown", edits)
	}

	requireScreen(t, edited.View().Content, "● posted to "+devChannel)
}

func TestASummaryEditedToNothingIsNotPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.edited = " \n"

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, "e", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "nothing to post: the summary was empty")

	if calls := busy.asked("post "); len(calls) != 0 {
		t.Errorf("an emptied summary posted: %q", calls)
	}
}

func TestAFailedEditOfTheSummaryKeepsThePreviewWithWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.editErr = errEditorFailed

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, "e").View().Content

	// Assert
	requireScreen(t, view, "┏━ Post to Slack", "✗ the editor exited with an error")
}

func TestAWebhookOnlySummaryPostNamesTheWebhooksChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.cfg.Messaging = teamsMessaging()

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey).View().Content

	// Assert
	requireScreen(t, view, "┏━ Post to Teams", "to  the channel its webhook is bound to")
	refuseScreen(t, footerLine(view), "change channel")
}

func TestARefusedSummaryPostKeepsThePreviewWithWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.postErr = errNotInChannel

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "┏━ Post to Slack", "✗ the credential was not accepted: not_in_channel")
}

func TestADryRunSummaryPostSaysWhereItWouldGoAndPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	model := sized(t, tui.New(busy.cfg, nil, busy.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, model, summaryKey, postSummaryKey, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would post to "+devChannel)

	if calls := busy.asked("post "); len(calls) != 0 {
		t.Errorf("a dry run posted: %q", calls)
	}
}

func TestTheSummaryOffersNoPostWithoutMessaging(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.cfg.Messaging = config.Messaging{}

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey).View().Content

	// Assert
	refuseScreen(t, footerLine(view), "p post")
	refuseScreen(t, view, "Post to")
}

func TestTheSummaryOffersItsPostOnceEverySourceHasAnswered(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, summaryWorld().live(t, 120, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, footerLine(view), "p post")
}

func TestTheSummaryOffersNoPostWhileASourceIsStillReading(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pane is opened and its reads begun, but none has answered yet.
	opened, _ := summaryWorld().live(t, 120, 40).Update(keyMsg(summaryKey))
	reading := concrete(t, opened)

	// Act
	view := typing(t, reading, postSummaryKey).View().Content

	// Assert
	refuseScreen(t, footerLine(view), "p post")
	refuseScreen(t, view, "Post to")
}

func TestTheSummaryOffersNoPostWithNothingToPostThrough(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	deps := busy.deps()
	deps.Messaging.Post = nil

	// Act
	view := typing(t, sized(t, tui.New(busy.cfg, nil, deps), 120, 40), summaryKey, postSummaryKey).View().Content

	// Assert
	refuseScreen(t, footerLine(view), "p post")
	refuseScreen(t, view, "Post to")
}

func TestNothingMoreIsSentWhileTheSummaryIsOnItsWay(t *testing.T) {
	t.Parallel()

	// Arrange
	// enter sends the post, which has not been made when the keys after it come.
	busy := summaryWorld()
	sending, _ := pressed(t, typing(t, busy.live(t, 120, 40), summaryKey, postSummaryKey), keyEnter)

	// Act
	view := typing(t, sending, keyEnter, "e", keyEsc).View().Content

	// Assert
	requireScreen(t, view, "┏━ Post to Slack", "posting")
	refuseScreen(t, footerLine(view), "enter post")

	if calls := busy.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q while the first post was on its way, want nothing more sent", calls)
	}
}

func TestTheSummaryPreviewHasNoEditWithoutAnEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	deps := busy.deps()
	deps.Editor.Edit = nil
	previewing := typing(t, sized(t, tui.New(busy.cfg, nil, deps), 120, 40), summaryKey, postSummaryKey)

	// Act
	view := typing(t, previewing, "e", "left", "right").View().Content

	// Assert
	requireScreen(t, view, "┏━ Post to Slack", "to  "+devChannel)

	if edits := busy.asked("edit "); len(edits) != 0 {
		t.Errorf("handed %q to an editor there is none of", edits)
	}
}
