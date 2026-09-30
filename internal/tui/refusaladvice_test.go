// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// gitlabScopeRefusal is gitlab.com turning down a merge request made with a
// token that has read_api but not api.
func gitlabScopeRefusal() error {
	return &forge.RefusalError{
		Kind: forge.KindGitLab, Status: forge.ErrRefused,
		Reason: "insufficient_scope: The request requires higher privileges than provided by the access token. " +
			"(needs the api scope)",
	}
}

func TestARefusedMergeRequestIsToldInGitLabsWordsOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.openErr = gitlabScopeRefusal()
	advice, _ := forge.Advice(opening.openErr)

	// Act
	view := typing(t, opening.live(t, 400, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "✗ "+advice)

	if said := strings.Count(plain(view), "insufficient_scope"); said != 1 {
		t.Errorf("GitLab's reason is shown %d times, want once:\n%s", said, plain(view))
	}
}

func TestARefusalWrappedInItsStepStillNamesTheStep(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.openErr = fmt.Errorf("opening the merge request: %w", gitlabScopeRefusal())

	// Act
	view := typing(t, opening.live(t, 400, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "GitLab refused this", "opening the merge request")
}
