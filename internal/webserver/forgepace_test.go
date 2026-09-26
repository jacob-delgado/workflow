// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// otherBranchName is a branch checked out after testBranchName.
const otherBranchName = "fix/PROJ-2"

// forgeCounts is how many times a server asked the forge, by seam.
type forgeCounts struct {
	finds  int
	checks int
}

// counted is deps with its forge seams counting each read into counts.
func counted(deps webserver.Deps, counts *forgeCounts) webserver.Deps {
	find, check := deps.FindPull, deps.CheckCI
	deps.FindPull = func(branch string) (forge.PullRequest, bool, error) {
		counts.finds++

		return find(branch)
	}
	deps.CheckCI = func(pull forge.PullRequest, head string) (forge.CI, error) {
		counts.checks++

		return check(pull, head)
	}

	return deps
}

// branchesInTurn is a Branch seam that answers with each branch in turn, one a
// read, and stays on the last.
func branchesInTurn(branches ...gitrepo.Branch) func() (gitrepo.Branch, error) {
	reads := 0

	return func() (gitrepo.Branch, error) {
		branch := branches[min(reads, len(branches)-1)]
		reads++

		return branch, nil
	}
}

func TestStreamAsksTheForgeAtMostOncePerCIInterval(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ciInterval string
		step       time.Duration
		wantReads  int
	}{
		"frames within the default twenty seconds":  {step: 5 * time.Second, wantReads: 1},
		"the default twenty seconds between frames": {step: 20 * time.Second, wantReads: streamedFrames},
		"a configured minute, half of it between frames": {
			ciInterval: "1m", step: 30 * time.Second, wantReads: 2,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var counts forgeCounts

			cfg := config.Default()
			cfg.Timing.CIInterval = tt.ciInterval

			// Act
			pushed := snapshots(t, streamPaced(t, counted(filledDeps(), &counts), cfg, tt.step))

			// Assert
			if counts.finds != tt.wantReads || counts.checks != tt.wantReads {
				t.Errorf("asked the forge for the pull %d times and its CI %d times across %d frames, want %d each",
					counts.finds, counts.checks, len(pushed), tt.wantReads)
			}

			for i, snap := range pushed {
				if !snap.Review.Found || snap.Review.Ci == nil {
					t.Errorf("frame %d review = %+v, want the pull and its CI the forge last gave", i, snap.Review)
				}
			}
		})
	}
}

func TestStreamsShareOneForgeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two pages open on one server a moment apart: the second is served what
	// the first page's frame read.
	var counts forgeCounts

	deps := counted(filledDeps(), &counts)
	deps.Clock = paceStart
	handler := serve(t, deps, config.Default())
	streamOnce(t, handler, "/api/events")

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !snap.Review.Found || snap.Review.Ci == nil {
		t.Errorf("second page's review = %+v, want the pull and its CI the first page's frame read", snap.Review)
	}

	if counts.finds != 1 || counts.checks != 1 {
		t.Errorf("asked the forge for the pull %d times and its CI %d times for two pages, want once each",
			counts.finds, counts.checks)
	}
}

func TestStreamAsksTheForgeAgainForAnotherBranchOrHead(t *testing.T) {
	t.Parallel()

	cases := map[string][]gitrepo.Branch{
		"a new head on the branch": {
			{Name: testBranchName, Head: "abc1"}, {Name: testBranchName, Head: "abc2"}, {Name: testBranchName, Head: "abc3"},
		},
		"another branch checked out": {
			{Name: testBranchName, Head: testCommitHash},
			{Name: otherBranchName, Head: testCommitHash},
			{Name: "fix/PROJ-3", Head: testCommitHash},
		},
	}

	for name, branches := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// No time passes between frames: only what the forge was asked
			// about has changed.
			var counts forgeCounts

			deps := counted(filledDeps(), &counts)
			deps.Branch = branchesInTurn(branches...)

			// Act
			pushed := snapshots(t, streamPaced(t, deps, config.Default(), 0))

			// Assert
			if counts.finds != len(branches) || counts.checks != len(branches) {
				t.Errorf("asked the forge for the pull %d times and its CI %d times across %d frames, want %d each",
					counts.finds, counts.checks, len(pushed), len(branches))
			}
		})
	}
}

func TestStreamKeepsTheLastForgeAnswerWhenARefreshFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The interval passes between every two frames, so each asks again; only
	// the first ask is answered.
	asked := 0
	deps := filledDeps()
	find := deps.FindPull
	deps.FindPull = func(branch string) (forge.PullRequest, bool, error) {
		asked++
		if asked > 1 {
			return forge.PullRequest{}, false, errSeam
		}

		return find(branch)
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, config.Default(), 20*time.Second))

	// Assert
	if asked != streamedFrames {
		t.Fatalf("asked the forge for the pull %d times across %d frames, want once a frame", asked, len(pushed))
	}

	for i, snap := range pushed {
		if !snap.Review.Found || snap.Review.Pull == nil || snap.Review.Pull.Number != 42 || snap.Review.Ci == nil {
			t.Errorf("frame %d review = %+v, want #42 and its CI kept from the last answer", i, snap.Review)
		}
	}
}

