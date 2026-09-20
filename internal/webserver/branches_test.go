// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
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
	cfg.Jira.Project = "PROJ"

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
			cfg.Jira.Project = "PROJ"

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
