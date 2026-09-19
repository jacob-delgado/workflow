// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestGetReviewReturnsThePullAndCI(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, filledDeps(), config.Default()), "/api/review"))

	// Assert
	if !review.Found || review.Pull == nil || review.Pull.Number != 42 {
		t.Fatalf("review = %+v, want the pull request found", review)
	}

	if review.Pull.Mergeable != api.Clean {
		t.Errorf("mergeable = %q, want clean", review.Pull.Mergeable)
	}

	if review.Ci == nil || review.Ci.State != api.Passed {
		t.Errorf("ci = %+v, want state passed", review.Ci)
	}
}

func TestGetReviewReportsNoPull(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Found || review.Pull != nil {
		t.Errorf("review = %+v, want none found", review)
	}
}

func TestGetReviewHasNothingWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = nil

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Found {
		t.Errorf("review = %+v, want none found without a forge", review)
	}
}

func TestGetReviewReportsABranchFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/review")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestGetReviewReportsAPullFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/review")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestGetReviewOmitsCIWhenItCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{}, errSeam }

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	// The pull request still shows; only its CI is left off.
	if !review.Found || review.Pull == nil {
		t.Fatalf("review = %+v, want the pull request found", review)
	}

	if review.Ci != nil {
		t.Errorf("ci = %+v, want it omitted when unreadable", review.Ci)
	}
}

func TestGetReviewOmitsCIWhenUnavailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.CheckCI = nil

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if !review.Found || review.Ci != nil {
		t.Errorf("review = %+v, want the pull without CI", review)
	}
}
