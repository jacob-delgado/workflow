// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// failedCI is a run whose one check failed.
func failedCI() forge.CI {
	return forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}
}

func TestAnnouncingNowIsRefusedWhileACheckFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	red := newWorld()
	red.ci = []forge.CI{failedCI()}

	// Act
	view := typing(t, red.live(t, 140, 40), "5", "p", keyEnter).View().Content

	// Assert
	if posts := red.asked("post "); len(posts) != 0 {
		t.Errorf("posted %q, want nothing posted while a check failed", posts)
	}

	requireScreen(t, view, "A check failed on the pull request, so nothing was announced.")
}

func TestAPostTheGateRefusesKeepsThePreviewOpenWithWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs as the preview opens, and a check has failed by the time it posts.
	turning := newWorld()
	turning.ci = []forge.CI{{State: forge.CIRunning, Total: 1}, failedCI()}

	// Act
	view := typing(t, turning.live(t, 140, 40), "5", "p", keyEnter).View().Content

	// Assert
	if posts := turning.asked("post "); len(posts) != 0 {
		t.Errorf("posted %q, want the gate to refuse it", posts)
	}

	requireScreen(t, view, "Announce to Slack")
	requireScreen(t, view, "A check failed on the pull request, so nothing was announced.")
}

func TestAQuitRightAfterAnnouncingWaitsForTheGateAndThePost(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	deps := repo.deps()
	read := deps.Forge.CheckStatus
	held := newHold(t)
	asked := make(chan context.Context, 1)
	deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		if held.armed.Load() {
			select {
			case asked <- ctx:
			default:
			}
		}

		held.wait()

		return read(ctx, pull, head)
	}

	session, endSession := context.WithCancel(t.Context())
	program, endProgram := context.WithCancel(t.Context())
	model := tui.New(repo.cfg, nil, deps).WithContext(session).WithLifetime(program)
	model = sized(t, model, 140, 50)
	model = typing(t, drain(t, model, model.Init()), "5", "p")

	held.armed.Store(true)
	holding(t, held, model, keyEnter)
	held.requireReached(t)

	gate := <-asked

	// Act: the session ends, as on a quit
	endSession()

	// Assert: the gate's read goes on, as the post it guards would
	err := gate.Err()
	if err != nil {
		t.Errorf("the gate's CI read ended with %v once its session did, want it to go on", err)
	}

	// Act: the program ends
	endProgram()

	// Assert: the read ends with it
	select {
	case <-gate.Done():
	case <-time.After(quitPatience):
		t.Error("the gate's CI read went on once its program ended, want it ended with the program")
	}
}

func TestAMergeACheckFailedOnSaysToReRunTheChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merge cannot wait for CI, so it is not pointed at the wait.
	merged := newWorld()
	merged.pull.State = forge.StateMerged
	merged.ci = []forge.CI{failedCI()}

	// Act
	view := typing(t, merged.live(t, 140, 40), "5", "p", keyEnter).View().Content

	// Assert
	if posts := merged.asked("post "); len(posts) != 0 {
		t.Errorf("posted %q, want nothing posted while a check failed", posts)
	}

	requireScreen(t, view, "A check failed on the pull request, so its merge was not announced.",
		"then announce it once they pass.")
	refuseScreen(t, view, "once CI passes")
}

func TestAMergeIsAnnouncedOnceTheChecksThatFailedOnItPass(t *testing.T) {
	t.Parallel()

	// Arrange
	// A check failed while it was open; it merged, and the failed checks were
	// re-run on the forge and passed, which the pane does not follow.
	rerun := newWorld()
	rerun.ci = []forge.CI{failedCI()}
	model := rerun.live(t, 140, 40)
	rerun.pull.State = forge.StateMerged
	rerun.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1}}
	model = typing(t, model, "4", "r")

	// Act
	typing(t, model, "5", "p", keyEnter)

	// Assert
	if posts := rerun.asked("post "); len(posts) != 1 {
		t.Errorf("posted %q, want the merge announced once its checks passed", posts)
	}
}
