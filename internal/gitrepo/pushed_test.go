// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// forkRemote is a remote a repository can name in remote.pushDefault.
const forkRemote = "fork"

func TestABranchIsPushedWhenItsPushRemoteHasEverything(t *testing.T) {
	t.Parallel()

	const name = "fix/PROJ-412-token-redaction"

	cases := map[string]struct {
		pushDefault reply
		upstream    string
		pushed      bool
	}{
		// A push goes to remote.pushDefault and makes that remote's branch the
		// upstream, so a branch level with it is on the remote it pushes to.
		"on the push default remote": {
			pushDefault: reply{out: []byte(forkRemote + "\n")},
			upstream:    forkRemote + "/" + name,
			pushed:      true,
		},
		"on origin, with no push default": {
			pushDefault: reply{err: errNoRef},
			upstream:    gitrepo.DefaultRemote + "/" + name,
			pushed:      true,
		},
		// The next push still goes to the push default, which does not hold it.
		"on origin, while pushes go elsewhere": {
			pushDefault: reply{out: []byte(forkRemote + "\n")},
			upstream:    gitrepo.DefaultRemote + "/" + name,
			pushed:      false,
		},
	}

	for caseName, tt := range cases {
		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			// Arrange
			replies := with(featureBranch(), map[string]reply{
				readPushDefault: tt.pushDefault,
				readUpstream:    {out: []byte(tt.upstream + "\n")},
				countAhead:      {out: []byte("0\t0\n")},
			})

			// Act
			branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

			// Assert
			if err != nil || branch.Upstream != tt.upstream || branch.Pushed() != tt.pushed {
				t.Errorf("ReadBranch = %+v, %v; want upstream %q, pushed %v", branch, err, tt.upstream, tt.pushed)
			}
		})
	}
}
