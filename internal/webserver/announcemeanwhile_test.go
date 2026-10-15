// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// An announcement asked to wait for CI is composed from one read of the
// branch and its CI, and held on another: what changes between them, or
// before a held one goes, decides what becomes of it.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// inTurn answers each call with the next of answers, and the last once they
// run out.
func inTurn[T any](answers ...T) func() T {
	var (
		lock sync.Mutex
		next int
	)

	return func() T {
		lock.Lock()
		defer lock.Unlock()

		answer := answers[min(next, len(answers)-1)]
		next++

		return answer
	}
}

// askToWait asks handler, with no preview first, to post the announcement to
// #dev once CI passes.
func askToWait(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	return postAnnounce(t, handler, map[string]string{channelField: slackChannel, whenField: whenCIPasses})
}

func TestAnAnnouncementWhoseBranchCannotBeReadAsItIsAskedToWaitIsHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is read to compose the announcement, and cannot be read again
	// to hold it.
	world := newForgeWorld()
	deps := world.deps()
	checkedOut := deps.Git.Branch
	readable := inTurn(true, false)
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) {
		if !readable() {
			return gitrepo.Branch{}, webtest.ErrSeam
		}

		return checkedOut(ctx)
	}
	handler := webtest.Serve(t, deps, config.Default())
	cancelHeld(t, handler)

	// Act
	recorder := askToWait(t, handler)

	// Assert
	// Its CI could not be read, so nothing failing could not be confirmed:
	// it waits for a read that says CI passed.
	held := webtest.Decode[api.QueuedAnnouncement](t, recorder)
	if recorder.Code != http.StatusAccepted || held.State != api.QueuedAnnouncementStateWaiting || held.Ci != nil {
		t.Errorf("status = %d, held %+v; want 202 and it waiting, its CI unread", recorder.Code, held)
	}
}

func TestAHeldAnnouncementMadeMeanwhileElsewhereIsDroppedUnposted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A terminal announces the same moment while this one waits for its CI.
	world := newForgeWorld()
	memory := &announceMemory{}
	deps := world.deps()
	remembering := memory.wire(deps)
	deps.Store.Announced, deps.Store.RecordAnnounce = remembering.Store.Announced, remembering.Store.RecordAnnounce
	handler := webtest.Serve(t, deps, config.Default())

	if recorder := askToWait(t, handler); recorder.Code != http.StatusAccepted {
		t.Fatalf("holding the announcement answered %d: %s", recorder.Code, recorder.Body.String())
	}

	memory.mu.Lock()
	memory.held = append(memory.held, readyAt42())
	memory.mu.Unlock()

	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateDropped ||
		orEmpty(held.Reason) != "#42 was already announced at this moment, here or from a terminal" {
		t.Errorf("held announcement = %+v, want it dropped as announced already", held)
	}

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing posted twice", posts)
	}
}
