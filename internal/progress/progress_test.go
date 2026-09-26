// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package progress_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/progress"
)

// Short names for the states, so the expected rows fit on a line.
const (
	todo = progress.NotStarted
	wip  = progress.InFlight
	done = progress.Done
	oops = progress.Failed
)

// states pulls the state out of each stage, in order.
func states(stages []progress.Stage) []progress.State {
	pulled := make([]progress.State, len(stages))
	for index, stage := range stages {
		pulled[index] = stage.State
	}

	return pulled
}

func TestStagesDeriveHowFarTheWorkHasGot(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		work progress.Work
		want []progress.State
	}{
		"nothing started": {
			work: progress.Work{},
			want: []progress.State{todo, todo, todo, todo, todo},
		},
		"an issue picked but not branched": {
			work: progress.Work{IssueSelected: true},
			want: []progress.State{wip, todo, todo, todo, todo},
		},
		"on a branch with changes to commit": {
			work: progress.Work{OnFeatureBranch: true, IssueNamed: true, UncommittedChanges: 2},
			want: []progress.State{done, done, wip, todo, todo},
		},
		"committed, review open, CI running": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 3, PullRequest: progress.PullRequestOpen, CI: forge.CIRunning,
			},
			want: []progress.State{done, done, done, wip, todo},
		},
		"CI passed and announced": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 3,
				PullRequest: progress.PullRequestOpen, CI: forge.CIPassed, Announced: true,
			},
			want: []progress.State{done, done, done, done, done},
		},
		"CI failed": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 1, PullRequest: progress.PullRequestOpen, CI: forge.CIFailed,
			},
			want: []progress.State{done, done, done, oops, todo},
		},
		"changes requested stops review reading done": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 1,
				PullRequest: progress.PullRequestOpen, CI: forge.CIPassed, ChangesRequested: true,
			},
			want: []progress.State{done, done, done, oops, todo},
		},
		"merged, with no CI left to pass": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 1, PullRequest: progress.PullRequestMerged,
			},
			want: []progress.State{done, done, done, done, todo},
		},
		"merged, the changes once asked for moot": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 1,
				PullRequest: progress.PullRequestMerged, ChangesRequested: true,
			},
			want: []progress.State{done, done, done, done, todo},
		},
		"a post waiting for CI": {
			work: progress.Work{
				OnFeatureBranch: true, IssueNamed: true, Commits: 1,
				PullRequest: progress.PullRequestOpen, CI: forge.CIPassed, PostPending: true,
			},
			want: []progress.State{done, done, done, done, wip},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := states(progress.Stages(tt.work, "Slack"))

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("Stages states = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStagesAreTheLoopInOrderNamedForTheMessagingService(t *testing.T) {
	t.Parallel()

	// Act
	stages := progress.Stages(progress.Work{}, "Teams")

	// Assert
	names := make([]string, len(stages))
	for index, stage := range stages {
		names[index] = stage.Name
	}

	if want := []string{"Issue", "Branch", "Commits", "Review", "Teams"}; !slices.Equal(names, want) {
		t.Errorf("stage names = %v, want %v", names, want)
	}
}

func TestPullStateOfFollowsTheReviewAFoundPullRequestHas(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		state forge.PullState
		want  progress.PullState
	}{
		"open, under review":           {state: forge.StateOpen, want: progress.PullRequestOpen},
		"merged, its review over":      {state: forge.StateMerged, want: progress.PullRequestMerged},
		"closed without merging, none": {state: forge.StateClosed, want: progress.NoPullRequest},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := progress.PullStateOf(tt.state)

			// Assert
			if got != tt.want {
				t.Errorf("PullStateOf(%v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
