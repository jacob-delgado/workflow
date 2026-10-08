// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errCIUnread is a CI read that failed, which leaves the pull request without
// its CI.
var errCIUnread = errors.New("the forge did not answer for the checks")

// framedStages is the stages one frame of the stream carries over deps.
func framedStages(t *testing.T, deps webserver.Deps) []api.Stage {
	t.Helper()

	handler := serve(t, deps, config.Default())

	return firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String()).Stages
}

// stageStates is how far each of the loop's stages has got, by its step; a
// stage left out has not started.
type stageStates struct {
	issue, branch, commits, review, announce api.StageState
}

// loopStages is the loop's five stages in order, named as the server names
// them, the last for kind's service, each in the state states gives it.
func loopStages(kind config.MessagingKind, states stageStates) []api.Stage {
	stage := func(step api.StageStep, name string, state api.StageState) api.Stage {
		if state == "" {
			state = api.StageStateNotStarted
		}

		return api.Stage{Step: step, Name: name, State: state}
	}

	return []api.Stage{
		stage(api.StageStepIssue, "Issue", states.issue), stage(api.StageStepBranch, "Branch", states.branch),
		stage(api.StageStepCommits, "Commits", states.commits), stage(api.StageStepReview, "Review", states.review),
		stage(api.StageStepAnnounce, kind.Service(), states.announce),
	}
}

// stageOf is the frame's stage at step, failing the test when it has none.
func stageOf(t *testing.T, stages []api.Stage, step api.StageStep) api.Stage {
	t.Helper()

	at := slices.IndexFunc(stages, func(stage api.Stage) bool { return stage.Step == step })
	if at < 0 {
		t.Fatalf("stages = %+v, want the %s stage", stages, step)
	}

	return stages[at]
}

func TestSnapshotCarriesTheLoopsStagesInOrder(t *testing.T) {
	t.Parallel()

	// Act
	stages := framedStages(t, filledDeps())

	// Assert
	want := loopStages(config.KindSlack, stageStates{
		issue: api.StageStateDone, branch: api.StageStateDone, commits: api.StageStateDone, review: api.StageStateDone,
	})
	if !slices.Equal(stages, want) {
		t.Errorf("stages = %+v, want %+v", stages, want)
	}
}

func TestSnapshotStagesStartNowhereOnTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: prBase, Base: testBase}, nil }
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, nil }

	// Act
	stages := framedStages(t, deps)

	// Assert
	for _, step := range []api.StageStep{api.StageStepIssue, api.StageStepBranch, api.StageStepCommits} {
		if got := stageOf(t, stages, step).State; got != api.StageStateNotStarted {
			t.Errorf("%s stage = %s on the base branch, want not_started", step, got)
		}
	}
}

// How the commits stage reads, by progress.commitState: done on a commit,
// whatever is still to commit, and under way while there is only something to
// commit.
func TestSnapshotReadsTheCommitsStage(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		commits []gitrepo.Commit
		changes []gitrepo.Change
		want    api.StageState
	}{
		"done on a commit with files still to commit": {
			commits: []gitrepo.Commit{{Hash: filledHead, Subject: testCommitSubject}},
			changes: []gitrepo.Change{{Path: "internal/config/redact.go", Unstaged: 'M'}},
			want:    api.StageStateDone,
		},
		"in flight with only files to commit": {
			changes: []gitrepo.Change{{Path: "internal/config/redact.go", Unstaged: 'M'}},
			want:    api.StageStateInFlight,
		},
		"not started with nothing committed or to commit": {want: api.StageStateNotStarted},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{Name: testBranchName, Base: testBase, Head: filledHead, Commits: tt.commits}, nil
			}
			deps.Changes = func() ([]gitrepo.Change, error) { return tt.changes, nil }

			// Act
			stages := framedStages(t, deps)

			// Assert
			if got := stageOf(t, stages, api.StageStepCommits).State; got != tt.want {
				t.Errorf("commits stage = %s, want %s", got, tt.want)
			}
		})
	}
}

// reviewCase is the pull request the forge answers and how its CI reads, for
// the review stage: ciErr set, the CI is not read at all.
type reviewCase struct {
	pull  forge.PullRequest
	ci    forge.CIState
	ciErr error
	want  api.StageState
}

// openPull is pull request 42, open, ready for review, with nothing asked of
// it.
func openPull() forge.PullRequest {
	return forge.PullRequest{Number: 42, URL: pull42, Title: pullTitle, State: forge.StateOpen}
}

// changesAsked is openPull with changes requested.
func changesAsked() forge.PullRequest {
	pull := openPull()
	pull.ChangesRequested = true

	return pull
}

// inState is openPull in state, with changes once asked for when asked is set.
func inState(state forge.PullState, asked bool) forge.PullRequest {
	pull := openPull()
	pull.State, pull.ChangesRequested = state, asked

	return pull
}

