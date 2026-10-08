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

func TestListChangesReturnsTheWorkingTree(t *testing.T) {
	t.Parallel()

	// Act
	changes := decode[api.ChangeList](t, get(t, serve(t, filledDeps(), config.Default()), "/api/changes"))

	// Assert
	if len(changes.Changes) != 1 || changes.Changes[0].Path != "internal/config/config.go" {
		t.Errorf("changes = %+v, want the one staged file", changes.Changes)
	}
}

func TestListChangesIsEmptyWithoutARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Changes = nil

	// Act
	changes := decode[api.ChangeList](t, get(t, serve(t, deps, config.Default()), "/api/changes"))

	// Assert
	if len(changes.Changes) != 0 {
		t.Errorf("changes = %+v, want empty without a repository", changes.Changes)
	}
}

func TestListChangesReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/changes")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestSnapshotChangesAreEmptyWhenTheReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failing read still hands back a change, so only the error can empty
	// the panel.
	deps := filledDeps()
	deps.Git.Changes = func() ([]gitrepo.Change, error) {
		return []gitrepo.Change{{Path: "README.md", Staged: 'M'}}, errSeam
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Changes.Changes == nil || len(snap.Changes.Changes) != 0 {
		t.Errorf("changes = %#v, want an empty list when the read fails", snap.Changes.Changes)
	}
}
