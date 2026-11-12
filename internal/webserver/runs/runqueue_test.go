// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

// A run waits its turn for the index behind the server's other index writes,
// and one queued ahead of it can move the branch it was asked for on.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// movingBranch is the checked-out branch, which a write queued ahead of a run
// moves, and the changes, staged throughout; read is closed once the changes
// are first read.
type movingBranch struct {
	branch atomic.Pointer[gitrepo.Branch]
	once   sync.Once
	read   chan struct{}
}

// newMovingBranch is on webtest.FilledDeps' branch, whose one commit is not
// yet pushed.
func newMovingBranch(t *testing.T) *movingBranch {
	t.Helper()

	feature, err := webtest.FilledDeps().Git.Branch(t.Context())
	if err != nil {
		t.Fatalf("reading the fixture's branch: %v", err)
	}

	moving := &movingBranch{read: make(chan struct{})}
	moving.branch.Store(&feature)

	return moving
}

// git is the branch and changes seams over the moving branch.
func (m *movingBranch) git() seams.Git {
	return seams.Git{
		Branch: func(context.Context) (gitrepo.Branch, error) { return *m.branch.Load(), nil },
		Changes: func(context.Context) ([]gitrepo.Change, error) {
			m.once.Do(func() { close(m.read) })

			return []gitrepo.Change{{Path: "internal/config/config.go", Staged: 'M'}}, nil
		},
	}
}

// toBase moves the branch to its base, every commit of which is pushed, as a
// finish does.
func (m *movingBranch) toBase() {
	m.branch.Store(&gitrepo.Branch{
		Name: webtest.TestBase, Upstream: "origin/" + webtest.TestBase, Ahead: 0, Head: "def456",
		Commits: []gitrepo.Commit{{Hash: "def456", Subject: "chore: pushed long ago"}},
	})
}

func TestAnAmendQueuedBehindAMoveToThePushedBaseAmendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The test holds the index, as a finish does while it moves the branch to
	// its base; the amend is asked for meanwhile, on the branch as it stood.
	var (
		indexWrites sync.Mutex
		amended     atomic.Bool
	)

	moving := newMovingBranch(t)
	given := runs.Seams{Git: moving.git()}
	given.Git.Amend = func(context.Context) (proc.Output, error) {
		amended.Store(true)

		return finished(nil), nil
	}
	runner := runs.New(given, func(error) {}, &indexWrites, func() int { return 1 })
	recorder, request, done := httptest.NewRecorder(), runRequest(t, amendRun), make(chan struct{})

	indexWrites.Lock()

	go func() {
		defer close(done)

		runner.StartRun(recorder, request)
	}()

	<-moving.read
	moving.toBase()

	// Act
	indexWrites.Unlock()
	<-done

	// Assert
	if amended.Load() {
		t.Error("the amend ran on the base it was queued behind a move to, want it refused: every commit there is pushed")
	}

	assertProblem(t, recorder, http.StatusConflict, "nothing to amend")
}
