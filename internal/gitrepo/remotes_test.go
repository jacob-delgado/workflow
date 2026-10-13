// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// pushedBranch is the branch the push remote cases ask about, and
// readBranchPushRemote the key git keeps its own push remote under.
const (
	pushedBranch         = "feat/token"
	readBranchPushRemote = "git -C /work config --get branch." + pushedBranch + ".pushRemote"
	readBranchRemote     = "git -C /work config --get branch." + pushedBranch + ".remote"
)

func TestPushRemoteReadsTheBranchKeyBeforeTheDefault(t *testing.T) {
	t.Parallel()

	none := reply{err: errNoRef}

	cases := map[string]struct {
		own, pushDefault, tracked reply
		want                      string
	}{
		"the branch's own push remote": {
			own: reply{out: []byte(forkRemote + "\n")}, pushDefault: reply{out: []byte("elsewhere\n")},
			tracked: none, want: forkRemote,
		},
		"the push default over the remote it tracks": {
			own: none, pushDefault: reply{out: []byte(forkRemote + "\n")},
			tracked: reply{out: []byte("elsewhere\n")}, want: forkRemote,
		},
		// git push -u fork feat records branch.feat.remote, and a plain push
		// goes there after.
		"only the remote it tracks": {
			own: none, pushDefault: none, tracked: reply{out: []byte(forkRemote + "\n")}, want: forkRemote,
		},
		"none of them": {own: none, pushDefault: none, tracked: none, want: gitrepo.DefaultRemote},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(fakeRunner(t, map[string]reply{
				readBranchPushRemote: tt.own,
				readPushDefault:      tt.pushDefault,
				readBranchRemote:     tt.tracked,
			}), workDir)

			// Act
			got := repo.PushRemote(t.Context(), pushedBranch)

			// Assert
			if got != tt.want {
				t.Errorf("PushRemote = %q, want %q", got, tt.want)
			}
		})
	}
}

// A detached HEAD has no branch to keep a push remote under, so only the
// repository's default is asked; fakeRunner fails any other command.
func TestADetachedHeadAsksOnlyForThePushDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{readPushDefault: {out: []byte(forkRemote + "\n")}}), workDir)

	// Act
	got := repo.PushRemote(t.Context(), "")

	// Assert
	if got != forkRemote {
		t.Errorf("PushRemote of a detached HEAD = %q, want %q", got, forkRemote)
	}
}
