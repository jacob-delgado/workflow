// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"encoding/json"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// stoppedBranch is the branch the stopped operations here stop on.
const stoppedBranch = "feature"

// rebaseStopped is a repository on stoppedBranch, one commit ahead of main,
// with a rebase of it stopped part way at that commit: HEAD is detached, and
// only the rebase names the branch.
func rebaseStopped(t *testing.T) string {
	t.Helper()

	dir := clitest.PRRepo(t, stoppedBranch)
	clitest.RebaseStopped(t, dir, "main")

	return dir
}

// mergeStopped is a repository on stoppedBranch, one commit ahead of main,
// with a merge into it stopped part way, so MERGE_HEAD waits.
func mergeStopped(t *testing.T) string {
	t.Helper()

	dir := clitest.PRRepo(t, stoppedBranch)
	clitest.MergeStopped(t, dir)

	return dir
}

func TestDoctorNamesAGitOperationStoppedPartWay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		stopped func(t *testing.T) string
		want    string
	}{
		"a rebase, by the branch it replays": {
			stopped: rebaseStopped,
			want:    "(rebasing " + stoppedBranch + " — run git rebase --continue or --abort first)",
		},
		"a merge, beside the branch": {
			stopped: mergeStopped,
			want:    stoppedBranch + " (merge stopped — run git merge --continue or --abort first)",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := tt.stopped(t)

			// Act
			output, _ := clitest.Run(t, dir, "doctor")

			// Assert
			if got := fieldValue(output, "Branch"); got != tt.want {
				t.Errorf("Branch = %q, want %q:\n%s", got, tt.want, output)
			}
		})
	}
}

func TestDoctorJSONNamesAGitOperationStoppedPartWay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		stopped   func(t *testing.T) string
		operation string
		rebasing  any
	}{
		"a rebase": {stopped: rebaseStopped, operation: "rebase", rebasing: stoppedBranch},
		"a merge":  {stopped: mergeStopped, operation: "merge", rebasing: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := tt.stopped(t)

			// Act
			printed, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--json")

			// Assert
			var report doctorRepository

			err := json.Unmarshal([]byte(printed.Stdout), &report)
			if err != nil || report.Repository["operation"] != tt.operation ||
				report.Repository["rebasing"] != tt.rebasing {
				t.Errorf("doctor --json repository = %v, %v; want operation %q, rebasing %v",
					report.Repository, err, tt.operation, tt.rebasing)
			}
		})
	}
}
