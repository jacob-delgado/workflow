// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// Branches the switcher scopes to your issues: one for an issue assigned to
// someone else, and one for an issue of yours that only the remote has.
const (
	someoneElsesBranch = "feat/PROJ-999-audit-log"
	remoteTaskBranch   = "feat/PROJ-388-x"
)

// notAskedLead opens the line the switcher shows when the tracker could not be
// asked which issues are yours.
const notAskedLead = "could not ask which issues are yours: "

// trackerAsked is what the switcher was asking the tracker, as a failure the
// interface has no sentence for says it.
const trackerAsked = "asking which issues are assigned to you: "

// errTrackerDown stands in for a tracker that cannot be asked anything.
var errTrackerDown = errors.New("the tracker is down")

// screenLineWith is the first line of a screen, its color escapes stripped, that
// shows phrase, or nothing when none does.
func screenLineWith(view, phrase string) string {
	for line := range strings.Lines(plain(view)) {
		if strings.Contains(line, phrase) {
			return line
		}
	}

	return ""
}

func TestTheSwitcherListsOnlyBranchesForYourIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	followUp := "feat/PROJ-412-follow-up"
	repo := cleanSwitcher()
	repo.branches = []string{featureName, followUp, otherTaskBranch, someoneElsesBranch}
	repo.assignedKeys = []jira.Key{issueKey, secondIssue}
	model := repo.live(t, 120, 40)

	// Act
	view := openSwitcher(t, model).View().Content

	// Assert
	requireScreen(t, view, followUp, otherTaskBranch)
	refuseScreen(t, view, "PROJ-999")
}

func TestTheSwitcherOffersARemoteBranchForYourIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branches = []string{featureName}
	repo.remoteBranches = []string{baseName, featureName, remoteTaskBranch}
	model := repo.live(t, 120, 40)

	// Act: open the switcher on the Branch pane
	opened := typing(t, model, "2", "s")

	// Assert: it offers the branch only the remote has, marked as the remote's
	requireScreen(t, opened.View().Content, remoteTaskBranch+" (remote)")

	// Act: check it out
	typing(t, opened, keyEnter)

	// Assert: git is asked to switch to it by name, which makes the local branch
	if got := repo.asked("checkout"); len(got) != 1 || got[0] != "checkout "+remoteTaskBranch {
		t.Errorf("checkout calls = %v, want one for the remote branch", got)
	}
}

func TestTheSwitcherAsksTheTrackerOnceWithEveryKey(t *testing.T) {
	t.Parallel()

	// Arrange
	// PROJ-388 is named three times: one branch here and on the remote, and one
	// only on the remote.
	repo := cleanSwitcher()
	repo.branches = []string{featureName, otherTaskBranch, someoneElsesBranch}
	repo.remoteBranches = []string{baseName, otherTaskBranch, remoteTaskBranch}
	model := repo.live(t, 120, 40)

	// Act
	openSwitcher(t, model)

	// Assert
	asked := repo.asked("lenient ")
	if len(asked) != 1 {
		t.Fatalf("lenient searches = %q, want one", asked)
	}

	for _, key := range []string{issueKey, secondIssue, "PROJ-999"} {
		if named := strings.Count(asked[0], key); named != 1 {
			t.Errorf("the search %q names %s %d times, want once", asked[0], key, named)
		}
	}
}

func TestTheSwitcherListsEveryIssueBranchWhenTheTrackerCannotBeAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branches = []string{featureName, someoneElsesBranch}
	repo.remoteBranches = []string{baseName, remoteTaskBranch}
	deps := repo.deps()
	deps.Jira.SearchLenient = func(string, int) (jira.SearchResult, error) {
		return jira.SearchResult{}, errTrackerDown
	}
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := openSwitcher(t, drain(t, model, model.Init())).View().Content

	// Assert
	// Nothing could be filtered, so someone else's issue is listed too.
	requireScreen(t, view, someoneElsesBranch, remoteTaskBranch+" (remote)")

	// An error the interface has no sentence for says what was being asked.
	if note := screenLineWith(view, trackerAsked); !strings.Contains(note, errTrackerDown.Error()) {
		t.Errorf("no line says the tracker could not be asked and why; the line naming %q is %q", trackerAsked, note)
	}
}

func TestTheSwitcherTellsAnUnreachableTrackerInItsSentence(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	deps := repo.deps()
	deps.Jira.SearchLenient = func(string, int) (jira.SearchResult, error) {
		return jira.SearchResult{}, fmt.Errorf("%w at https://jira.corp.example: dial tcp: i/o timeout", jira.ErrUnreachable)
	}
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := openSwitcher(t, drain(t, model, model.Init())).View().Content

	// Assert
	requireScreen(t, view, "✗ "+notAskedLead+"Jira did not answer in time.")
	refuseScreen(t, view, "jira.corp.example")
}

func TestAFailedRemoteListingShowsOnlyItsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.remoteBranchesErr = errListRemotes

	// Act
	view := openSwitcher(t, repo.live(t, 120, 40)).View().Content

	// Assert
	requireScreen(t, view, errListRemotes.Error())
	refuseScreen(t, view, otherTaskBranch)
}

func TestWithNoTrackerTheSwitcherListsEveryIssueBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branches = []string{featureName, someoneElsesBranch}
	repo.remoteBranches = []string{baseName, remoteTaskBranch}
	deps := repo.deps()
	deps.Jira.SearchLenient = nil
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := openSwitcher(t, drain(t, model, model.Init())).View().Content

	// Assert
	requireScreen(t, view, someoneElsesBranch, remoteTaskBranch+" (remote)")
	refuseScreen(t, view, notAskedLead)
}

func TestABranchHereAndOnTheRemoteIsListedOnceAsTheLocalOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.remoteBranches = []string{baseName, otherTaskBranch}

	// Act
	view := openSwitcher(t, repo.live(t, 120, 40)).View().Content

	// Assert
	if listed := strings.Count(plain(view), otherTaskBranch); listed != 1 {
		t.Errorf("%s is listed %d times, want once:\n%s", otherTaskBranch, listed, plain(view))
	}

	refuseScreen(t, view, otherTaskBranch+" (remote)")
}

func TestTheSwitcherListsNoBranchWhenNoneOfTheIssuesAreYours(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.remoteBranches = []string{baseName, remoteTaskBranch}
	repo.assignedKeys = []jira.Key{}

	// Act
	view := openSwitcher(t, repo.live(t, 120, 40)).View().Content

	// Assert
	requireScreen(t, view, "No other task branch to switch to.")
}

func TestWithNoRemoteTheSwitcherListsTheLocalBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.noRemoteBranches = true

	// Act
	view := openSwitcher(t, repo.live(t, 120, 40)).View().Content

	// Assert
	requireScreen(t, view, otherTaskBranch)
}
