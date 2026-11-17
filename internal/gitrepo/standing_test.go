// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// standingName is the branch the standing cases ask about, pushed to origin,
// and its upstream when it tracks itself there.
const (
	standingName = "feat"
	ownUpstream  = gitrepo.DefaultRemote + "/" + standingName
)

// standingCommits are the three commits every standing case holds, oldest
// first.
func standingCommits() []gitrepo.Commit {
	return []gitrepo.Commit{{Hash: "a1", Subject: "one"}, {Hash: "b2", Subject: "two"}, {Hash: "c3", Subject: "three"}}
}

// counting is how git counted a branch against its upstream: not at all, level
// with it, or two ahead of it.
type counting struct {
	uncounted bool
	ahead     int
}

// standing is a branch on standingName tracking upstream, counted as count.
func standing(upstream string, count counting, truncated bool) gitrepo.Branch {
	return gitrepo.Branch{
		Name: standingName, Upstream: upstream, PushRemote: gitrepo.DefaultRemote,
		Ahead: count.ahead, Uncounted: count.uncounted, Commits: standingCommits(), Truncated: truncated,
	}
}

func TestWhereABranchStandsDecidesWhatIsPushed(t *testing.T) {
	t.Parallel()

	uncounted, level, twoAhead := counting{uncounted: true}, counting{}, counting{ahead: 2}
	all, lastTwo := standingCommits(), standingCommits()[1:]

	cases := map[string]struct {
		upstream  string
		count     counting
		truncated bool
		pushed    bool
		holdsAll  bool
		unpushed  []gitrepo.Commit
	}{
		"no upstream, uncounted":             {upstream: "", count: uncounted, unpushed: all},
		"no upstream, level":                 {upstream: "", count: level, unpushed: all},
		"no upstream, two ahead":             {upstream: "", count: twoAhead, unpushed: all},
		"no upstream, truncated":             {upstream: "", count: level, truncated: true},
		"tracks origin/main, uncounted":      {upstream: originMain, count: uncounted},
		"tracks origin/main, level":          {upstream: originMain, count: level, holdsAll: true},
		"tracks origin/main, two ahead":      {upstream: originMain, count: twoAhead, unpushed: lastTwo},
		"tracks origin/main, level, cut":     {upstream: originMain, count: level, truncated: true, holdsAll: true},
		"tracks origin/main, two ahead, cut": {upstream: originMain, count: twoAhead, truncated: true},
		"tracks itself, uncounted":           {upstream: ownUpstream, count: uncounted},
		"tracks itself, level":               {upstream: ownUpstream, count: level, pushed: true, holdsAll: true},
		"tracks itself, two ahead":           {upstream: ownUpstream, count: twoAhead, unpushed: lastTwo},
		"tracks itself, uncounted, cut":      {upstream: ownUpstream, count: uncounted, truncated: true},
		"tracks itself, level, cut": {
			upstream: ownUpstream, count: level, truncated: true, pushed: true, holdsAll: true,
		},
		"tracks itself, two ahead, cut": {upstream: ownUpstream, count: twoAhead, truncated: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			branch := standing(tt.upstream, tt.count, tt.truncated)

			// Act
			pushed, holdsAll, unpushed := branch.Pushed(), branch.UpstreamHoldsAll(), branch.Unpushed()

			// Assert
			if pushed != tt.pushed || holdsAll != tt.holdsAll || !slices.Equal(unpushed, tt.unpushed) {
				t.Errorf("Pushed, UpstreamHoldsAll, Unpushed = %v, %v, %v; want %v, %v, %v",
					pushed, holdsAll, unpushed, tt.pushed, tt.holdsAll, tt.unpushed)
			}
		})
	}
}

// A detached HEAD has no branch to have pushed, whatever its upstream says.
func TestADetachedHeadIsNeverPushed(t *testing.T) {
	t.Parallel()

	// Arrange
	branch := gitrepo.Branch{Detached: true, Upstream: gitrepo.DefaultRemote + "/", PushRemote: gitrepo.DefaultRemote}

	// Act & Assert
	if branch.Pushed() {
		t.Errorf("Pushed of a detached HEAD tracking %q = true, want false", branch.Upstream)
	}
}

func TestABranchTracksNothingAnotherBranchOrItsOwn(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		upstream string
		want     gitrepo.Tracking
	}{
		"no upstream":        {upstream: "", want: gitrepo.TracksNothing},
		"another branch":     {upstream: originMain, want: gitrepo.TracksOther},
		"where it is pushed": {upstream: ownUpstream, want: gitrepo.TracksOwn},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			branch := standing(tt.upstream, counting{}, false)

			// Act & Assert
			if got := branch.Tracking(); got != tt.want {
				t.Errorf("Tracking with upstream %q = %v, want %v", tt.upstream, got, tt.want)
			}
		})
	}
}

func TestABranchIsCountedNotAtAllUnknownLevelOrAhead(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		upstream string
		count    counting
		want     gitrepo.Counted
	}{
		"no upstream": {upstream: "", count: counting{ahead: 2}, want: gitrepo.CountNone},
		"uncounted":   {upstream: ownUpstream, count: counting{uncounted: true}, want: gitrepo.CountUnknown},
		"level":       {upstream: originMain, count: counting{}, want: gitrepo.CountLevel},
		"two ahead":   {upstream: originMain, count: counting{ahead: 2}, want: gitrepo.CountAhead},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			branch := standing(tt.upstream, tt.count, false)

			// Act & Assert
			if got := branch.Counted(); got != tt.want {
				t.Errorf("Counted of %+v = %v, want %v", tt.count, got, tt.want)
			}
		})
	}
}
