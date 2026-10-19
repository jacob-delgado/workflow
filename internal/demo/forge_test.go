// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// manual is a clock a test moves on by hand.
type manual struct {
	elapsed time.Duration
}

// since is how far the test has moved the clock.
func (m *manual) since() time.Duration {
	return m.elapsed
}

// committedFeature is the demo's world, made with options, with the feature
// branch committed on and not yet pushed, its clock moved on by hand.
func committedFeature(t *testing.T, options ...demo.Option) (tui.Deps, *manual) {
	t.Helper()

	clock := &manual{}
	deps := demo.New(clock.since, options...).Deps()

	err := deps.Git.CreateBranch(t.Context(), feature, "origin/main")
	if err != nil {
		t.Fatal(err)
	}

	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))

	return deps, clock
}

// pushedFeature is committedFeature with the feature branch pushed.
func pushedFeature(t *testing.T, options ...demo.Option) (tui.Deps, *manual) {
	t.Helper()

	deps, clock := committedFeature(t, options...)
	_, _ = ran(t)(deps.Git.Push(t.Context(), feature))

	return deps, clock
}

// request asks for the feature branch's pull request into main.
func request() forge.NewPullRequest {
	return forge.NewPullRequest{
		Title: "fix: redact tokens in the request log", Body: "PROJ-412", Head: forge.Head{Branch: feature}, Base: mainBranch,
	}
}

func TestOpeningAPullRequestKeepsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, _ := pushedFeature(t)

	// Act
	pull, err := deps.Forge.CreatePullRequest(t.Context(), request())

	// Assert
	if err != nil || !pull.Opened() || pull.Title != request().Title || pull.URL == "" {
		t.Fatalf("opening the pull request answered %+v, %v", pull, err)
	}

	found, open, err := deps.Forge.FindPullRequest(t.Context(), feature)
	if err != nil || !open || found.Number != pull.Number {
		t.Errorf("finding the branch's pull request = %+v, %v, %v; want #%d", found, open, err, pull.Number)
	}
}

func TestABranchNotPushedHasNoPullRequestOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	_, err := deps.Forge.CreatePullRequest(t.Context(), request())

	// Assert
	if err == nil {
		t.Error("a pull request opened for a branch GitHub does not have")
	}

	if _, open, _ := deps.Forge.FindPullRequest(t.Context(), feature); open {
		t.Error("the refused pull request was kept")
	}
}

func TestASecondPullRequestForOneBranchIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, _ := pushedFeature(t)
	first, _ := deps.Forge.CreatePullRequest(t.Context(), request())

	// Act
	_, err := deps.Forge.CreatePullRequest(t.Context(), request())

	// Assert
	if err == nil {
		t.Error("a second pull request opened for one branch")
	}

	if found, _, _ := deps.Forge.FindPullRequest(t.Context(), feature); found.Number != first.Number {
		t.Errorf("the branch's pull request became #%d, want #%d still", found.Number, first.Number)
	}
}

func TestCIHasOneCheckPassedAsThePullRequestOpens(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, _ := pushedFeature(t)
	pull, _ := deps.Forge.CreatePullRequest(t.Context(), request())

	// Act
	ci, err := deps.Forge.CheckStatus(t.Context(), pull, pull.HeadCommit)

	// Assert
	if err != nil || ci.State != forge.CIRunning || ci.Done != 1 || ci.Total != len(ci.Checks) || ci.Total < 2 {
		t.Errorf("CI on opening = %+v, %v; want one check passed and the rest running", ci, err)
	}
}

func TestCIPassesOneCheckAtATime(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, clock := pushedFeature(t)
	pull, _ := deps.Forge.CreatePullRequest(t.Context(), request())
	clock.elapsed += 2 * time.Second

	// Act
	ci, err := deps.Forge.CheckStatus(t.Context(), pull, pull.HeadCommit)

	// Assert
	if err != nil || ci.State != forge.CIRunning || ci.Done == 0 || ci.Done == ci.Total {
		t.Errorf("CI two seconds in = %+v, %v; want some checks passed and some running", ci, err)
	}
}

func TestCIPassesFourSecondsAfterThePullRequestOpens(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, clock := pushedFeature(t)
	pull, _ := deps.Forge.CreatePullRequest(t.Context(), request())
	clock.elapsed += 4 * time.Second

	// Act
	ci, err := deps.Forge.CheckStatus(t.Context(), pull, pull.HeadCommit)

	// Assert
	if err != nil || ci.State != forge.CIPassed || ci.Done != ci.Total || ci.Failed != 0 {
		t.Errorf("CI four seconds in = %+v, %v; want every check passed", ci, err)
	}
}

func TestCIOfAPullRequestNeverOpenedIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, _ := pushedFeature(t)

	// Act
	_, err := deps.Forge.CheckStatus(t.Context(), forge.PullRequest{Number: 9}, "abc1234")

	// Assert
	if err == nil {
		t.Error("CI read for a pull request the demo never opened")
	}
}

func TestTodaysSummaryHoldsThePullRequestOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, _ := pushedFeature(t)
	pull, _ := deps.Forge.CreatePullRequest(t.Context(), request())
	start := deps.Clock().Truncate(24 * time.Hour)

	// Act
	today, err := deps.Forge.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	if err != nil || len(today.Events) != 1 || today.Events[0].Number != pull.Number ||
		today.Events[0].Kind != forge.EventOpened {
		t.Errorf("today's forge activity = %+v, %v; want the pull request opened", today, err)
	}
}
