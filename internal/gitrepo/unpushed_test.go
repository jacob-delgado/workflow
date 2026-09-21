// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestUnpushedAreTheCommitsNotYetOnTheUpstream(t *testing.T) {
	t.Parallel()

	first := gitrepo.Commit{Hash: "aaa", Subject: "one"}
	second := gitrepo.Commit{Hash: "bbb", Subject: "two"}
	third := gitrepo.Commit{Hash: "ccc", Subject: "three"}
	upstream := "origin/x"

	cases := map[string]struct {
		branch gitrepo.Branch
		want   []gitrepo.Commit
	}{
		"never pushed, so all are unpushed": {
			branch: gitrepo.Branch{Upstream: "", Commits: []gitrepo.Commit{first, second}},
			want:   []gitrepo.Commit{first, second},
		},
		"only the last ahead are unpushed": {
			branch: gitrepo.Branch{Upstream: upstream, Ahead: 2, Commits: []gitrepo.Commit{first, second, third}},
			want:   []gitrepo.Commit{second, third},
		},
		"nothing ahead, nothing unpushed": {
			branch: gitrepo.Branch{Upstream: upstream, Ahead: 0, Commits: []gitrepo.Commit{first}},
			want:   nil,
		},
		"a truncated history is not trusted": {
			// The retained commits are the oldest; the unpushed ones were dropped,
			// so the slice cannot say which are local — offer none.
			branch: gitrepo.Branch{
				Upstream: upstream, Ahead: 2, Truncated: true,
				Commits: []gitrepo.Commit{first, second, third},
			},
			want: nil,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.branch.Unpushed()

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("Unpushed() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