func TestStreamKeepsTheLastCIWhenItsReadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The interval passes between every two frames, so each asks again; the
	// pull is found every time, and its CI only the first.
	checked := 0
	deps := filledDeps()
	check := deps.CheckCI
	deps.CheckCI = func(pull forge.PullRequest, head string) (forge.CI, error) {
		checked++
		if checked > 1 {
			return forge.CI{}, errSeam
		}

		return check(pull, head)
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, config.Default(), 20*time.Second))

	// Assert
	if checked != streamedFrames {
		t.Fatalf("asked the forge for CI %d times across %d frames, want once a frame", checked, len(pushed))
	}

	for i, snap := range pushed {
		review := snap.Review
		if review.Pull == nil || review.Pull.Number != 42 || review.Ci == nil || review.Ci.State != api.Passed {
			t.Errorf("frame %d review = %+v, want #42 and the CI that passed, kept from the last answer", i, review)
		}
	}
}

func TestStreamDropsTheLastCIForAPullItWasNotReadFor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		firstFound bool
		later      forge.PullRequest
	}{
		"the pull merged since, with no CI to ask about": {
			firstFound: true, later: forge.PullRequest{Number: 42, State: forge.StateMerged},
		},
		"another pull found for the branch": {
			firstFound: true, later: forge.PullRequest{Number: 43},
		},
		"a pull found where none was": {later: forge.PullRequest{Number: 42}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The first frame reads the forge whole; each later one, a frame
			// an interval on, finds later, whose CI cannot be read.
			finds := 0
			deps := filledDeps()
			find, check := deps.FindPull, deps.CheckCI
			deps.FindPull = func(branch string) (forge.PullRequest, bool, error) {
				finds++
				if finds > 1 {
					return tt.later, true, nil
				}

				pull, _, err := find(branch)

				return pull, tt.firstFound, err
			}
			deps.CheckCI = func(pull forge.PullRequest, head string) (forge.CI, error) {
				if finds > 1 {
					return forge.CI{}, errSeam
				}

				return check(pull, head)
			}

			// Act
			pushed := snapshots(t, streamPaced(t, deps, config.Default(), 20*time.Second))

			// Assert
			last := pushed[len(pushed)-1].Review
			if last.Pull == nil || last.Pull.Number != tt.later.Number || last.Ci != nil {
				t.Errorf("last frame's review = %+v, want #%d with no CI rather than one held for another",
					last, tt.later.Number)
			}
		})
	}
}

func TestStreamDropsTheLastCIForANewHead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The same pull is found at each head, but only the first head's CI can
	// be read: what CI said of that commit says nothing of the next.
	checks := 0
	deps := filledDeps()
	deps.Branch = branchesInTurn(gitrepo.Branch{Name: testBranchName, Head: "abc1"},
		gitrepo.Branch{Name: testBranchName, Head: "abc2"})
	check := deps.CheckCI
	deps.CheckCI = func(pull forge.PullRequest, head string) (forge.CI, error) {
		checks++
		if checks > 1 {
			return forge.CI{}, errSeam
		}

		return check(pull, head)
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, config.Default(), 0))

	// Assert
	if last := pushed[len(pushed)-1].Review; last.Pull == nil || last.Ci != nil {
		t.Errorf("last frame's review = %+v, want #42 with no CI rather than the first head's", last)
	}
}

func TestStreamWaitsOutTheIntervalAfterAFailedForgeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := 0
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		asked++

		return forge.PullRequest{}, false, errSeam
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, config.Default(), 5*time.Second))

	// Assert
	if asked != 1 {
		t.Errorf("asked a failing forge %d times across %d frames within one interval, want once", asked, len(pushed))
	}
}

func TestStreamDropsTheLastForgeAnswerForAnotherBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first branch's pull is found; the forge cannot answer for the next.
	first := gitrepo.Branch{Name: testBranchName, Head: testCommitHash}
	deps := filledDeps()
	deps.Branch = branchesInTurn(first, gitrepo.Branch{Name: otherBranchName, Head: testCommitHash})
	find := deps.FindPull
	deps.FindPull = func(branch string) (forge.PullRequest, bool, error) {
		if branch != first.Name {
			return forge.PullRequest{}, false, errSeam
		}

		return find(branch)
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, config.Default(), 0))

	// Assert
	if last := pushed[len(pushed)-1]; last.Review.Found {
		t.Errorf("last frame's review = %+v on %q, want none rather than %q's pull", last.Review, last.Branch.Name,
			first.Name)
	}
}

func TestStreamAsksTheForgeAgainOnceAPullIsOpenedHere(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge finds no pull request until the page opens one. The frame
	// before the open has been served; the one after must not be served that
	// answer for the rest of the interval.
	opened := false
	deps := openableDeps()
	create := deps.CreatePull
	deps.CreatePull = func(request forge.NewPullRequest) (forge.PullRequest, error) {
		opened = true

		return create(request)
	}
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 7, URL: prURL, Title: "opened"}, opened, nil
	}
	deps.Clock = paceStart

	handler := serve(t, deps, config.Default())
	streamOnce(t, handler, "/api/events")

	openAnswer := send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)
	if openAnswer.Code != http.StatusOK {
		t.Fatalf("opening the pull request: status = %d (%s), want 200", openAnswer.Code, openAnswer.Body.String())
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !snap.Review.Found || snap.Review.Pull == nil || snap.Review.Pull.Number != 7 {
		t.Errorf("review after the open = %+v, want #7, the pull just opened", snap.Review)
	}
}
