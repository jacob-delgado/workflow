// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// mergeablePull is an open pull request ready to merge: no conflicts, one
// approval and no changes asked for.
func mergeablePull() forge.PullRequest {
	return forge.PullRequest{Number: 7, State: forge.StateOpen, Mergeable: forge.MergeClean, Approvals: 1}
}

// passedCI is CI that passed.
func passedCI() forge.CI {
	return forge.CI{State: forge.CIPassed}
}

// originMain is the base the feature branches here start from.
const originMain = "origin/main"

// featureBranch is a feature branch off origin/main with one commit, pushed.
func featureBranch() gitrepo.Branch {
	return gitrepo.Branch{
		Name: "feat/x", Base: originMain, Upstream: "origin/feat/x", PushRemote: "origin",
		Commits: []gitrepo.Commit{{Hash: "aaa1111", Subject: "feat: x"}},
	}
}

func TestCanMergeHoldsTheTerminalsRule(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pull func(forge.PullRequest) forge.PullRequest
		ci   forge.CI
		want bool
	}{
		"green, approved and clean": {pull: same, ci: passedCI(), want: true},
		"a draft": {pull: func(p forge.PullRequest) forge.PullRequest {
			p.Draft = true

			return p
		}, ci: passedCI(), want: false},
		"merged already": {pull: func(p forge.PullRequest) forge.PullRequest {
			p.State = forge.StateMerged

			return p
		}, ci: passedCI(), want: false},
		"with conflicts": {pull: func(p forge.PullRequest) forge.PullRequest {
			p.Mergeable = forge.MergeConflicts

			return p
		}, ci: passedCI(), want: false},
		"not approved": {pull: func(p forge.PullRequest) forge.PullRequest {
			p.Approvals = 0

			return p
		}, ci: passedCI(), want: false},
		"changes asked for": {pull: func(p forge.PullRequest) forge.PullRequest {
			p.ChangesRequested = true

			return p
		}, ci: passedCI(), want: false},
		"CI still running": {pull: same, ci: forge.CI{State: forge.CIRunning}, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.CanMerge(tt.pull(mergeablePull()), tt.ci)

			// Assert
			if got != tt.want {
				t.Errorf("CanMerge = %t, want %t", got, tt.want)
			}
		})
	}
}

// same is a pull request left as it was.
func same(pull forge.PullRequest) forge.PullRequest {
	return pull
}

func TestCanFinishOnlyAMergedBranchWithNothingUnpushed(t *testing.T) {
	t.Parallel()

	merged := forge.PullRequest{Number: 7, State: forge.StateMerged}
	ahead := featureBranch()
	ahead.Ahead = 1
	onBase := featureBranch()
	onBase.Name = targetBase
	noBase := featureBranch()
	noBase.Base = ""

	cases := map[string]struct {
		pull   forge.PullRequest
		branch gitrepo.Branch
		want   bool
	}{
		"merged and pushed":          {pull: merged, branch: featureBranch(), want: true},
		"still open":                 {pull: mergeablePull(), branch: featureBranch(), want: false},
		"a commit not pushed":        {pull: merged, branch: ahead, want: false},
		"on the base branch already": {pull: merged, branch: onBase, want: false},
		"no base to return to":       {pull: merged, branch: noBase, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.CanFinish(tt.pull, tt.branch)

			// Assert
			if got != tt.want {
				t.Errorf("CanFinish = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCanRerunOnlyFailedCIOnAnOpenPull(t *testing.T) {
	t.Parallel()

	failed := forge.CI{State: forge.CIFailed}

	cases := map[string]struct {
		pull forge.PullRequest
		ci   forge.CI
		want bool
	}{
		"open and failed":   {pull: mergeablePull(), ci: failed, want: true},
		"open and green":    {pull: mergeablePull(), ci: passedCI(), want: false},
		"merged and failed": {pull: forge.PullRequest{State: forge.StateMerged}, ci: failed, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.CanRerun(tt.pull, tt.ci)

			// Assert
			if got != tt.want {
				t.Errorf("CanRerun = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCanRebaseAFeatureBranchWithABase(t *testing.T) {
	t.Parallel()

	detached := featureBranch()
	detached.Name = ""
	noBase := featureBranch()
	noBase.Base = ""

	cases := map[string]struct {
		branch gitrepo.Branch
		want   bool
	}{
		"a feature branch": {branch: featureBranch(), want: true},
		"a detached HEAD":  {branch: detached, want: false},
		"no base":          {branch: noBase, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.CanRebase(tt.branch)

			// Assert
			if got != tt.want {
				t.Errorf("CanRebase = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFoldableIsTheUnpushedCommitsWhenSomethingIsStaged(t *testing.T) {
	t.Parallel()

	local := featureBranch()
	local.Upstream = ""
	pushed := featureBranch()

	cases := map[string]struct {
		changes []gitrepo.Change
		branch  gitrepo.Branch
		want    int
	}{
		"staged, with a local commit": {changes: []gitrepo.Change{stagedEdit()}, branch: local, want: 1},
		"nothing staged":              {changes: []gitrepo.Change{unstagedEdit()}, branch: local, want: 0},
		"staged, every commit pushed": {changes: []gitrepo.Change{stagedEdit()}, branch: pushed, want: 0},
		"staged, on no branch at all": {changes: []gitrepo.Change{stagedEdit()}, branch: gitrepo.Branch{}, want: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.Foldable(tt.changes, tt.branch)

			// Assert
			if len(got) != tt.want {
				t.Errorf("Foldable = %v, want %d commits", got, tt.want)
			}
		})
	}
}
