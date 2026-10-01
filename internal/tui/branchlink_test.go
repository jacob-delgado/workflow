// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

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
	edits := repo.asked("edit 42")
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
	if edits, linked := repo.asked("edit 42"), repo.asked("link-issue"); len(edits) != 1 || len(linked) != 0 {
		t.Errorf("edited %q and linked %q; want the edit tried and the branch left unlinked", edits, linked)
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
