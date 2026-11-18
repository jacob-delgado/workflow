// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// readStopped asks git where the files are that say an operation is stopped
// part way, in the order they decide it.
const readStopped = "git -C /work rev-parse --path-format=absolute" +
	" --git-path rebase-merge/head-name --git-path rebase-apply/head-name" +
	" --git-path MERGE_HEAD --git-path CHERRY_PICK_HEAD --git-path REVERT_HEAD"

// The paths readStopped names, relative to the git dir.
const (
	mergeRebaseHead = "rebase-merge/head-name"
	applyRebaseHead = "rebase-apply/head-name"
	mergeHead       = "MERGE_HEAD"
	cherryPickHead  = "CHERRY_PICK_HEAD"
	revertHead      = "REVERT_HEAD"
)

// stoppedMarkers are the paths readStopped names, in its order.
func stoppedMarkers() []string {
	return []string{mergeRebaseHead, applyRebaseHead, mergeHead, cherryPickHead, revertHead}
}

// featureName is the branch featureBranch checks out, and stoppedAt the
// commit a stopped merge, cherry-pick or revert names.
const (
	featureName = "fix/PROJ-412-token-redaction"
	stoppedAt   = firstHash + "\n"
)

// nothingStopped answers readStopped with paths under a git dir that does
// not exist, so no operation reads as stopped.
func nothingStopped() reply {
	return markersUnder("/work/.git")
}

// markersUnder answers readStopped with the markers' paths under gitDir.
func markersUnder(gitDir string) reply {
	paths := make([]string, 0, len(stoppedMarkers()))
	for _, marker := range stoppedMarkers() {
		paths = append(paths, filepath.Join(gitDir, marker))
	}

	return reply{out: []byte(strings.Join(paths, "\n") + "\n")}
}

// stoppedIn is a git dir holding files, each name a path under it and each
// value what the file holds, answered as readStopped's reply.
func stoppedIn(t *testing.T, files map[string]string) reply {
	t.Helper()

	gitDir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(gitDir, name)

		err := os.MkdirAll(filepath.Dir(path), 0o700)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return markersUnder(gitDir)
}

func TestEachOperationNamesTheGitCommandThatEndsIt(t *testing.T) {
	t.Parallel()

	cases := map[gitrepo.Operation]string{
		gitrepo.OperationNone:       "",
		gitrepo.OperationRebase:     "rebase",
		gitrepo.OperationMerge:      "merge",
		gitrepo.OperationCherryPick: "cherry-pick",
		gitrepo.OperationRevert:     "revert",
	}

	for operation, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := operation.Command(); got != want {
				t.Errorf("Command() = %q, want %q", got, want)
			}
		})
	}
}

func TestABranchReadNamesTheOperationStoppedPartWay(t *testing.T) {
	t.Parallel()

	const headName = "refs/heads/" + featureName + "\n"

	cases := map[string]struct {
		files     map[string]string
		operation gitrepo.Operation
		rebasing  string
	}{
		"nothing stopped": {files: nil, operation: gitrepo.OperationNone, rebasing: ""},
		"a rebase stopped, under the merge backend": {
			files:     map[string]string{mergeRebaseHead: headName},
			operation: gitrepo.OperationRebase,
			rebasing:  featureName,
		},
		"a rebase stopped, under the apply backend": {
			files:     map[string]string{applyRebaseHead: headName},
			operation: gitrepo.OperationRebase,
			rebasing:  featureName,
		},
		"a rebase of a detached HEAD stopped": {
			files:     map[string]string{mergeRebaseHead: "detached HEAD\n"},
			operation: gitrepo.OperationRebase,
			rebasing:  "",
		},
		// git rebase -r stopped at a merge leaves the merge's head beside the
		// rebase's, and the rebase is what --continue and --abort end.
		"a rebase stopped at a merge it replays": {
			files:     map[string]string{mergeRebaseHead: headName, mergeHead: stoppedAt},
			operation: gitrepo.OperationRebase,
			rebasing:  featureName,
		},
		"a merge stopped": {
			files:     map[string]string{mergeHead: stoppedAt},
			operation: gitrepo.OperationMerge,
			rebasing:  "",
		},
		"a cherry-pick stopped": {
			files:     map[string]string{cherryPickHead: stoppedAt},
			operation: gitrepo.OperationCherryPick,
			rebasing:  "",
		},
		"a revert stopped": {
			files:     map[string]string{revertHead: stoppedAt},
			operation: gitrepo.OperationRevert,
			rebasing:  "",
		},
		// git am applies its patches under rebase-apply too, without a head-name.
		"an am stopped, which is no rebase": {
			files:     map[string]string{"rebase-apply/patch": "diff\n"},
			operation: gitrepo.OperationNone,
			rebasing:  "",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(fakeRunner(t, with(featureBranch(), map[string]reply{
				readStopped: stoppedIn(t, tt.files),
			})), workDir)

			// Act
			branch, err := repo.ReadBranch(t.Context())

			// Assert
			if err != nil || branch.Operation != tt.operation || branch.Rebasing != tt.rebasing {
				t.Errorf("ReadBranch = operation %v rebasing %q, %v; want %v rebasing %q",
					branch.Operation, branch.Rebasing, err, tt.operation, tt.rebasing)
			}
		})
	}
}

func TestABranchWhoseStoppedOperationCannotBeReadReadsAsNoneStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, with(featureBranch(), map[string]reply{
		readStopped: {err: errNotARepository},
	})), workDir)

	// Act
	branch, err := repo.ReadBranch(t.Context())

	// Assert
	if err != nil || branch.Operation != gitrepo.OperationNone || branch.Name == "" {
		t.Errorf("ReadBranch = %+v, %v; want the branch read with nothing stopped", branch, err)
	}
}

func TestDescribeNamesTheOperationStoppedPartWay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		files     map[string]string
		operation gitrepo.Operation
		rebasing  string
	}{
		"a rebase": {
			files:     map[string]string{mergeRebaseHead: "refs/heads/feat/token-redaction\n"},
			operation: gitrepo.OperationRebase,
			rebasing:  "feat/token-redaction",
		},
		"a merge": {
			files:     map[string]string{mergeHead: stoppedAt},
			operation: gitrepo.OperationMerge,
			rebasing:  "",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(fakeRunner(t, with(onABranch(), map[string]reply{
				readStopped: stoppedIn(t, tt.files),
			})), workDir)

			// Act
			described, err := repo.Describe(t.Context())

			// Assert
			if err != nil || described.Operation != tt.operation || described.Rebasing != tt.rebasing {
				t.Errorf("Describe = operation %v rebasing %q, %v; want %v rebasing %q",
					described.Operation, described.Rebasing, err, tt.operation, tt.rebasing)
			}
		})
	}
}
