// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// renamed is a staged rename, whose diff reads both of its paths.
func renamed() gitrepo.Change {
	return gitrepo.Change{Path: "internal/new.go", OriginalPath: "internal/old.go", Staged: 'R'}
}

// diffOf is where a changed file's diff is read.
func diffOf(path string) string {
	return "/api/changes/diff?path=" + url.QueryEscape(path)
}

func TestChangeDiffReadsTheChangeTheTreeLists(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []gitrepo.Change

	deps := filledDeps()
	deps.Git.Changes = func() ([]gitrepo.Change, error) { return []gitrepo.Change{renamed()}, nil }
	deps.Git.Diff = func(change gitrepo.Change) ([]string, error) {
		asked = append(asked, change)

		return []string{"--- a/internal/old.go", "+++ b/internal/new.go", "+package internal"}, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("internal/new.go"))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
	}

	diff := decode[api.FileDiff](t, recorder)
	if diff.Path != "internal/new.go" || len(diff.Lines) != 3 || diff.Lines[2] != "+package internal" {
		t.Errorf("diff = %+v, want the three lines of internal/new.go", diff)
	}

	if !slices.Equal(asked, []gitrepo.Change{renamed()}) {
		t.Errorf("Diff was asked for %v, want the rename as the tree lists it, both paths", asked)
	}
}

func TestChangeDiffOfAnUnchangedFileIsNotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = func(gitrepo.Change) ([]string, error) { return nil, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("/etc/passwd"))

	// Assert
	assertProblem(t, recorder, http.StatusNotFound, "lists no change at that path")
}

func TestChangeDiffWithNoRepositoryIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = nil

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("internal/config/config.go"))

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestChangeDiffThatGitCannotReadKeepsItsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = func(gitrepo.Change) ([]string, error) {
		return nil, fmt.Errorf("reading the diff in %s: %w", repoPath, errSeam)
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("internal/config/config.go"))

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

	if strings.Contains(recorder.Body.String(), repoPath) {
		t.Errorf("the refusal %s names where the repository is", recorder.Body)
	}
}

func TestChangeDiffWithNoDifferenceIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = func(gitrepo.Change) ([]string, error) { return nil, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("internal/config/config.go"))

	// Assert
	if diff := decode[api.FileDiff](t, recorder); diff.Lines == nil || len(diff.Lines) != 0 {
		t.Errorf("lines = %#v, want an empty list", diff.Lines)
	}
}

func TestChangeDiffWithNoTreeToReadIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = func(gitrepo.Change) ([]string, error) { return nil, nil }
	deps.Git.Changes = nil

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("a.go"))

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestChangeDiffWhoseTreeCannotBeReadSaysToTryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Git.Diff = func(gitrepo.Change) ([]string, error) { return nil, nil }
	deps.Git.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), diffOf("a.go"))

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)
}
