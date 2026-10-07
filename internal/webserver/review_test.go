// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"sync/atomic"
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

	if review.Pull.Mergeable != api.PullRequestMergeableClean {
		t.Errorf("mergeable = %q, want clean", review.Pull.Mergeable)
	}

	if review.Ci == nil || review.Ci.State != api.CIStatePassed {
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

func TestGetReviewHasNothingWhenNoRepositoryIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = nil

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Found {
		t.Errorf("review = %+v, want none found when no repository is configured", review)
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

func TestGetReviewAsksNoCIAboutAMergedPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merged pull request has no live CI, as the terminal already knows.
	var asked atomic.Int32

	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 42, State: forge.StateMerged}, true, nil
	}
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		asked.Add(1)

		return forge.CI{State: forge.CIPassed}, nil
	}

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if !review.Found || review.Pull == nil {
		t.Fatalf("review = %+v, want the merged pull request found", review)
	}

	if calls := asked.Load(); calls != 0 || review.Ci != nil {
		t.Errorf("CI asked %d times, ci = %+v; want no CI asked about a merged pull request", calls, review.Ci)
	}
}

func TestGetReviewCarriesThePullRequestsState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		state forge.PullState
		want  api.PullRequestState
	}{
		"open":   {state: forge.StateOpen, want: api.PullRequestStateOpen},
		"merged": {state: forge.StateMerged, want: api.PullRequestStateMerged},
		"closed": {state: forge.StateClosed, want: api.PullRequestStateClosed},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 42, State: tt.state}, true, nil
			}

			// Act
			review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

			// Assert
			if review.Pull == nil || review.Pull.State != tt.want {
				t.Errorf("pull = %+v, want its state %s", review.Pull, tt.want)
			}
		})
	}
}

func TestGetReviewMapsEveryCIState(t *testing.T) {
	t.Parallel()

	// Arrange
	// One CI carrying every state — the overall state and one check per state —
	// so the mapping is exercised for all of them at once.
	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{
			State: forge.CIRunning,
			Checks: []forge.Check{
				{Name: "none", State: forge.CINone},
				{Name: "passed", State: forge.CIPassed},
				{Name: "failed", State: forge.CIFailed},
			},
		}, nil
	}

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Ci == nil || review.Ci.State != api.CIStateRunning || len(review.Ci.Checks) != 3 {
		t.Fatalf("ci = %+v, want state running and three checks", review.Ci)
	}

	want := []api.CIState{api.CIStateNone, api.CIStatePassed, api.CIStateFailed}
	for i, check := range review.Ci.Checks {
		if check.State != want[i] {
			t.Errorf("check %d state = %q, want %q", i, check.State, want[i])
		}
	}
}

func TestGetReviewMapsMergeability(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from forge.Mergeability
		want api.PullRequestMergeable
	}{
		"unknown":   {forge.MergeUnknown, api.PullRequestMergeableUnknown},
		"clean":     {forge.MergeClean, api.PullRequestMergeableClean},
		"conflicts": {forge.MergeConflicts, api.PullRequestMergeableConflicts},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 1, Mergeable: tt.from}, true, nil
			}

			// Act
			review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

			// Assert
			if review.Pull == nil || review.Pull.Mergeable != tt.want {
				t.Errorf("mergeable = %v, want %q", review.Pull, tt.want)
			}
		})
	}
}

func TestSnapshotReviewIsNotFoundWhenTheBranchReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge would find a pull request for any branch, so only the failed
	// branch read can leave the review empty.
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: testBranchName}, errSeam }

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Review.Found || snap.Review.Pull != nil {
		t.Errorf("review = %+v, want none found when the branch read fails", snap.Review)
	}
}

func TestSnapshotReviewIsNotFoundWhenThePullReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failing find still hands back a pull request, so only the error can
	// empty the review.
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 42}, true, errSeam
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Review.Found || snap.Review.Pull != nil {
		t.Errorf("review = %+v, want none found when the pull request read fails", snap.Review)
	}
}

func TestSnapshotReviewIsNotFoundWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is read and known, so only the missing forge can leave the
	// review empty.
	deps := filledDeps()
	deps.FindPull = nil

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Branch.Name != testBranchName {
		t.Fatalf("branch = %q, want %q read for the frame", snap.Branch.Name, testBranchName)
	}

	if snap.Review.Found || snap.Review.Pull != nil {
		t.Errorf("review = %+v, want none found without a forge", snap.Review)
	}
}
