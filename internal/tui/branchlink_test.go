// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errUnlinkRefused is a repository refusing to forget a link.
var errUnlinkRefused = errors.New("cannot write the link")

// offConvention is a branch begun outside workflow, named for no issue.
const offConvention = "my-thing"

// onOffConventionBranch is the world on offConvention, with or without a pull
// request opened from it.
func onOffConventionBranch(withPull bool) *world {
	repo := newWorld()
	repo.pullFound = withPull
	repo.branch = gitrepo.Branch{
		Name: offConvention, Head: "abc123", Base: baseRef,
		Upstream: "origin/" + offConvention, PushRemote: gitrepo.DefaultRemote,
	}

	return repo
}

func TestILinksABranchWithNoPullRequestToTheSelectedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(false)

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter).View().Content

	// Assert
	if linked := repo.asked("link-issue"); len(linked) != 1 || linked[0] != "link-issue my-thing "+issueKey {
		t.Errorf("linked %q, want my-thing linked to %s", linked, issueKey)
	}

	requireScreen(t, view, "linked my-thing to "+issueKey)
}

func TestLinkingTakesAKeyTypedInPlaceOfTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(false)

	// Act
	typing(t, repo.live(t, 120, 40), "2", "i", "ctrl+u", "#", "5", "7", keyEnter)

	// Assert
	if linked := repo.asked("link-issue"); len(linked) != 1 || linked[0] != "link-issue my-thing 57" {
		t.Errorf("linked %q, want my-thing linked to the forge's 57", linked)
	}
}

func TestLinkingABranchWithAPullRequestShowsItsDescriptionFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Jira: ["+issueKey+"]")

	if linked := repo.asked("link-issue"); len(linked) != 0 {
		t.Errorf("linked %q before the description was confirmed", linked)
	}
}

func TestConfirmingTheDescriptionLinksAndUpdatesThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter).View().Content

	// Assert
	edits := repo.asked("rewrite 42")
	if len(edits) != 1 || !strings.Contains(edits[0], "Jira: ["+issueKey+"]") || len(repo.asked("link-issue")) != 1 {
		t.Errorf("edited %q and linked %q; want the issue line added and the branch linked", edits, repo.asked("link-issue"))
	}

	requireScreen(t, view, "Link on "+issueKey)
}

func TestAPullRequestThatCannotBeEditedLeavesTheBranchUnlinked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.editPullErr = forge.ErrUnreachable

	// Act
	typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter)

	// Assert
	if edits, linked := repo.asked("rewrite 42"), repo.asked("link-issue"); len(edits) != 1 || len(linked) != 0 {
		t.Errorf("edited %q and linked %q; want the edit tried and the branch left unlinked", edits, linked)
	}
}

func TestLinkingAddsTheIssueLineToTheDescriptionTheForgeHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// What is shown of a description has its controls and bidirectional marks
	// replaced and its carriage returns dropped; the forge holds it as written.
	repo := onOffConventionBranch(true)
	repo.heldDescription = "Fixes the parser.\r\n\u202Eright to left\u202C\r\n"

	// Act
	typing(t, repo.live(t, 120, 40), "2", "i", keyEnter, keyEnter)

	// Assert
	rewrites := repo.asked("rewrite 42")
	if len(rewrites) != 1 || !strings.HasPrefix(rewrites[0], "rewrite 42\n"+repo.heldDescription) ||
		!strings.Contains(rewrites[0], "Jira: ["+issueKey+"]") {
		t.Errorf("rewrote %q, want the issue line added to the description as written", rewrites)
	}

	if edits := repo.asked("edit 42"); len(edits) != 0 {
		t.Errorf("edited %q, want the title and the shown description left alone", edits)
	}
}

// A forge that cannot edit a pull request leaves its description as it is, so
// the description is not shown as about to change: the branch is linked at
// once.
func TestLinkingWhereThePullRequestCannotBeEditedLinksWithoutShowingADescription(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	uneditable := repo.deps()
	uneditable.Forge.RewriteDescription = nil
	model := sized(t, tui.New(repo.cfg, nil, uneditable), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "2", "i", keyEnter).View().Content

	// Assert
	refuseScreen(t, view, "description becomes")

	if linked := repo.asked("link-issue"); len(linked) != 1 {
		t.Errorf("linked %q, want the branch linked at once", linked)
	}
}

// A merged pull request is done with: linking the branch offers nothing on it,
// as with no pull request at all.
func TestLinkingABranchWhosePullRequestMergedOffersNoPullRequestLink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.pull.State = forge.StateMerged

	// Act
	view := typing(t, repo.live(t, 120, 40), "2", "i", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "linked my-thing to "+issueKey)
	refuseScreen(t, view, "Link on "+issueKey)
}

func TestTheReviewPaneNamesTheIssueTheBranchWasLinkedTo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.branch.IssueLink = issueKey

	// Act
	view := typing(t, repo.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "issue  "+issueKey+" "+issueSummary)
}

func TestTheReviewPaneFindsTheIssueThePullRequestNames(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := onOffConventionBranch(true)
	repo.pull.Body = "Speeds it up.\n\nJira: " + issueKey

	// Act
	view := typing(t, repo.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "issue  "+issueKey)
}

func TestTheReviewPaneSaysWhyEachCheckFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.ci = []forge.CI{{State: forge.CIFailed, Total: 2, Done: 2, Failed: 1, Checks: []forge.Check{
		{Name: "build-docs", State: forge.CIPassed},
		{ID: "501", Name: "unit-race", Stage: "test", Reason: "script failure", State: forge.CIFailed},
	}}}

	// Act
	view := typing(t, failing.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "test · unit-race", "script failure")
	refuseScreen(t, view, "· build-docs")
}

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
