// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
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

// mergedHead is the commit a feature branch here is at, and the one its pull
// request merged; committedSince is one made on it after.
const (
	mergedHead     = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	committedSince = "c0ffee0c0ffee0c0ffee0c0ffee0c0ffee0c0ffe"
)

// featureBranch is a feature branch off origin/main with one commit, pushed.
func featureBranch() gitrepo.Branch {
	return gitrepo.Branch{
		Name: "feat/x", Head: mergedHead, Base: originMain, Upstream: "origin/feat/x", PushRemote: "origin",
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

func TestRefuseFinishSaysWhyABranchCannotBeFinished(t *testing.T) {
	t.Parallel()

	merged := forge.PullRequest{Number: 7, State: forge.StateMerged, HeadCommit: mergedHead}
	closed := forge.PullRequest{Number: 7, State: forge.StateClosed}
	unpublished := featureBranch()
	unpublished.Upstream = ""
	// The forge deleted the branch after the merge and a prune dropped its
	// upstream; a commit made since is on no remote at all.
	upstreamGone := unpublished
	upstreamGone.Head = committedSince
	ahead := featureBranch()
	ahead.Head, ahead.Ahead = committedSince, 1
	// A branch tracking the base is ahead of it by every commit a squash
	// merge rewrote, yet at the very commit that merged.
	tracksTheBase := featureBranch()
	tracksTheBase.Upstream, tracksTheBase.Ahead = originMain, 1
	silent := merged
	silent.HeadCommit = ""
	onBase := featureBranch()
	onBase.Name = targetBase
	detached := featureBranch()
	detached.Name = ""
	noBase := featureBranch()
	noBase.Base = ""

	cases := map[string]struct {
		pull   forge.PullRequest
		found  bool
		branch gitrepo.Branch
		want   error
	}{
		"merged and pushed":           {pull: merged, found: true, branch: featureBranch(), want: nil},
		"at what merged, no upstream": {pull: merged, found: true, branch: unpublished, want: nil},
		"at what merged, ahead":       {pull: merged, found: true, branch: tracksTheBase, want: nil},
		"pushed, the forge silent":    {pull: silent, found: true, branch: featureBranch(), want: nil},
		"still open":                  {pull: mergeablePull(), found: true, branch: featureBranch(), want: loop.ErrNotMerged},
		"none found":                  {pull: forge.PullRequest{}, branch: featureBranch(), want: loop.ErrNotMerged},
		"closed without merging":      {pull: closed, found: true, branch: featureBranch(), want: loop.ErrNotMerged},
		"a commit not pushed":         {pull: merged, found: true, branch: ahead, want: loop.ErrUnpushedWork},
		"upstream gone, committed since": {
			pull: merged, found: true, branch: upstreamGone, want: loop.ErrUnpushedWork,
		},
		"no upstream, the forge silent": {
			pull: silent, found: true, branch: unpublished, want: loop.ErrUnpushedWork,
		},
		"on the base branch already": {pull: merged, found: true, branch: onBase, want: loop.ErrNotOnFeatureBranch},
		"a detached HEAD":            {pull: merged, found: true, branch: detached, want: loop.ErrNotOnFeatureBranch},
		"no base to return to":       {pull: merged, found: true, branch: noBase, want: loop.ErrNoBase},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := loop.RefuseFinish(tt.pull, tt.found, tt.branch)

			// Assert
			if !errors.Is(got, tt.want) {
				t.Errorf("RefuseFinish = %v, want %v", got, tt.want)
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
