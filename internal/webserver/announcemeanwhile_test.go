// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// An announcement asked to wait for CI is composed from one read of the
// branch and its CI, and held on another: what changes between them, or
// before a held one goes, decides what becomes of it.

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
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

func TestAnAnnouncementWhoseCIFailsAsItIsAskedToWaitIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	// The CI runs as the announcement is composed, ready for review, and has
	// failed by the time the hold is decided.
	world := newForgeWorld()
	deps := world.deps()
	state := inTurn(forge.CIRunning, forge.CIFailed)
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: state(), Total: 1}, nil
	}

	// Act
	recorder := askToWait(t, serve(t, deps, config.Default()))

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "no running CI to wait for")

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing", posts)
	}
}

func TestAnAnnouncementWhoseBranchCannotBeReadAsItIsAskedToWaitIsAFault(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is read to compose the announcement, and cannot be read again
	// to hold it.
	world := newForgeWorld()
	deps := world.deps()
	checkedOut := deps.Git.Branch
	readable := inTurn(true, false)
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		if !readable() {
			return gitrepo.Branch{}, errSeam
		}

		return checkedOut()
	}
	handler := serve(t, deps, config.Default())

	// Act
	recorder := askToWait(t, handler)

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

	if held := heldAnnouncement(t, handler); held != nil {
		t.Errorf("held announcement = %+v, want none", held)
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
	handler := serve(t, deps, config.Default())

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
