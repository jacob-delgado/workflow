// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// linkedBranch is the world on offConvention, linked by hand to issueKey, with
// its pull request open.
func linkedBranch() *world {
	repo := onOffConventionBranch(true)
	repo.branch.IssueLink = issueKey

	return repo
}

func TestILinkedBranchNamesItsIssueAndOffersUnlink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i").View().Content

	// Assert
	requireScreen(t, view, offConvention+" is linked to "+issueKey, "u unlink")
}

func TestUnlinkForgetsTheLinkAtOnceAndLeavesTheDescription(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", "u").View().Content

	// Assert
	if unlinked := repo.asked("unlink-issue"); len(unlinked) != 1 || unlinked[0] != "unlink-issue "+offConvention {
		t.Errorf("unlinked %q, want %s unlinked once", unlinked, offConvention)
	}

	if edits := repo.asked("edit 42"); len(edits) != 0 {
		t.Errorf("edited %q; unlinking leaves the description as it is", edits)
	}

	requireScreen(t, view, "unlinked "+offConvention+" from "+issueKey)
}

func TestARefusedUnlinkStaysInTheFormWithItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()
	repo.unlinkErr = errUnlinkRefused

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", "u").View().Content

	// Assert
	requireScreen(t, view, "cannot write the link", offConvention+" is linked to "+issueKey)
}

func TestADryRunUnlinksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "2", "i", "u").View().Content

	// Assert
	if unlinked := repo.asked("unlink-issue"); len(unlinked) != 0 {
		t.Errorf("unlinked %q under a dry run", unlinked)
	}

	requireScreen(t, view, "dry run: would unlink "+offConvention+" from "+issueKey)
}

func TestOnALinkedBranchOnlyUUnlinksAndEscCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", "x", keyEnter, keyEsc).View().Content

	// Assert
	if unlinked := repo.asked("unlink-issue"); len(unlinked) != 0 {
		t.Errorf("unlinked %q, want only u to unlink", unlinked)
	}

	refuseScreen(t, view, "is linked to")
}

func TestAPasteOnALinkedBranchTypesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()
	model := typing(t, repo.live(t, 120, 40), "2", "i")

	// Act
	view := pasting(t, model, "#57").View().Content

	// Assert
	refuseScreen(t, view, "#57")
	requireScreen(t, view, offConvention+" is linked to "+issueKey)
}

// heldUnlink forgets the branch's link, and then holds git's answer.
func heldUnlink(deps *tui.Deps, held *hold) {
	unlink := deps.Git.UnlinkIssue
	deps.Git.UnlinkIssue = func(ctx context.Context, branch string) error {
		err := unlink(ctx, branch)

		held.wait()

		return err
	}
}

func TestAnUnlinkBeingSentTakesNoSecondUnlink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := linkedBranch()
	held := newHold(t)
	started := heldLive(t, held, repo, heldUnlink)
	held.armed.Store(true)
	sending := holding(t, held, started, "2", "i", "u")

	// Act
	view := holding(t, held, sending, "u", keyEsc).View().Content

	// Assert
	if unlinked := repo.asked("unlink-issue"); len(unlinked) != 1 {
		t.Errorf("unlinked %q, want the one unlink already out", unlinked)
	}

	requireScreen(t, view, "unlinking…", offConvention+" is linked to "+issueKey)
	refuseScreen(t, footerLine(view), "unlink")
}
