// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The last look at a switch or a reopen names the post waiting for CI among
// what leaving would lose. Once that post stops waiting, the look says what
// is no longer so: it closes, as the quit guard does, and the interface
// stays, saying how the post went.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// lostPost is how a switch's or a reopen's last look names the post waiting
// for CI.
const lostPost = "the announcement waiting for CI"

// savedNote is how the footer says a save kept for the next open, as esc on
// the reopen's last look says it.
const savedNote = "● saved " + savedSettingsPath + "; it applies once workflow reopens"

// wideForADrop is a terminal wide enough for the notice row to hold a dropped
// post's failure and the save beside it.
const wideForADrop = 220

// armedThen is repo's interface, 140 columns wide, with a post armed by w,
// whose read of CI is kept back as read, then keys typed.
func armedThen(t *testing.T, repo *world, keys ...string) (tui.Model, tea.Cmd) {
	t.Helper()

	return armedAt(t, repo, 140, keys...)
}

// armedAt is armedThen at width columns.
func armedAt(t *testing.T, repo *world, width int, keys ...string) (tui.Model, tea.Cmd) {
	t.Helper()

	armedPost, read := pressed(t, typing(t, repo.live(t, width, 40), "5", "p"), "w")

	return typing(t, armedPost, keys...), read
}

// requirePlainNotice fails the test unless the notice row, the one above the
// footer, shows want in no color: every color the row opens before it is
// reset first.
func requirePlainNotice(t *testing.T, view, want string) {
	t.Helper()

	lines := strings.Split(view, "\n")
	before, _, found := strings.Cut(lines[len(lines)-2], want)

	if opened, reset := sgrBalance(before); !found || opened != reset {
		t.Errorf("the notice does not show %q in no color:\n%q", want, lines[len(lines)-2])
	}
}

func TestTheSwitchLookClosesWhenTheWaitingPostWentOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withDirectories(newWorld())
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	looking, read := armedThen(t, repo, reposKey, "j", switchDirKey)
	requireScreen(t, looking.View().Content, "Switch to ~/src/web", lostPost)

	// Act
	went := drain(t, looking, read)

	// Assert
	refuseScreen(t, went.View().Content, "Switch to ~/src/web")
	requireScreen(t, went.View().Content, "announced to")

	if dir := went.Destination().Dir(); dir != "" {
		t.Errorf("switched to %q; want the interface to stay", dir)
	}
}

// reopenLookOverADrop is the reopen's last look over a post armed at width
// columns, whose pull request closed while its read of CI, kept back as
// read, was out.
func reopenLookOverADrop(t *testing.T, width int) (tui.Model, tea.Cmd) {
	t.Helper()

	repo := reposWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	changed, read := armedAt(t, repo, width, edited(projectRow, editedProject)...)
	looking, _ := pressedAndAnswered(t, changed, saveKey)
	requireScreen(t, looking.View().Content, "Reopen", lostPost)

	closePull(repo)

	return looking, read
}

func TestTheReopenLookClosesWhenTheWaitingPostIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	looking, read := reopenLookOverADrop(t, wideForADrop)

	// Act
	dropped := drain(t, looking, read)

	// Assert
	refuseScreen(t, dropped.View().Content, lostPost)
	requireFailureNotice(t, dropped.View().Content, strings.TrimPrefix(droppedPost, failGlyph+" ")+" · "+savedNote)

	if dir := dropped.Destination().Dir(); dir != "" {
		t.Errorf("reopened in %q; want the interface to stay", dir)
	}
}

func TestTheReopenLookClosesSayingHowThePostWentAndTheSave(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := reposWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	changed, read := armedThen(t, repo, edited(projectRow, editedProject)...)
	looking, _ := pressedAndAnswered(t, changed, saveKey)
	requireScreen(t, looking.View().Content, "Reopen", lostPost)

	// Act
	went := drain(t, looking, read).View().Content

	// Assert
	refuseScreen(t, went, lostPost)
	requireNotice(t, went, "● announced to "+slackChannel+" · "+savedNote)
}

func TestTheSaveSaidAfterADroppedPostIsNotRed(t *testing.T) {
	t.Parallel()

	// Arrange
	looking, read := reopenLookOverADrop(t, wideForADrop)

	// Act
	dropped := drain(t, looking, read).View().Content

	// Assert
	requirePlainNotice(t, dropped, savedNote)
}

func TestADroppedPostLeadsTheNoticeWhereTheSaveDoesNotFit(t *testing.T) {
	t.Parallel()

	// Arrange
	looking, read := reopenLookOverADrop(t, 100)

	// Act
	dropped := drain(t, looking, read).View().Content

	// Assert
	requireFailureNotice(t, dropped, strings.TrimPrefix(droppedPost, failGlyph+" ")+" · ")
}

func TestTheSaveIsSaidOnceThoughThePostIsTriedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := reposWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	repo.postErr = errAskedToWait
	changed, read := armedThen(t, repo, edited(projectRow, editedProject)...)
	looking, _ := pressedAndAnswered(t, changed, saveKey)
	asked := drain(t, looking, read)
	requireNotice(t, asked.View().Content, "trying again · "+savedNote)

	postsNow(repo)

	// Act
	posted := typing(t, asked, keyEsc, "5", "r").View().Content

	// Assert
	requireNotice(t, posted, "● announced to "+slackChannel)
	refuseScreen(t, posted, savedNote)
}

func TestTheSwitchLookStaysOpenWhileThePostStillWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withDirectories(newWorld())
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	looking, read := armedThen(t, repo, reposKey, "j", switchDirKey)

	// Act
	view := drain(t, looking, read).View().Content

	// Assert
	requireScreen(t, view, "Switch to ~/src/web", lostPost)
}

func TestAnotherLastLookStaysOpenWhenTheWaitingPostWentOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.branch.Ahead = 1
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	looking, read := armedThen(t, repo, "2", "P")

	// Act
	view := drain(t, looking, read).View().Content

	// Assert
	requireScreen(t, view, "Push "+featureName+" to origin?", "announced to")
}

func TestTheReopenLookStaysOpenWhenAPostAlreadyOutIsDelivered(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read w begins finds CI passed, and the post it sends is kept back,
	// so the settings are saved while the post is out and nothing waits.
	repo := reposWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	posting, post := withholding(t, armedPost, read, repo.counting("post "))
	looking, _ := pressedAndAnswered(t, typing(t, posting, edited(projectRow, editedProject)...), saveKey)
	requireScreen(t, looking.View().Content, "Reopen workflow here")

	// Act
	view := handedOver(t, looking, post).View().Content

	// Assert
	requireScreen(t, view, "Reopen workflow here", "announced to")
}
