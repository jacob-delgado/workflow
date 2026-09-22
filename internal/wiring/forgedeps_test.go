// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The forge seams' success arms — the branch each takes once the connection is
// made — are reached black-box by pointing the forge at its CLI and answering
// through a stand-in gh, rather than injecting the unexported connect.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheForgeSeamsReturnTheForgesAnswerThroughTheCLI(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{
		create: `{"number":43,"html_url":"https://github.com/owner/repo/pull/43","title":"fix: token","state":"open"}`,
	})

	cfg, where := githubCLIWorkspace(t)

	seams := wiring.Deps(t.Context(), cfg, where, nil).Forge

	// Act
	_, found, findErr := seams.FindPullRequest("feat/x")
	created, createErr := seams.CreatePullRequest(forge.NewPullRequest{Title: "fix: token"})
	_, statusErr := seams.CheckStatus(forge.PullRequest{Number: 43}, "abc123")
	reviews, reviewErr := seams.ReviewRequests()
	author, authorErr := seams.Author()

	// Assert
	// Every seam gets past its connect guard and returns the forge's answer,
	// which the connection-failure tests never reach.
	if findErr != nil || found {
		t.Errorf("FindPullRequest = %v, %v; want no pull found and no error", found, findErr)
	}

	if createErr != nil || created.Number != 43 {
		t.Errorf("CreatePullRequest = %+v, %v; want the created pull #43", created, createErr)
	}

	if statusErr != nil {
		t.Errorf("CheckStatus returned %v, want the forge's answer", statusErr)
	}

	if reviewErr != nil || len(reviews) != 0 {
		t.Errorf("ReviewRequests = %+v, %v; want an empty list and no error", reviews, reviewErr)
	}

	if authorErr != nil || author != "octo" {
		t.Errorf("Author = %q, %v; want the login the forge reported", author, authorErr)
	}
}
