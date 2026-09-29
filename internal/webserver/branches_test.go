// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestSnapshotListsTaskBranchesMarkingTheCheckedOutOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' Branch is on fix/PROJ-412, so that is the one in flight on HEAD.
	deps := filledDeps()
	deps.Branches = func() ([]string, error) {
		return []string{testBranchName, "feat/PROJ-500-metrics", "main"}, nil
	}
	cfg := config.Default()
	cfg.Jira.Project = testProject

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, cfg), "/api/events").Body.String())

	// Assert
	byKey := map[string]api.TaskBranch{}
	for _, branch := range snap.Branches {
		byKey[branch.IssueKey] = branch
	}

	if len(byKey) != 2 {
		t.Fatalf("branches = %+v, want two issue branches (main excluded)", snap.Branches)
	}

	if current := byKey["PROJ-412"]; current.Name != testBranchName || !current.Current {
		t.Errorf("PROJ-412 branch = %+v, want name fix/PROJ-412 marked current", current)
	}

	if other := byKey["PROJ-500"]; other.Name != "feat/PROJ-500-metrics" || other.Current {
		t.Errorf("PROJ-500 branch = %+v, want name feat/PROJ-500-metrics not current", other)
	}
}

func TestSnapshotBranchesAreEmptyWhenListingFails(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return nil, errSeam }

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if len(snap.Branches) != 0 {
		t.Errorf("branches = %+v, want none when listing the branches fails", snap.Branches)
	}
}

func TestNoTaskBranchIsCurrentWhenTheCheckedOutBranchIsUnknown(t *testing.T) {
	t.Parallel()

	// The checked-out branch is unknown either because the read failed or because
	// there is no branch seam; neither must mark a listed branch as current.
	cases := map[string]func() (gitrepo.Branch, error){
		"the branch read fails":   func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam },
		"there is no branch seam": nil,
	}

	for name, branch := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Branch = branch
			deps.Branches = func() ([]string, error) { return []string{testBranchName}, nil }
			cfg := config.Default()
			cfg.Jira.Project = testProject

			// Act
			snap := firstSnapshot(t, streamOnce(t, serve(t, deps, cfg), "/api/events").Body.String())

			// Assert
			if len(snap.Branches) != 1 {
				t.Fatalf("branches = %+v, want the one listed branch present", snap.Branches)
			}

			if snap.Branches[0].Current {
				t.Errorf("branch %q marked current, want none when the checked-out branch is unknown", snap.Branches[0].Name)
			}
		})
	}
}

func TestSnapshotBranchesAreAnEmptyArrayWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branches = nil

	// Act
	body := streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String()

	// Assert
	// The wire value is an empty array, never null, so the typed client and the
	// stream schema both see an array as the contract promises.
	if !strings.Contains(body, `"branches":[]`) {
		t.Errorf("stream body did not carry branches as an empty array: %q", body)
	}
}

// Branches the scoping tests list: two local ones and one only the remote has,
// each naming an issue of the PROJ project, and the issue the tracker says is
// yours of the local ones.
const (
	yourIssue        = jira.Key("PROJ-1")
	yourLocalBranch  = "fix/PROJ-1"
	otherLocalBranch = "fix/PROJ-2"
	yourRemoteBranch = "feat/PROJ-3-x"
	newLocalBranch   = "fix/PROJ-9"
	keyInYourSearch  = "key in ("
	minuteAndABit    = time.Minute + time.Second
	listedTooMany    = "branches = %+v, want only %v"
	listedEveryIssue = "branches = %+v, want every branch naming an issue, %v"
)

// scopedDeps is filledDeps with the scoping tests' branches, local and remote,
// and a lenient search that answers the keys in yours as assigned, or fails
// with fail, counting each ask.
func scopedDeps(yours []jira.Key, fail error, asked *atomic.Int32) webserver.Deps {
	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return []string{yourLocalBranch, otherLocalBranch}, nil }
	deps.RemoteBranches = func() ([]string, error) { return []string{yourLocalBranch, yourRemoteBranch}, nil }
	deps.SearchLenient = func(jql string, _ int) (jira.SearchResult, error) {
		if strings.HasPrefix(jql, keyInYourSearch) {
			asked.Add(1)
		}

		issues := make([]jira.Issue, 0, len(yours))
		for _, key := range yours {
			issues = append(issues, jira.Issue{Key: key})
		}

		return jira.SearchResult{Issues: issues, Total: len(issues)}, fail
	}

	return deps
}

