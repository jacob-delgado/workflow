// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// CI polling must not multiply: a refresh of the same pull request keeps the
// poll already running rather than starting a second, and a poll left over from
// a replaced review stops itself. These reach reviewState and the poll messages
// directly, since the harness cannot press a key while a poll is outstanding.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// errForgeDown is a CI check that failed, for the back-off path.
var errForgeDown = errors.New("the forge is down")

func TestASamePullRefreshKeepsItsPollingAndCI(t *testing.T) {
	t.Parallel()

	// Arrange
	model := New(config.Config{}, nil, Deps{})
	model.branch.branch.Name = "feat/x"
	model.review = reviewState{
		pull: forge.PullRequest{Number: 42}, found: true, loaded: true,
		ci: forge.CI{State: forge.CIRunning}, checked: true, polling: true, generation: 3,
	}

	// Act
	refreshed, _ := pullFound{branch: "feat/x", pull: forge.PullRequest{Number: 42}, found: true}.apply(model)

	// Assert
	if r := refreshed.review; !r.polling || !r.checked || r.ci.State != forge.CIRunning || r.generation != 3 {
		t.Errorf("a refresh of the same pull request reset it: %+v", r)
	}
}

func TestAStalePollFromAnEarlierReviewStops(t *testing.T) {
	t.Parallel()

	// Arrange
	model := New(config.Config{}, nil, Deps{})
	model.review = reviewState{pull: forge.PullRequest{Number: 42}, found: true, polling: true, generation: 5}

	// Act
	after, cmd := ciPoll{number: 42, generation: 4}.apply(model)

	// Assert
	if cmd != nil || !after.review.polling {
		t.Error("a poll from a replaced review asked again or cleared the current poll")
	}
}

func TestPollingStopsAfterAFailedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	model := New(config.Config{}, nil, Deps{})
	model.review = reviewState{
		pull: forge.PullRequest{Number: 42}, found: true,
		ci: forge.CI{State: forge.CIRunning}, ciErr: errForgeDown,
	}

	// Act
	_, cmd := model.keepPolling(nil)

	// Assert
	if cmd != nil {
		t.Error("kept polling at the full rate after a failed check")
	}
}
