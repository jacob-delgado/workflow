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

// review is a found pull request in state, its changes requested or not,
// with its CI in ci, as the stages read it.
func review(state forge.PullState, changesRequested bool, ci forge.CIState) progress.Review {
	return progress.ReviewOf(forge.PullRequest{State: state, ChangesRequested: changesRequested}, true, ci)
}

// branched is work on a feature branch named for an issue, with commits on
// it, under review, and announced as announce says.
func branched(commits int, under progress.Review, announce progress.AnnounceWork) progress.Work {
	return progress.Work{
		Issue: progress.IssueNamed, OnFeatureBranch: true, Commits: commits, Review: under, Announce: announce,
	}
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
			work: progress.Work{Issue: progress.IssuePicked},
			want: []progress.State{wip, todo, todo, todo, todo},
		},
		"on a branch with changes to commit": {
			work: progress.Work{Issue: progress.IssueNamed, OnFeatureBranch: true, UncommittedChanges: 2},
			want: []progress.State{done, done, wip, todo, todo},
		},
		"changes on the base branch, not yet work on a branch": {
			work: progress.Work{UncommittedChanges: 2},
			want: []progress.State{todo, todo, todo, todo, todo},
		},
		"committed, with changes still to commit": {
			work: progress.Work{Issue: progress.IssueNamed, OnFeatureBranch: true, Commits: 1, UncommittedChanges: 2},
			want: []progress.State{done, done, done, todo, todo},
		},
		"committed, review open, CI running": {
			work: branched(3, review(forge.StateOpen, false, forge.CIRunning), progress.AnnounceNone),
			want: []progress.State{done, done, done, wip, todo},
		},
		"review open, no CI to pass": {
			work: branched(1, review(forge.StateOpen, false, forge.CINone), progress.AnnounceNone),
			want: []progress.State{done, done, done, wip, todo},
		},
		"CI passed and announced": {
			work: branched(3, review(forge.StateOpen, false, forge.CIPassed), progress.AnnounceMade),
			want: []progress.State{done, done, done, done, done},
		},
		"CI failed": {
			work: branched(1, review(forge.StateOpen, false, forge.CIFailed), progress.AnnounceNone),
			want: []progress.State{done, done, done, oops, todo},
		},
		"changes requested stops review reading done": {
			work: branched(1, review(forge.StateOpen, true, forge.CIPassed), progress.AnnounceNone),
			want: []progress.State{done, done, done, oops, todo},
		},
		"changes requested, with no CI, stop review reading done": {
			work: branched(1, review(forge.StateOpen, true, forge.CINone), progress.AnnounceNone),
			want: []progress.State{done, done, done, oops, todo},
		},
		"merged, with no CI left to pass": {
			work: branched(1, review(forge.StateMerged, false, forge.CINone), progress.AnnounceNone),
			want: []progress.State{done, done, done, done, todo},
		},
		"merged, the changes once asked for moot": {
			work: branched(1, review(forge.StateMerged, true, forge.CINone), progress.AnnounceNone),
			want: []progress.State{done, done, done, done, todo},
		},
		"merging, its CI last read green": {
			work: branched(1, review(forge.StateMerging, false, forge.CIPassed), progress.AnnounceNone),
			want: []progress.State{done, done, done, wip, todo},
		},
		"merged and fetched, its commits now in the base": {
			work: branched(0, review(forge.StateMerged, false, forge.CINone), progress.AnnounceNone),
			want: []progress.State{done, done, done, done, todo},
		},
		"a post waiting for CI": {
			work: branched(1, review(forge.StateOpen, false, forge.CIPassed), progress.AnnounceUnderWay),
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

func TestEachStageNamesTheSystemItsWorkHappensIn(t *testing.T) {
	t.Parallel()

	// Act
	stages := progress.Stages(progress.Work{}, "Slack")

	// Assert
	systems := make([]progress.System, len(stages))
	for index, stage := range stages {
		systems[index] = stage.System
	}

	want := []progress.System{progress.Tracker, progress.Git, progress.Git, progress.Forge, progress.Messaging}
	if !slices.Equal(systems, want) {
		t.Errorf("stage systems = %v, want %v", systems, want)
	}
}

func TestEachStageSaysWhichStepOfTheLoopItIs(t *testing.T) {
	t.Parallel()

	// Act
	stages := progress.Stages(progress.Work{}, "Webhook")

	// Assert
	steps := make([]progress.Step, len(stages))
	for index, stage := range stages {
		steps[index] = stage.Step
	}

	want := []progress.Step{
		progress.StepIssue, progress.StepBranch, progress.StepCommits, progress.StepReview, progress.StepAnnounce,
	}
	if !slices.Equal(steps, want) {
		t.Errorf("stage steps = %v, want %v", steps, want)
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
		"merging, its merge in flight": {state: forge.StateMerging, want: progress.PullRequestMerging},
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

func TestAReviewWithNoPullRequestToFollowDropsWhatItsCIAndReviewersSaid(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pull  forge.PullRequest
		found bool
	}{
		"none found":             {pull: forge.PullRequest{State: forge.StateOpen, ChangesRequested: true}, found: false},
		"closed without merging": {pull: forge.PullRequest{State: forge.StateClosed, ChangesRequested: true}, found: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := progress.ReviewOf(tt.pull, tt.found, forge.CIFailed)

			// Assert
			if got != progress.NoReview() {
				t.Errorf("ReviewOf = %+v, want no review, its failed CI and changes asked for dropped", got)
			}
		})
	}
}

func TestAReviewOfAPullRequestFollowedKeepsItsCI(t *testing.T) {
	t.Parallel()

	// Arrange
	pull := forge.PullRequest{State: forge.StateOpen}

	// Act
	got := progress.ReviewOf(pull, true, forge.CIFailed)

	// Assert
	if got == progress.NoReview() {
		t.Errorf("ReviewOf = %+v, want the open pull request and its failed CI kept", got)
	}
}
