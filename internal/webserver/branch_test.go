// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestGetBranchReturnsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, filledDeps(), config.Default()), "/api/branch"))

	// Assert
	if branch.Name != testBranchName || branch.Base != testBase || branch.Ahead != 2 {
		t.Errorf("branch = %+v, want the current branch", branch)
	}
}

func TestGetBranchNamesTheRemoteItsPushGoesTo(t *testing.T) {
	t.Parallel()

	// Arrange
	const forkRemote = "fork"

	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: testBranchName, PushRemote: forkRemote}, nil
	}

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.PushRemote != forkRemote {
		t.Errorf("push_remote = %q, want %q", branch.PushRemote, forkRemote)
	}
}

func TestGetBranchIsEmptyWhenNoRepositoryIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = nil

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.Name != "" || len(branch.Commits) != 0 {
		t.Errorf("branch = %+v, want an empty branch", branch)
	}
}

func TestGetBranchReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/branch")

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
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: testBranchName}, errSeam }

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Branch.Name != "" || snap.Branch.Commits == nil {
		t.Errorf("branch = %+v, want an empty branch with an empty commit list", snap.Branch)
	}
}
