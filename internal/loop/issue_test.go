// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// namedBranch is a branch named for its issue, and unnamedBranch one named
// for nothing, as work begun outside workflow often is.
const (
	namedBranch   = "fix/PROJ-1-thing"
	unnamedBranch = "my-thing"
)

func TestBranchIssueFindsTheIssueWhereverItWasNamed(t *testing.T) {
	t.Parallel()

	pull := &forge.PullRequest{Title: "Speed up search", Body: "Closes #42"}
	cases := map[string]struct {
		source loop.IssueSource
		want   string
		origin loop.IssueOrigin
	}{
		"a link wins over the name": {
			source: loop.IssueSource{Branch: namedBranch, Link: "PROJ-7", Pull: pull},
			want:   "PROJ-7", origin: loop.OriginLink,
		},
		"the name wins over the pull request": {
			source: loop.IssueSource{Branch: namedBranch, Pull: pull}, want: "PROJ-1", origin: loop.OriginBranch,
		},
		"the pull request when nothing else names one": {
			source: loop.IssueSource{Branch: unnamedBranch, Pull: pull}, want: "42", origin: loop.OriginPull,
		},
		"a link that names no issue is passed over": {
			source: loop.IssueSource{Branch: namedBranch, Link: "soon"}, want: "PROJ-1", origin: loop.OriginBranch,
		},
		"nothing names one": {source: loop.IssueSource{Branch: unnamedBranch}, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			ref, origin, found := loop.BranchIssue(tt.source)

			// Assert
			if ref.Key != tt.want || found != (tt.want != "") || (found && origin != tt.origin) {
				t.Errorf("BranchIssue = %+v from %v, %v; want %q from %v", ref, origin, found, tt.want, tt.origin)
			}
		})
	}
}

func TestBranchIssueKeepsALinkedIssuesTracker(t *testing.T) {
	t.Parallel()

	// Act
	ref, _, _ := loop.BranchIssue(loop.IssueSource{Branch: unnamedBranch, Link: "#57"})

	// Assert
	if ref != (convention.IssueRef{Key: "57", Tracker: convention.TrackerForge}) {
		t.Errorf("BranchIssue = %+v, want the forge's 57", ref)
	}
}
