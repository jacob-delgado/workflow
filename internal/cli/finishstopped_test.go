// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/loop"
)

func TestFinishDuringAStoppedOperationIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		stop func(t *testing.T, repo string)
		want string
	}{
		"a merge": {
			stop: clitest.MergeStopped,
			want: "cannot finish " + finishedBranch +
				": a merge is stopped part way; run git merge --continue or git merge --abort first",
		},
		"a rebase, naming the branch it replays": {
			stop: func(t *testing.T, repo string) {
				t.Helper()
				clitest.RebaseStopped(t, repo, finishBase)
			},
			want: "cannot finish " + finishedBranch +
				": a rebase is stopped part way; run git rebase --continue or git rebase --abort first",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := mergedFinishRepo(t)
			tt.stop(t, repo)

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")

			// Assert
			if !errors.Is(err, loop.ErrOperationStopped) || err.Error() != tt.want {
				t.Errorf("finish = %v, want %q", err, tt.want)
			}

			wantExit(t, err, 4)
			requireBranchKept(t, repo)
		})
	}
}
