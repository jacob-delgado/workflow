// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errGitFailed is a read of the repository that failed.
var errGitFailed = errors.New("git failed")

// mergedPull is a pull request merged at mergedHead.
func mergedPull() forge.PullRequest {
	return forge.PullRequest{Number: 7, State: forge.StateMerged, HeadCommit: mergedHead}
}

// madeSince is own commits made on a branch after its pull request merged,
// the newest gitrepo.NamedAfterMerge of them listed newest first, beside
// onBase the base's branch already holds.
func madeSince(own, onBase int) gitrepo.AfterMerge {
	listed := min(own, gitrepo.NamedAfterMerge)

	latest := make([]gitrepo.Commit, 0, listed)
	for i := range listed {
		latest = append(latest, gitrepo.Commit{Hash: "c0ffee" + strconv.Itoa(i), Subject: "feat: " + strconv.Itoa(i)})
	}

	return gitrepo.AfterMerge{Latest: latest, Own: own, OnBase: onBase, Base: "origin/main"}
}

// answering is a read of the commits made after a merge that answers made
// and err, and keeps the pull request it was asked about in asked.
func answering(made gitrepo.AfterMerge, err error, asked *forge.PullRequest) func(
	context.Context, forge.PullRequest,
) (gitrepo.AfterMerge, error) {
	return func(_ context.Context, pull forge.PullRequest) (gitrepo.AfterMerge, error) {
		*asked = pull

		return made, err
	}
}

func TestRefuseMadeAfterRefusesABranchCommittedToSinceItMerged(t *testing.T) {
	t.Parallel()

	// Arrange
	// P005: a branch name reused once its pull request merged, or work gone
	// on past the merge, pushed: its upstream holds every commit.
	var asked forge.PullRequest

	// Act
	got := loop.RefuseMadeAfter(t.Context(), answering(madeSince(2, 0), nil, &asked), mergedPull())

	// Assert
	want := "made after it merged, which a new pull request would carry: c0ffee0 feat: 0; c0ffee1 feat: 1"
	if !errors.Is(got, loop.ErrMadeAfterMerge) || !strings.HasSuffix(fmt.Sprint(got), want) {
		t.Errorf("RefuseMadeAfter = %v, want ErrMadeAfterMerge ending %q", got, want)
	}

	if asked != mergedPull() {
		t.Errorf("asked about %+v, want the merged pull request", asked)
	}
}

func TestRefuseMadeAfterNamesFiveCommitsAndCountsTheRest(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked forge.PullRequest

	// Act
	got := loop.RefuseMadeAfter(t.Context(), answering(madeSince(8, 0), nil, &asked), mergedPull())

	// Assert
	want := "c0ffee3 feat: 3; c0ffee4 feat: 4; and 3 more"
	if !strings.HasSuffix(fmt.Sprint(got), want) {
		t.Errorf("RefuseMadeAfter = %v, want five named, ending %q", got, want)
	}
}

func TestRefuseMadeAfterNamesItsOwnCommitsBesideThoseOnTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	// Squash-merged, then moved onto the newer base and committed to.
	var asked forge.PullRequest

	// Act
	got := loop.RefuseMadeAfter(t.Context(), answering(madeSince(1, 7), nil, &asked), mergedPull())

	// Assert
	want := "which a new pull request would carry: c0ffee0 feat: 0, beside 7 already on origin/main"
	if !errors.Is(got, loop.ErrMadeAfterMerge) || !strings.HasSuffix(fmt.Sprint(got), want) {
		t.Errorf("RefuseMadeAfter = %v, want ErrMadeAfterMerge ending %q", got, want)
	}
}

func TestRefuseMadeAfterRefusesABranchMovedOnlyOntoTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked forge.PullRequest

	// Act
	got := loop.RefuseMadeAfter(t.Context(), answering(madeSince(0, 3), nil, &asked), mergedPull())

	// Assert
	want := "it has commits made after it merged: 3, all already on origin/main"
	if !errors.Is(got, loop.ErrMadeAfterMerge) || fmt.Sprint(got) != want {
		t.Errorf("RefuseMadeAfter = %v, want ErrMadeAfterMerge reading %q", got, want)
	}
}

func TestRefuseMadeAfterFindsNothingInTheWay(t *testing.T) {
	t.Parallel()

	var askedMerged, askedOpen forge.PullRequest

	cases := map[string]struct {
		madeAfter func(context.Context, forge.PullRequest) (gitrepo.AfterMerge, error)
		pull      forge.PullRequest
	}{
		"nothing made since":   {madeAfter: answering(gitrepo.AfterMerge{}, nil, &askedMerged), pull: mergedPull()},
		"not merged":           {madeAfter: answering(madeSince(1, 0), nil, &askedOpen), pull: mergeablePull()},
		"no repository to ask": {madeAfter: nil, pull: mergedPull()},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.RefuseMadeAfter(t.Context(), tt.madeAfter, tt.pull)

			// Assert
			if got != nil {
				t.Errorf("RefuseMadeAfter = %v, want nothing in the way", got)
			}
		})
	}
}

func TestRefuseMadeAfterSaysWhatItCouldNotRead(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"git failed":                    errGitFailed,
		"the merged commit not fetched": gitrepo.ErrNotFetched,
	}

	for name, readErr := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var asked forge.PullRequest

			// Act
			got := loop.RefuseMadeAfter(t.Context(), answering(gitrepo.AfterMerge{}, readErr, &asked), mergedPull())

			// Assert
			if !errors.Is(got, readErr) || errors.Is(got, loop.ErrMadeAfterMerge) {
				t.Errorf("RefuseMadeAfter = %v, want %v, and no commits named", got, readErr)
			}
		})
	}
}