// How the review stage reads, by progress.reviewState: in flight until CI
// passes, failed on a CI failure or changes asked for, done once CI passes or
// the pull request merges, and as no pull request at all once it is closed. A
// CI that could not be read counts as none, and a draft reads as any pull
// request.
func TestSnapshotReadsTheReviewStage(t *testing.T) {
	t.Parallel()

	draft := openPull()
	draft.Draft = true

	cases := map[string]reviewCase{
		"committed, review open, CI running": {
			pull: openPull(), ci: forge.CIRunning, want: api.StageStateInFlight,
		},
		"review open, no CI to pass": {pull: openPull(), ci: forge.CINone, want: api.StageStateInFlight},
		"review open, CI not read":   {pull: openPull(), ciErr: errCIUnread, want: api.StageStateInFlight},
		"its CI passed":              {pull: openPull(), ci: forge.CIPassed, want: api.StageStateDone},
		"its CI failed":              {pull: openPull(), ci: forge.CIFailed, want: api.StageStateFailed},
		"changes requested, with no CI, stop review reading done": {
			pull: changesAsked(), ci: forge.CINone, want: api.StageStateFailed,
		},
		"changes requested, CI not read, stop review reading done": {
			pull: changesAsked(), ciErr: errCIUnread, want: api.StageStateFailed,
		},
		"changes requested stops review reading done": {
			pull: changesAsked(), ci: forge.CIPassed, want: api.StageStateFailed,
		},
		"a draft whose CI passed, as any pull request": {pull: draft, ci: forge.CIPassed, want: api.StageStateDone},
		"merged, with no CI left to pass": {
			pull: inState(forge.StateMerged, false), ci: forge.CIRunning, want: api.StageStateDone,
		},
		"merged, the changes once asked for moot": {
			pull: inState(forge.StateMerged, true), ci: forge.CIFailed, want: api.StageStateDone,
		},
		"closed without merging, none": {
			pull: inState(forge.StateClosed, true), ci: forge.CIFailed, want: api.StageStateNotStarted,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return tt.pull, true, nil }
			deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
				return forge.CI{State: tt.ci, Total: 1}, tt.ciErr
			}

			// Act
			stages := framedStages(t, deps)

			// Assert
			if got := stageOf(t, stages, api.StageStepReview).State; got != tt.want {
				t.Errorf("review stage = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSnapshotReadsTheReviewStageNotStartedWithNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

	// Act
	stages := framedStages(t, deps)

	// Assert
	if got := stageOf(t, stages, api.StageStepReview).State; got != api.StageStateNotStarted {
		t.Errorf("review stage = %s with no pull request, want not_started", got)
	}
}

func TestSnapshotReadsTheAnnounceStageDoneOnceAnnouncedAtItsMoment(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}

	// Act
	stages := framedStages(t, memory.wire(filledDeps()))

	// Assert
	if got := stageOf(t, stages, api.StageStepAnnounce).State; got != api.StageStateDone {
		t.Errorf("announce stage = %s once announced, want done", got)
	}
}

func TestSnapshotReadsTheAnnounceStageInFlightWhileAnAnnouncementWaitsForCI(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)

	// Act
	stages := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String()).Stages

	// Assert
	if got := stageOf(t, stages, api.StageStepAnnounce).State; got != api.StageStateInFlight {
		t.Errorf("announce stage = %s while an announcement waits for CI, want in_flight", got)
	}
}

func TestSnapshotNamesTheAnnounceStageForTheService(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.Kind = config.KindTeams

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, filledDeps(), cfg), "/api/events").Body.String())

	// Assert
	if got := stageOf(t, snap.Stages, api.StageStepAnnounce).Name; got != "Teams" {
		t.Errorf("announce stage is named %q, want Teams, the service it posts to", got)
	}
}

// branchStagesOf is the stages a frame over deps gives each branch named for
// an issue, by the branch's name, beside the frame's own stages. filledDeps'
// Branch is on fix/PROJ-412, with a commit and an open pull request whose CI
// passed; feat/PROJ-500-metrics is not checked out.
func branchStagesOf(t *testing.T) (map[string][]api.Stage, []api.Stage) {
	t.Helper()

	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return []string{testBranchName, elsewhereBranch}, nil }
	cfg := config.Default()
	cfg.Jira.Project = testProject

	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, cfg), "/api/events").Body.String())

	byName := map[string][]api.Stage{}
	for _, branch := range snap.Branches {
		byName[branch.Name] = branch.Stages
	}

	return byName, snap.Stages
}

// elsewhereBranch is a branch named for an issue that is not checked out.
const elsewhereBranch = "feat/PROJ-500-metrics"

func TestSnapshotGivesTheCheckedOutBranchTheFramesStages(t *testing.T) {
	t.Parallel()

	// Act
	byName, framed := branchStagesOf(t)

	// Assert
	if got := byName[testBranchName]; len(got) == 0 || !slices.Equal(got, framed) {
		t.Errorf("checked-out branch's stages = %+v, want the frame's %+v", got, framed)
	}
}

func TestSnapshotReadsABranchNotCheckedOutAsPickedUpAndBranchedFor(t *testing.T) {
	t.Parallel()

	// Act
	byName, _ := branchStagesOf(t)

	// Assert
	// Its commits, pull request and announcement are read for the checked-out
	// branch alone, so none of the checked-out branch's progress is its own.
	want := loopStages(config.KindSlack, stageStates{issue: api.StageStateDone, branch: api.StageStateDone})
	if got := byName[elsewhereBranch]; !slices.Equal(got, want) {
		t.Errorf("%s's stages = %+v, want %+v", elsewhereBranch, got, want)
	}
}

func TestSnapshotReadsAnIssueWithNoBranchAsPickedAndNothingBegun(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.Kind = config.KindTeams

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, filledDeps(), cfg), "/api/events").Body.String())

	// Assert
	want := loopStages(config.KindTeams, stageStates{issue: api.StageStateInFlight})
	if !slices.Equal(snap.UnstartedStages, want) {
		t.Errorf("an unstarted issue's stages = %+v, want %+v", snap.UnstartedStages, want)
	}
}
