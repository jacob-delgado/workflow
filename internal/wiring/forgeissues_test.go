// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// When there is no Jira, the forge's own issues back the Issues pane. These
// drive that tracker through wiring.Deps — its Search, Issue, Transitions and
// Transition seams — against a stand-in gh answering the issue endpoints.

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// theBugList is the assigned-issue search the fake forge answers with, and
// theBugDetail is that same issue read in full.
const (
	theBugList   = `{"items":[{"number":42,"html_url":"https://github.com/owner/repo/issues/42","title":"the bug"}]}`
	theBugDetail = `{"number":42,"html_url":"https://github.com/owner/repo/issues/42",` +
		`"title":"the bug","body":"it broke","user":{"login":"ana"}}`
)

// forgeTracker is the Issues-pane tracker backed by the forge — no Jira
// configured — pointed at github.com through its CLI.
func forgeTracker(t *testing.T) tui.JiraDeps {
	t.Helper()

	cfg := config.Config{Forge: config.Forge{CLI: true, Kind: githubKind, Host: hostGitHub}}
	where := wiring.Workspace{Root: t.TempDir(), Remote: remoteGitHub}

	return wiring.Deps(t.Context(), cfg, where, nil).Jira
}

func TestTheForgeTrackerListsAssignedIssuesAsSearchRows(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})
	tracker := forgeTracker(t)

	// Act
	result, err := tracker.Search("", 0)

	// Assert
	if err != nil || result.Total != 1 || len(result.Issues) != 1 {
		t.Fatalf("Search = %+v, %v; want one row", result, err)
	}

	if row := result.Issues[0]; row.Key != "42" || row.Summary != "the bug" || row.StatusCategory != jira.CategoryNew {
		t.Errorf("issue row = %+v, want key 42, its title and the open category", row)
	}
}

func TestTheForgeTrackerReadsAnIssueAsDetail(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{issue: theBugDetail})
	tracker := forgeTracker(t)

	// Act
	detail, err := tracker.Issue("42")

	// Assert
	if err != nil || detail.Issue.Key != "42" || detail.Description != "it broke" || detail.Reporter != "ana" {
		t.Errorf("Issue = %+v, %v; want the body and author", detail, err)
	}
}

func TestTheForgeTrackerClosesAnIssue(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{issue: theBugDetail})
	tracker := forgeTracker(t)

	// Act
	err := tracker.Transition("42", jira.Transition{ID: transitionClose}, nil)
	// Assert
	if err != nil {
		t.Fatalf("Transition to close: %v", err)
	}

	args := ghStub.args()
	if !containsAll(args, "-X", "PATCH") || len(args) == 0 || !strings.Contains(args[len(args)-1], "/issues/42") {
		t.Errorf("gh was called as %v, want a PATCH closing issue 42", args)
	}
}

func TestTheForgeTrackerRejectsANonNumericKey(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})
	tracker := forgeTracker(t)

	// Act
	err := tracker.Transition("PROJ-1", jira.Transition{ID: transitionClose}, nil)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "not a forge issue number") {
		t.Errorf("Transition = %v, want a key that is not a forge issue number reported", err)
	}

	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("Transition = %v, want it marked not-found so the web API answers 404", err)
	}
}

func TestTheForgeTrackerReportsAForgeThatCannotBeReached(t *testing.T) {
	// No gh to route through and no token, so the connection cannot be made.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())

	tracker := forgeTracker(t)

	cases := map[string]func() error{
		seamSearch: func() error {
			_, err := tracker.Search("", 0)

			return err
		},
		seamIssue: func() error {
			_, err := tracker.Issue("42")

			return err
		},
		"Transition": func() error {
			return tracker.Transition("42", jira.Transition{ID: transitionClose}, nil)
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			err := act()

			// Assert
			if !errors.Is(err, forge.ErrNoToken) {
				t.Errorf("%s = %v, want the connect failure", name, err)
			}
		})
	}
}

func TestTheForgeTrackerReportsAForgeError(t *testing.T) {
	// The fake answers with an unreadable body, so the forge client's decode
	// fails and the seam must report it rather than swallow it.
	installForgeCLI(t, "gh", forgeReplies{search: "{", issue: "{"})
	tracker := forgeTracker(t)

	cases := map[string]func() error{
		seamSearch: func() error {
			_, err := tracker.Search("", 0)

			return err
		},
		seamIssue: func() error {
			_, err := tracker.Issue("42")

			return err
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			err := act()

			// Assert
			if err == nil {
				t.Errorf("%s reported no error, want the forge's failure surfaced", name)
			}
		})
	}
}

func TestTheForgeTrackerOffersOnlyClose(t *testing.T) {
	// Arrange
	tracker := forgeTracker(t)

	// Act
	transitions, err := tracker.Transitions("42")

	// Assert
	if err != nil || len(transitions) != 1 ||
		transitions[0].Name != "Close" || transitions[0].ToStatusCategory != jira.CategoryDone {
		t.Errorf("Transitions = %+v, %v; want a single Close to the done category", transitions, err)
	}
}

func TestTheTrackerPicksJiraWhenConfiguredAndTheForgeOtherwise(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cfg         config.Config
		jiraBackend bool // the Jira-only Comment seam is present for the Jira backend alone
	}{
		"Jira when it is configured": {
			cfg: config.Config{Jira: config.Jira{BaseURL: "https://jira.example.com"}}, jiraBackend: true,
		},
		"the forge otherwise": {cfg: config.Config{}, jiraBackend: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			where := wiring.Workspace{Root: t.TempDir(), Remote: githubRemote}

			// Act
			tracker := wiring.Deps(t.Context(), tt.cfg, where, nil).Jira

			// Assert
			if tracker.Search == nil {
				t.Fatalf("the tracker for %q has no search", name)
			}

			if got := tracker.Comment != nil; got != tt.jiraBackend {
				t.Errorf("the tracker for %q has the Jira-only Comment seam = %v, want %v", name, got, tt.jiraBackend)
			}
		})
	}
}
