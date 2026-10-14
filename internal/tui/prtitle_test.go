// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// titleField is how the composer's title field starts on screen, once it
// asks what the pull request says.
const titleField = "> "

// issuelessBranch is a branch whose name names no issue.
const issuelessBranch = "fix-token-redaction"

// unlistedIssue is the world before a pull request is opened, titled from the
// issue, with the branch's issue left out of the list — assigned to someone
// else, say — so only reading it can give the title.
func unlistedIssue() *world {
	w := withoutPull()
	w.cfg.PullRequest.TitleSource = "issue"
	w.issues = w.issues[1:]
	w.detail.Issue.Summary = issueSummary

	return w
}

// openedBeforeTheIssueAnswers opens the composer on the world and returns it
// with the read of the issue for its title not yet answered, so a test can
// act first and deliver the answer after.
func openedBeforeTheIssueAnswers(t *testing.T, w *world) (tui.Model, tea.Cmd) {
	t.Helper()

	opened, read := typing(t, w.live(t, 120, 40), "4").Update(keyMsg("n"))

	return concrete(t, opened), read
}

func TestTheComposerTakesItsTitleFromTheConfiguredSource(t *testing.T) {
	t.Parallel()

	// Arrange
	// With the issue as the title source, the title is the issue rather than the
	// branch's oldest commit (pullTitle).
	opening := withoutPull()
	opening.cfg.PullRequest.TitleSource = "issue"

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+issueKey+": "+issueSummary)
}

func TestTheComposerReadsAnUnlistedIssueForItsTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := unlistedIssue()

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+issueKey+": "+issueSummary)
}

func TestAFailedIssueReadKeepsTheCommitTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failed read still carries a summary, which must not be taken.
	opening := unlistedIssue()
	opening.detailErr = errNotVisible

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+pullTitle)
}

func TestTheCommitSourceReadsNoIssueForTheTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := unlistedIssue()
	opening.cfg.PullRequest.TitleSource = string(convention.TitleFromCommit)

	// Act
	typing(t, opening.live(t, 120, 40), "4", "n")

	// Assert
	if reads := opening.asked("issue " + issueKey); len(reads) != 0 {
		t.Errorf("a commit-titled composer read the issue: %q", reads)
	}
}

func TestABranchNamingNoIssueReadsNoneForTheTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := unlistedIssue()
	opening.branch.Name = issuelessBranch

	// Act
	typing(t, opening.live(t, 120, 40), "4", "n")

	// Assert
	if slices.Contains(opening.asked("issue"), "issue ") {
		t.Error("the composer read an issue for a branch naming none")
	}
}

func TestTheComposerOpensWithoutAnIssueSeam(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := unlistedIssue()
	deps := opening.deps()
	deps.Jira.Issue = nil
	model := sized(t, tui.New(opening.cfg, nil, deps), 120, 40)
	started := drain(t, model, model.Init())

	// Act
	view := typing(t, started, "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+pullTitle)
}

func TestAKeyBeforeTheIssueAnswersTypesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheIssueAnswers(t, unlistedIssue())
	typed := typing(t, opened, "Z", keyEnter)

	// Act
	view := typing(t, drain(t, typed, read), keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+issueKey+": "+issueSummary)
	refuseScreen(t, view, "Z")
}

func TestEnterBeforeTheIssueAnswersOpensNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := unlistedIssue()
	opened, _ := openedBeforeTheIssueAnswers(t, opening)

	// Act
	typing(t, opened, keyEnter, keyEnter)

	// Assert
	if calls := opening.asked("open "); len(calls) != 0 {
		t.Errorf("opened %q before the issue answered, want nothing", calls)
	}
}

func TestAnIssueAnsweringAfterTheComposerClosedOpensNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, read := openedBeforeTheIssueAnswers(t, unlistedIssue())
	closed := typing(t, opened, keyEsc)

	// Act
	view := drain(t, closed, read).View().Content

	// Assert
	refuseScreen(t, view, titleField)
}

func TestAnIssueAnsweringForAnotherBranchLeavesItsTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	// Before the issue answers, the composer is closed and the branch is
	// switched in a shell to one naming no issue, whose composer proposes the
	// same commit subject.
	opening := unlistedIssue()
	opened, read := openedBeforeTheIssueAnswers(t, opening)
	closed := typing(t, opened, keyEsc)
	opening.branch.Name = issuelessBranch
	reopened := typing(t, closed, "r", "n")

	// Act
	view := typing(t, drain(t, reopened, read), keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+pullTitle)
}

func TestAKeptDraftTakesTheIssueTitleOnReopening(t *testing.T) {
	t.Parallel()

	// Arrange
	// The composer is closed before the issue answers, so the draft it keeps
	// still holds the proposed title, and that answer is never delivered.
	opened, _ := openedBeforeTheIssueAnswers(t, unlistedIssue())
	closed := typing(t, opened, keyEsc)

	// Act
	view := typing(t, closed, "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, titleField+issueKey+": "+issueSummary)
}
