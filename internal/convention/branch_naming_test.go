// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// The pieces the branch-naming cases share, named so a repeated literal does
// not read as a coincidence.
const (
	bugType   = "Bug"
	bugKind   = "bug"
	bugBranch = "bugfix"
)

func TestBranchNamingFollowsAConfiguredTemplate(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		template      string
		defaultPrefix string
		prefixes      map[string]string
		slugLimit     int
		issueType     string
		key, summary  string
		want          string
	}{
		"the default is unchanged when nothing is configured": {
			issueType: bugType, key: projKey, summary: issueSummary,
			want: "fix/PROJ-412-fix-token-redaction",
		},
		"a template can put the key first": {
			template:  "{key}/{slug}",
			issueType: "Story", key: "PROJ-7", summary: "Add retries",
			want: "PROJ-7/add-retries",
		},
		"a type-to-prefix map governs the prefix": {
			prefixes:  map[string]string{bugKind: bugBranch},
			issueType: bugType, key: "OPS-11", summary: "Crash",
			want: "bugfix/OPS-11-crash",
		},
		"a type not in the map takes the default prefix": {
			prefixes: map[string]string{bugKind: bugBranch}, defaultPrefix: "chore",
			issueType: "Task", key: "OPS-12", summary: "Tidy",
			want: "chore/OPS-12-tidy",
		},
		"an empty summary leaves a clean name": {
			template:  "{prefix}/{key}-{slug}",
			issueType: bugType, key: "OPS-13", summary: "!!!",
			want: "fix/OPS-13",
		},
		"the type is matched without regard to case": {
			prefixes:  map[string]string{bugKind: bugBranch},
			issueType: "BUG", key: "OPS-14", summary: "x",
			want: "bugfix/OPS-14-x",
		},
		"a slug limit shortens the summary part": {
			slugLimit: 10,
			issueType: bugType, key: "OPS-15", summary: "Replace the retry loop",
			want: "fix/OPS-15-replace",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			naming := convention.NewBranchNaming(tt.template, tt.defaultPrefix, tt.prefixes, tt.slugLimit)

			// Act
			got := naming.Name(tt.issueType, tt.key, tt.summary)

			// Assert
			if got != tt.want {
				t.Errorf("Name = %q, want %q", got, tt.want)
			}

			err := convention.ValidateBranchName(got)
			if err != nil {
				t.Errorf("Name proposed %q, which git would refuse: %v", got, err)
			}
		})
	}
}

// Whatever shape the template makes, IssueKey must still find the key in it:
// everything else the interface does with a branch is derived from that.
func TestBranchNamingKeepsTheKeyFindable(t *testing.T) {
	t.Parallel()

	// Arrange
	naming := convention.NewBranchNaming("{key}--{slug}", "", nil, 0)

	// Act
	name := naming.Name("Story", "PROJ-99", "add retries")
	got, found := convention.IssueKey(name, "")

	// Assert
	if !found || got != "PROJ-99" {
		t.Errorf("IssueKey(%q) = %q, %v, want PROJ-99 found", name, got, found)
	}
}
