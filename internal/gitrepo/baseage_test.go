// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestABaseWithNoDateToReadIsLeftUndated(t *testing.T) {
	t.Parallel()

	cases := map[string]reply{
		"git cannot read the base's date": {err: errNoRef},
		// git writes a date past the year 9999 with five digits, which is not
		// RFC 3339, as it does for a commit whose committer set one.
		"a date past the year 9999": {out: []byte("10000-01-01T00:00:00Z\n")},
	}

	for name, baseDate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			replies := with(featureBranch(), map[string]reply{baseAge + originMain: baseDate})

			// Act
			branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

			// Assert
			if err != nil || branch.Base != originMain || !slices.Equal(branch.Commits, featureCommits()) {
				t.Fatalf("ReadBranch = %+v, %v; want the branch read on its base", branch, err)
			}

			if !branch.BaseUpdated.IsZero() {
				t.Errorf("BaseUpdated = %v, want no date for the base", branch.BaseUpdated)
			}
		})
	}
}
