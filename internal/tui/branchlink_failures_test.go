// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errLinkRefused is a repository refusing to keep a link.
var errLinkRefused = errors.New("cannot write the link")

func TestARefusedLinkLeavesTheDescriptionAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.branchLinkErr = errLinkRefused

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter).View().Content

	// Assert
	if rewrites := repo.asked("rewrite 42"); len(rewrites) != 0 {
		t.Errorf("rewrote %q although the branch could not be linked", rewrites)
	}

	requireScreen(t, view, "not linked", "description was left as it was", "write the link")
}

func TestADescriptionTheForgeRefusesLeavesTheBranchLinkedAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.editPullErr = forge.ErrUnreachable

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter).View().Content

	// Assert
	if linked, rewrites := repo.asked("link-issue"), repo.asked("rewrite 42"); len(linked) != 1 || len(rewrites) != 1 {
		t.Errorf("linked %q and rewrote %q; want the branch linked, then the edit tried", linked, rewrites)
	}

	requireScreen(t, view, "is linked", "description was not updated")
}

func TestTheBranchIsLinkedBeforeTheDescriptionIsRewritten(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)

	// Act
	typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter)

	// Assert
	var order []string

	for _, call := range repo.asked("") {
		if len(call) >= 10 && (call[:10] == "link-issue" || call[:10] == "rewrite 42") {
			order = append(order, call[:10])
		}
	}

	if len(order) != 2 || order[0] != "link-issue" || order[1] != "rewrite 42" {
		t.Errorf("wrote in the order %q, want the link first and the description after it", order)
	}
}

func TestTypingAnotherIssueClearsTheRefusedLink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(false)
	repo.branchLinkErr = errLinkRefused
	refused := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter)

	// Act
	retyped := typing(t, refused, keyClearLine, "#", "5")

	// Assert
	refuseScreen(t, retyped.View().Content, "write the link")
}

func TestPastingAnotherIssueClearsTheRefusedLink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(false)
	repo.branchLinkErr = errLinkRefused
	refused := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter)

	// Act
	repasted := typing(t, refused, keyClearLine, pasted("#57"))

	// Assert
	refuseScreen(t, repasted.View().Content, "write the link")
}
