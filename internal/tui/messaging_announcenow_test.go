// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// An announcement posted now while the one armed for CI still waits goes in
// that one's place: the waiting post is held back while it is out.

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// postingNow is the interface posting an announcement now while the one
// armed by w waits: the hold's first read of CI, still out, and Slack's
// answer to the post made now, held until release is called.
type postingNow struct {
	sending tui.Model
	read    tea.Cmd
	posted  <-chan tea.Msg
	release func()
}

// announcingNow arms the announcement with w, keeping its first read back,
// and posts the same announcement now from the preview, Slack holding that
// post.
func announcingNow(t *testing.T, repo *world) postingNow {
	t.Helper()

	repo.postParked, repo.postRelease = make(chan struct{}, 1), make(chan struct{})
	release := sync.OnceFunc(func() { close(repo.postRelease) })
	t.Cleanup(release)

	armed, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	sending, post := pressed(t, typing(t, armed, "p"), keyEnter)

	answer := make(chan tea.Msg, 1)

	go func() { answer <- post() }()

	select {
	case <-repo.postParked:
	case <-time.After(failsafe):
		t.Fatalf("the post had not reached Slack after %v: enter did not send it", failsafe)
	}

	return postingNow{sending: sending, read: read, posted: answer, release: release}
}

// answered is the model once Slack's answer to the post made now is
// delivered.
func (p postingNow) answered(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	return drain(t, model, func() tea.Msg { return <-p.posted })
}

func TestAnAnnounceNowPostIsNotToldAsAnnouncedElsewhere(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	now := announcingNow(t, repo)

	// Act: the waiting post's read finds CI passed while the post made now is out
	heard := drain(t, now.sending, now.read)

	// Assert: the waiting post is held back, not told as another session's
	refuseScreen(t, heard.View().Content, "another session")

	// Act: Slack answers the post made now
	now.release()

	view := now.answered(t, heard).View().Content

	// Assert: it is told as this session's, and went alone
	requireScreen(t, view, "● announced to "+slackChannel)
	refuseScreen(t, view, "another session", "announces when CI passes")

	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("post calls = %q, want the one made now alone", posts)
	}
}

func TestTheWaitingPostGoesWhenTheOneMadeNowFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The waiting post's read lands while the post made now is out, and
	// that post fails a moment later, after which Slack takes posts again.
	// Nothing is pressed: the hold is read again on its beat alone.
	repo := newWorld()
	repo.postErr = errNotInChannel
	repo.ciInterval = 100 * time.Millisecond
	now := announcingNow(t, repo)
	slackFails := fakeAfter(10*time.Millisecond, func(time.Time) tea.Msg {
		now.release()

		failure := <-now.posted

		repo.mu.Lock()
		repo.postErr, repo.postParked = nil, nil
		repo.mu.Unlock()

		return failure
	})

	// Act
	view := drain(t, now.sending, tea.Batch(now.read, slackFails)).View().Content

	// Assert
	refuseScreen(t, view, "announces when CI passes", "announced elsewhere")

	if posts := repo.asked("post "); len(posts) != 2 {
		t.Errorf("post calls = %q, want the one that failed and then the waiting one", posts)
	}
}

func TestTheWaitingPostGoingOutClosesARefusedPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	// The post made now fails, which leaves its preview open with the
	// refusal; the waiting post then goes on the hold's beat.
	repo := newWorld()
	repo.postErr = errNotInChannel
	repo.ciInterval = 100 * time.Millisecond
	now := announcingNow(t, repo)
	slackFails := fakeAfter(10*time.Millisecond, func(time.Time) tea.Msg {
		now.release()

		failure := <-now.posted

		repo.mu.Lock()
		repo.postErr, repo.postParked = nil, nil
		repo.mu.Unlock()

		return failure
	})

	// Act
	view := drain(t, now.sending, tea.Batch(now.read, slackFails)).View().Content

	// Assert
	refuseScreen(t, footerLine(view), "enter announce now")
	requireScreen(t, view, "● announced to "+slackChannel)
}

func TestTheWaitingPostGoingOutLeavesAnotherPullRequestsRefusedPreviewOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's post waits; the checkout moves to #43, whose post made now is
	// refused, and the hold's read, asked before, then finds #42's CI
	// passed.
	repo, waiting := waitingOn(t, forge.CIRunning)
	keptOn(repo)

	switched := typing(t, waiting, "2", "r", "5")
	reading, read := pressed(t, switched, "r")

	repo.postErr = errNotInChannel
	refused := typing(t, reading, "p", keyEnter)

	postsNow(repo)
	passesNow(repo)

	// Act
	view := drain(t, refused, read).View().Content

	// Assert
	requireScreen(t, view, "┏━ Announce to Slack", "enter announce now")

	if posts := repo.asked("post "); len(posts) != 2 || !strings.Contains(posts[1], pullURL) {
		t.Errorf("post calls = %q, want #43's refused, then #42's waiting post", posts)
	}
}

func TestTheWaitingPostGoingOutKeepsALaterFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's post waits; the one made now is refused, and the read of what
	// was announced after it fails, taking the footer's row from the
	// refusal, before the hold's read, asked before, finds CI passed.
	repo, waiting := waitingOn(t, forge.CIRunning)
	reading, read := pressed(t, waiting, "r")
	preview := typing(t, reading, "p")

	repo.postErr, repo.announcedErr = errNotInChannel, errStoreFull
	refused := typing(t, preview, keyEnter)

	repo.announcedErr = nil

	postsNow(repo)
	passesNow(repo)

	// Act
	view := drain(t, refused, read).View().Content

	// Assert
	refuseScreen(t, view, "┏━ Announce to Slack")
	requireScreen(t, view, "could not read what was announced")
}

func TestTheWaitingPostGoingOutClosesARefusedPreviewUnderTheOwnerPicker(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's post waits; the one made now is refused, and the owner picker
	// opens over its preview before the hold's read, asked before, finds
	// CI passed.
	repo := taggingWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}
	reading, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p", "w"), "r")

	repo.postErr = errNotInChannel
	picking := typing(t, reading, "p", keyEnter, "a")

	postsNow(repo)
	passesNow(repo)

	// Act
	view := drain(t, picking, read).View().Content

	// Assert
	refuseScreen(t, view, "Link "+ownerBen+" to Slack", "┏━ Announce to Slack")
	requireScreen(t, view, "● announced to "+slackChannel)
}
