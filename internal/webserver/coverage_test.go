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

func TestListChangesIsEmptyWithoutARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Changes = nil

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
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/changes")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestGetMessagingHasNoAuthorWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = nil

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, deps, config.Default()), "/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty without a forge", destination.Author)
	}
}
