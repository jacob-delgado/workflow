// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A forge lists a new pull request's checks a moment after it opens, so the
// first read of CI after opening one can report none.

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// openingIntoNoChecks is a branch with no pull request whose CI reports no
// check when first read, under ui.notify, polled fast enough to see what
// follows within the test.
func openingIntoNoChecks(later ...forge.CI) *world {
	repo := withoutPull()
	repo.cfg.UI.Notify = true
	repo.ciInterval = pollTick

	repo.ci = append([]forge.CI{{State: forge.CINone}}, later...)

	return repo
}

func TestCIReportingNoCheckJustAfterOpeningIsReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingIntoNoChecks(
		forge.CI{State: forge.CIRunning, Total: 1, Done: 0},
		forge.CI{State: forge.CIPassed, Total: 1, Done: 1},
	)

	// Act
	typing(t, repo.live(t, 120, 40), "4", "n", keyEnter, keyEnter)

	// Assert
	if rung := repo.asked("notify"); len(rung) != 1 {
		t.Errorf("notify calls = %v, want one ring once the checks the forge listed later passed", rung)
	}
}

func TestCIFinishedByTheReadAfterOneReportingNoCheckRings(t *testing.T) {
	t.Parallel()

	// Arrange
	// The checks the forge listed after the pull request opened ran and
	// finished between two reads.
	repo := openingIntoNoChecks(forge.CI{State: forge.CIPassed, Total: 1, Done: 1})

	// Act
	typing(t, repo.live(t, 120, 40), "4", "n", keyEnter, keyEnter)

	// Assert
	if rung := repo.asked("notify"); len(rung) != 1 {
		t.Errorf("notify calls = %v, want one ring once the checks the forge listed later passed", rung)
	}
}

func TestCIReportingNoCheckLongAfterOpeningIsNotReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each read of CI takes as long as an armed announcement waits for a
	// check, and reports none.
	repo := openingIntoNoChecks()
	deps := repo.deps()
	check := deps.Forge.CheckStatus
	deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		repo.mu.Lock()
		repo.elapsed += armed.NoCIPatience
		repo.mu.Unlock()

		return check(ctx, pull, head)
	}

	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	typing(t, drain(t, model, model.Init()), "4", "n", keyEnter, keyEnter)

	// Assert
	if reads := repo.asked("ci "); len(reads) != 1 {
		t.Errorf("CI reads = %q, want the one after opening, and none once it waited as long as an announcement", reads)
	}
}
