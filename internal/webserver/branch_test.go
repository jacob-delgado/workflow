// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestGetBranchReturnsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	branch := webtest.Decode[api.Branch](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(), config.Default()),
		"/api/branch"))

	// Assert
	if branch.Name != webtest.TestBranchName || branch.Base != webtest.TestBase || branch.Ahead != 2 {
		t.Errorf("branch = %+v, want the current branch", branch)
	}
}

func TestGetBranchNamesTheRemoteItsPushGoesTo(t *testing.T) {
	t.Parallel()

	// Arrange
	const forkRemote = "fork"

	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: webtest.TestBranchName, PushRemote: forkRemote}, nil
	}

	// Act
	branch := webtest.Decode[api.Branch](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.PushRemote != forkRemote {
		t.Errorf("push_remote = %q, want %q", branch.PushRemote, forkRemote)
	}
}

func TestGetBranchIsEmptyWhenNoRepositoryIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Git.Branch = nil

	// Act
	branch := webtest.Decode[api.Branch](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.Name != "" || len(branch.Commits) != 0 {
		t.Errorf("branch = %+v, want an empty branch", branch)
	}
}

func TestGetBranchReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, webtest.ErrSeam }

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/branch")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestSnapshotBranchIsEmptyWhenTheReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failing read still hands back a branch, so only the error can empty
	// the panel.
	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: webtest.TestBranchName}, webtest.ErrSeam
	}

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if snap.Branch.Name != "" || snap.Branch.Commits == nil {
		t.Errorf("branch = %+v, want an empty branch with an empty commit list", snap.Branch)
	}
}

func TestTheBranchMarksTheCommitsNotYetPushed(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{
			Name: webtest.TestBranchName, Base: webtest.TestBase, Upstream: gitrepo.DefaultRemote + "/" + webtest.TestBranchName,
			PushRemote: gitrepo.DefaultRemote, Ahead: 1,
			Commits: []gitrepo.Commit{{Hash: "aaa1111", Subject: "feat: first"}, {Hash: "bbb2222", Subject: "fix: second"}},
		}, nil
	}

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	marked := make([]bool, 0, len(snap.Branch.Commits))
	for _, commit := range snap.Branch.Commits {
		marked = append(marked, commit.Unpushed)
	}

	if !slices.Equal(marked, []bool{false, true}) {
		t.Errorf("unpushed = %v, want only the commit the upstream lacks", marked)
	}
}