// scopedConfig is the default configuration on the PROJ project.
func scopedConfig() config.Config {
	cfg := config.Default()
	cfg.Jira.Project = testProject

	return cfg
}

// listed is each branch as name, and "(remote)" after one only the remote has.
func listed(branches []api.TaskBranch) []string {
	names := make([]string, 0, len(branches))
	for _, branch := range branches {
		name := branch.Name
		if branch.Remote != nil && *branch.Remote {
			name += " (remote)"
		}

		names = append(names, name)
	}

	return names
}

func TestSnapshotBranchesAreOnlyThoseForYourIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	deps := scopedDeps([]jira.Key{yourIssue, "PROJ-3"}, nil, &asked)

	// Act
	body := streamOnce(t, serve(t, deps, scopedConfig()), "/api/events").Body.String()

	// Assert
	want := []string{yourLocalBranch, yourRemoteBranch + " (remote)"}
	if got := listed(firstSnapshot(t, body).Branches); !slices.Equal(got, want) {
		t.Errorf(listedTooMany, got, want)
	}

	if !strings.Contains(body, `"remote":false`) {
		t.Errorf("body = %s, want a local branch's remote sent as false, which the client's schema defaults it to", body)
	}
}

func TestSnapshotBranchesKeepTheCheckedOutBranchWhoseIssueIsNotYours(t *testing.T) {
	t.Parallel()

	// Arrange
	// An unassigned issue from a view, or one moved to done, is still the one
	// being worked on while its branch is checked out.
	var asked atomic.Int32

	deps := scopedDeps([]jira.Key{yourIssue}, nil, &asked)
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: otherLocalBranch}, nil }

	// Act
	body := streamOnce(t, serve(t, deps, scopedConfig()), "/api/events").Body.String()

	// Assert
	branches := firstSnapshot(t, body).Branches

	checkedOut := slices.IndexFunc(branches, func(branch api.TaskBranch) bool { return branch.Name == otherLocalBranch })
	if checkedOut < 0 || !branches[checkedOut].Current {
		t.Errorf("branches = %+v, want the checked-out %s kept and marked current", listed(branches), otherLocalBranch)
	}
}

func TestSnapshotBranchesAskTheTrackerOncePerKeySet(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	handler := serve(t, scopedDeps([]jira.Key{yourIssue}, nil, &asked), scopedConfig())
	streamOnce(t, handler, "/api/events")

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if asked.Load() != 1 {
		t.Errorf("asked the tracker %d times for two frames over the same branches, want once", asked.Load())
	}

	if got := listed(snap.Branches); !slices.Equal(got, []string{yourLocalBranch}) {
		t.Errorf(listedTooMany, got, []string{yourLocalBranch})
	}
}

func TestSnapshotBranchesAskTheTrackerAgain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		between  time.Duration
		branches [][]string
	}{
		"once a minute has passed": {
			between: minuteAndABit, branches: [][]string{{yourLocalBranch}, {yourLocalBranch}, {yourLocalBranch}},
		},
		"when the branches name other issues": {
			branches: [][]string{{yourLocalBranch}, {otherLocalBranch}, {yourLocalBranch, otherLocalBranch}},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var asked atomic.Int32

			deps := scopedDeps([]jira.Key{yourIssue}, nil, &asked)
			deps.RemoteBranches = nil
			frame := 0
			deps.Branches = func() ([]string, error) {
				names := tt.branches[min(frame, len(tt.branches)-1)]
				frame++

				return names, nil
			}

			// Act
			pushed := snapshots(t, streamPaced(t, deps, scopedConfig(), tt.between))

			// Assert
			if asked.Load() != streamedFrames {
				t.Errorf("asked the tracker %d times over %d frames, want %d", asked.Load(), len(pushed), streamedFrames)
			}
		})
	}
}

func TestSnapshotBranchesFollowTheTrackersLatestAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each frame's branches name other issues, so each is asked about; the last
	// answer covers PROJ-2 and says it is not yours.
	var asked atomic.Int32

	deps := scopedDeps([]jira.Key{yourIssue}, nil, &asked)
	deps.RemoteBranches = nil
	frames := [][]string{{yourLocalBranch}, {otherLocalBranch}, {yourLocalBranch, otherLocalBranch}}
	frame := 0
	deps.Branches = func() ([]string, error) {
		names := frames[min(frame, len(frames)-1)]
		frame++

		return names, nil
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, scopedConfig(), 0))

	// Assert
	want := []string{yourLocalBranch}
	if got := listed(pushed[len(pushed)-1].Branches); !slices.Equal(got, want) {
		t.Errorf("last frame's branches = %v, want %v by the tracker's latest answer", got, want)
	}
}

