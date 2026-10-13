// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The two commands a finish runs through the Runner, and the pull it hands to
// its caller, named so the tests read as the sequence.
const (
	finishSwitch = "git -C /work -c checkout.defaultRemote=upstream switch -- main"
	finishPull   = "the caller's pull"
	finishDelete = "git -C /work branch -D -- feat/token"
)

// tokenFinishing is feat/token finished onto main, pulled from upstream.
func tokenFinishing() gitrepo.Finishing {
	return gitrepo.Finishing{Branch: "feat/token", Base: localMain, Remote: upstreamName}
}

// errStepFailed is a finish step git refused.
var errStepFailed = errors.New("exit status 1")

// recordedPull is a finish's pull seam that records itself among the commands
// ran, then answers err.
func recordedPull(ran *[]string, err error) func() error {
	return func() error {
		*ran = append(*ran, finishPull)

		return err
	}
}

func TestFinishBranchSwitchesPullsAndDeletes(t *testing.T) {
	t.Parallel()

	// Arrange
	// No reply answers a pull: the fixture fails on any, so the pull must come
	// through the seam rather than the Runner.
	run, ran := recordingRunner(t, map[string]reply{
		finishSwitch: {},
		finishDelete: {},
	})
	repo := gitrepo.At(run, workDir)

	// Act
	err := repo.FinishBranch(t.Context(), tokenFinishing(), recordedPull(ran, nil))
	// Assert
	if err != nil {
		t.Fatalf("FinishBranch returned %v", err)
	}

	if want := []string{finishSwitch, finishPull, finishDelete}; !slices.Equal(*ran, want) {
		t.Errorf("ran %q, want the three steps in order", *ran)
	}
}

func TestFinishBranchStopsAtTheStepThatFails(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		replies map[string]reply
		pullErr error
		named   string
		want    []string
	}{
		"the switch": {
			replies: map[string]reply{finishSwitch: {err: errStepFailed}},
			pullErr: nil,
			named:   "git -c checkout.defaultRemote=upstream switch -- main",
			want:    []string{finishSwitch},
		},
		// The pull cannot fast-forward, so the branch must not be deleted.
		"the pull": {
			replies: map[string]reply{finishSwitch: {}},
			pullErr: errStepFailed,
			named:   "git pull --ff-only -- upstream main",
			want:    []string{finishSwitch, finishPull},
		},
		"the delete": {
			replies: map[string]reply{finishSwitch: {}, finishDelete: {err: errStepFailed}},
			pullErr: nil,
			named:   "git branch -D -- feat/token",
			want:    []string{finishSwitch, finishPull, finishDelete},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, tt.replies)
			repo := gitrepo.At(run, workDir)

			// Act
			err := repo.FinishBranch(t.Context(), tokenFinishing(), recordedPull(ran, tt.pullErr))

			// Assert
			if !errors.Is(err, errStepFailed) || !strings.Contains(err.Error(), tt.named) {
				t.Errorf("FinishBranch returned %v, want the failure naming %q", err, tt.named)
			}

			if !slices.Equal(*ran, tt.want) {
				t.Errorf("ran %q, want %q and nothing after", *ran, tt.want)
			}
		})
	}
}

func TestFinishingCommandsAreTheCommandsAFinishRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	// The preview a force delete is confirmed against shows these, so they
	// must be the commands the finish runs, word for word.
	run, ran := recordingRunner(t, map[string]reply{finishSwitch: {}, finishDelete: {}})

	err := gitrepo.At(run, workDir).FinishBranch(t.Context(), tokenFinishing(), recordedPull(ran, nil))
	if err != nil {
		t.Fatalf("FinishBranch returned %v", err)
	}

	// Act
	commands := tokenFinishing().Commands()

	// Assert
	want := []string{
		strings.Replace(finishSwitch, "-C /work ", "", 1),
		"git " + strings.Join(gitrepo.PullCommand(workDir, upstreamName, "main").Args, " "),
		strings.Replace(finishDelete, "-C /work ", "", 1),
	}

	if !slices.Equal(commands, want) || want[1] != "git pull --ff-only -- upstream main" {
		t.Errorf("Commands = %q, want %q, as FinishBranch ran them", commands, want)
	}
}