func TestSnapshotBranchesListEveryIssueBranchWhenTheTrackerCannotBeAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	deps := scopedDeps(nil, jira.ErrUnreachable, &asked)

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, scopedConfig()), "/api/events").Body.String())

	// Assert
	want := []string{yourLocalBranch, otherLocalBranch, yourRemoteBranch + " (remote)"}
	if got := listed(snap.Branches); !slices.Equal(got, want) {
		t.Errorf(listedEveryIssue, got, want)
	}
}

func TestSnapshotBranchesAskATrackerThatFailedOnceAMinute(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	deps := scopedDeps(nil, jira.ErrUnreachable, &asked)

	// Act
	pushed := snapshots(t, streamPaced(t, deps, scopedConfig(), 0))

	// Assert
	if asked.Load() != 1 {
		t.Errorf("asked a failing tracker %d times over %d frames within the minute, want once", asked.Load(), len(pushed))
	}

	want := []string{yourLocalBranch, otherLocalBranch, yourRemoteBranch + " (remote)"}
	if got := listed(pushed[len(pushed)-1].Branches); !slices.Equal(got, want) {
		t.Errorf(listedEveryIssue, got, want)
	}
}

func TestSnapshotBranchesCountTheIssueOfANewBranchAsYoursWhileTheTrackerIsDown(t *testing.T) {
	t.Parallel()

	// Arrange
	// The tracker answered for PROJ-1 (yours) and PROJ-2 (not), then failed once
	// fix/PROJ-9 was made: PROJ-9 counts as yours, as every issue does before
	// any answer, and the other two keep the answer given.
	var asked atomic.Int32

	deps := scopedDeps([]jira.Key{yourIssue}, nil, &asked)
	deps.RemoteBranches = nil
	answered := deps.SearchLenient
	deps.SearchLenient = func(jql string, startAt int) (jira.SearchResult, error) {
		if asked.Load() >= 1 {
			asked.Add(1)

			return jira.SearchResult{}, jira.ErrUnreachable
		}

		return answered(jql, startAt)
	}
	frame := 0
	deps.Branches = func() ([]string, error) {
		frame++
		if frame == 1 {
			return []string{yourLocalBranch, otherLocalBranch}, nil
		}

		return []string{yourLocalBranch, otherLocalBranch, newLocalBranch}, nil
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, scopedConfig(), 0))

	// Assert
	if asked.Load() < 2 {
		t.Fatalf("asked the tracker %d times, want it asked again once the branches named another issue", asked.Load())
	}

	want := []string{yourLocalBranch, newLocalBranch}
	if got := listed(pushed[len(pushed)-1].Branches); !slices.Equal(got, want) {
		t.Errorf("last frame's branches = %v, want %v", got, want)
	}
}

func TestSnapshotBranchesKeepTheLastAnswerWhenALaterAskFails(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	deps := scopedDeps([]jira.Key{yourIssue}, nil, &asked)
	answered := deps.SearchLenient
	deps.SearchLenient = func(jql string, startAt int) (jira.SearchResult, error) {
		if asked.Load() >= 1 {
			asked.Add(1)

			return jira.SearchResult{}, jira.ErrUnreachable
		}

		return answered(jql, startAt)
	}

	// Act
	pushed := snapshots(t, streamPaced(t, deps, scopedConfig(), minuteAndABit))

	// Assert
	if asked.Load() < 2 {
		t.Fatalf("asked the tracker %d times, want it asked again after a minute", asked.Load())
	}

	if got := listed(pushed[len(pushed)-1].Branches); !slices.Equal(got, []string{yourLocalBranch}) {
		t.Errorf("last frame's branches = %v, want the tracker's last answer, %v", got, []string{yourLocalBranch})
	}
}

func TestSnapshotBranchesAreTheLocalOnesWhenTheRemoteCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return []string{yourLocalBranch}, nil }
	deps.RemoteBranches = func() ([]string, error) { return nil, errSeam }

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, scopedConfig()), "/api/events").Body.String())

	// Assert
	if got := listed(snap.Branches); !slices.Equal(got, []string{yourLocalBranch}) {
		t.Errorf(listedEveryIssue, got, []string{yourLocalBranch})
	}
}
