// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// Jira finds its token on first use, so a command that never reaches it never
// runs its token command, which may stop to ask for a passphrase or a
// fingerprint. A stand-in token command records each run for these tests to
// count.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// recordedToken is what the stand-in token command prints.
const recordedToken = "token-from-the-command"

// tokenCommand is a stand-in token_command: a script that prints recordedToken
// and marks each run in a file of its own.
type tokenCommand struct {
	command string
	record  string
}

// newTokenCommand writes the stand-in.
func newTokenCommand(t *testing.T) tokenCommand {
	t.Helper()

	return writeTokenCommand(t, "printf '"+recordedToken+"\\n'\n")
}

// newFailingTokenCommand writes a stand-in that fails each run.
func newFailingTokenCommand(t *testing.T) tokenCommand {
	t.Helper()

	return writeTokenCommand(t, "exit 1\n")
}

// writeTokenCommand writes a stand-in that marks its run, then does what then
// says. The command is split on spaces, so its path must hold none, and a
// temporary directory's does not.
func writeTokenCommand(t *testing.T, then string) tokenCommand {
	t.Helper()

	dir := t.TempDir()
	record := filepath.Join(dir, "runs")
	command := filepath.Join(dir, "token")
	write(t, command, "#!/bin/sh\nprintf 'x\\n' >> '"+record+"'\n"+then, 0o700)

	return tokenCommand{command: command, record: record}
}

// runs is how many times the stand-in has run.
func (c tokenCommand) runs() int {
	data, _ := os.ReadFile(c.record)

	return strings.Count(string(data), "x")
}

// standInJira answers every request with an empty search result, keeping the
// Authorization header of each and counting them.
type standInJira struct {
	url           string
	requests      *atomic.Int32
	authorization *atomic.Value
}

func newStandInJira(t *testing.T) standInJira {
	t.Helper()

	jiraStandIn := standInJira{url: "", requests: &atomic.Int32{}, authorization: &atomic.Value{}}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		jiraStandIn.requests.Add(1)
		jiraStandIn.authorization.Store(request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"total":0,"issues":[]}`))
	}))
	t.Cleanup(server.Close)

	jiraStandIn.url = server.URL

	return jiraStandIn
}

func TestTheJiraTokenCommandRunsOnlyOnceJiraIsFirstAsked(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})

	tokens := newTokenCommand(t)
	jiraStandIn := newStandInJira(t)

	cfg, where := githubCLIWorkspace(t)
	cfg.Jira = config.Jira{BaseURL: jiraStandIn.url, TokenCommand: tokens.command}

	// Act: wire the seams
	deps := wired(t, cfg, where, nil)

	// Assert: the token command has not run
	if runs := tokens.runs(); runs != 0 {
		t.Fatalf("wiring the seams ran the Jira token command %d times, want none", runs)
	}

	// Act: list the review requests, which reach the forge alone
	_, reviewErr := deps.Forge.ReviewRequests()

	// Assert: it has still not run
	if runs := tokens.runs(); reviewErr != nil || runs != 0 {
		t.Fatalf("ReviewRequests = %v and ran the Jira token command %d times, want an answer and no run",
			reviewErr, runs)
	}

	// Act: search Jira twice
	_, firstErr := deps.Jira.Search("assignee = currentUser()", 0)
	_, secondErr := deps.Jira.Search("assignee = currentUser()", 0)

	// Assert: it ran once, and both searches carried what it printed
	if firstErr != nil || secondErr != nil {
		t.Fatalf("Search = %v, then %v; want both answered", firstErr, secondErr)
	}

	if runs := tokens.runs(); runs != 1 {
		t.Errorf("two searches ran the Jira token command %d times, want once", runs)
	}

	if signed := jiraStandIn.authorization.Load(); signed != "Bearer "+recordedToken {
		t.Errorf("Jira was sent Authorization %q, want the command's token", signed)
	}
}

func TestAJiraTokenSourceThatGivesNoTokenIsReportedAsNoCredential(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		settings config.Jira
		want     string
	}{
		"a command that fails": {
			settings: config.Jira{TokenCommand: "false"},
			want:     "token command",
		},
		"a command that prints nothing": {
			settings: config.Jira{TokenCommand: "true"},
			want:     "token_command",
		},
		"an environment variable that is not set": {
			settings: config.Jira{TokenEnv: "WORKFLOW_TEST_VARIABLE_NEVER_SET"},
			want:     "WORKFLOW_TEST_VARIABLE_NEVER_SET",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			jiraStandIn := newStandInJira(t)
			cfg := config.Default()
			cfg.Jira = tt.settings
			cfg.Jira.BaseURL = jiraStandIn.url
			seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Jira

			// Act
			_, err := seams.Search("assignee = currentUser()", 0)

			// Assert
			if !errors.Is(err, jira.ErrNoCredential) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search = %v, want jira.ErrNoCredential naming %q", err, tt.want)
			}

			if asked := jiraStandIn.requests.Load(); asked != 0 {
				t.Errorf("Jira was asked %d times without a token, want never", asked)
			}
		})
	}
}

// Every seam that asks Jira meets a token command that fails the same way:
// as no credential, before anything is sent.
func TestEveryJiraSeamReportsATokenCommandThatFails(t *testing.T) {
	t.Parallel()

	cases := map[string]func(seams tui.JiraDeps) error{
		"Search": func(seams tui.JiraDeps) error {
			_, err := seams.Search("assignee = currentUser()", 0)

			return err
		},
		"Issue": func(seams tui.JiraDeps) error {
			_, err := seams.Issue("OPS-1")

			return err
		},
		"Transitions": func(seams tui.JiraDeps) error {
			_, err := seams.Transitions("OPS-1")

			return err
		},
		"Transition": func(seams tui.JiraDeps) error {
			return seams.Transition("OPS-1", jira.Transition{ID: "1"}, nil)
		},
		"Comment": func(seams tui.JiraDeps) error {
			_, err := seams.Comment("OPS-1", "hello")

			return err
		},
		"Assign": func(seams tui.JiraDeps) error {
			return seams.Assign("OPS-1", "someone")
		},
		"AddWorklog": func(seams tui.JiraDeps) error {
			_, err := seams.AddWorklog("OPS-1", "1h", "")

			return err
		},
		"LinkPullRequest": func(seams tui.JiraDeps) error {
			return seams.LinkPullRequest("OPS-1", "https://github.com/owner/repo/pull/7", "fix: token")
		},
	}

	for seam, act := range cases {
		t.Run(seam, func(t *testing.T) {
			t.Parallel()

			// Arrange
			jiraStandIn := newStandInJira(t)
			cfg := config.Default()
			cfg.Jira = config.Jira{BaseURL: jiraStandIn.url, TokenCommand: "false"}
			seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Jira

			// Act
			err := act(seams)

			// Assert
			if !errors.Is(err, jira.ErrNoCredential) {
				t.Errorf("%s = %v, want jira.ErrNoCredential", seam, err)
			}

			if asked := jiraStandIn.requests.Load(); asked != 0 {
				t.Errorf("%s asked Jira %d times without a token, want never", seam, asked)
			}
		})
	}
}

func TestAForgeIssueNumberNeverRunsTheJiraTokenCommand(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newTokenCommand(t)
	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: newStandInJira(t).url, TokenCommand: tokens.command}
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Jira

	// Act
	_, err := seams.Issue("42")

	// Assert
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("Issue(42) = %v, want jira.ErrNotFound", err)
	}

	if runs := tokens.runs(); runs != 0 {
		t.Errorf("reading a forge issue number ran the Jira token command %d times, want none", runs)
	}
}

func TestLinkingAJiraIssueNeverRunsItsTokenCommand(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newTokenCommand(t)
	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: jiraAddress, TokenCommand: tokens.command}
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Jira

	// Act
	link := seams.BrowseURL("OPS-1")

	// Assert
	if link != jiraAddress+"/browse/OPS-1" {
		t.Errorf("BrowseURL(OPS-1) = %q, want the issue's page", link)
	}

	if runs := tokens.runs(); runs != 0 {
		t.Errorf("linking an issue ran the Jira token command %d times, want none", runs)
	}
}

func TestResolvingAheadRunsTheJiraTokenCommandOnceForEveryLaterSearch(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newTokenCommand(t)
	jiraStandIn := newStandInJira(t)
	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: jiraStandIn.url, TokenCommand: tokens.command}
	deps, resolveAhead := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act: resolve ahead
	resolveAhead()

	// Assert: the token command ran, and Jira was not asked
	if runs, asked := tokens.runs(), jiraStandIn.requests.Load(); runs != 1 || asked != 0 {
		t.Fatalf("resolving ahead ran the Jira token command %d times and asked Jira %d times, want once and never",
			runs, asked)
	}

	// Act: search Jira
	_, searchErr := deps.Jira.Search("assignee = currentUser()", 0)

	// Assert: the search carried the token, and the command did not run again
	if searchErr != nil {
		t.Fatalf("Search = %v, want an answer", searchErr)
	}

	if runs := tokens.runs(); runs != 1 {
		t.Errorf("the search ran the Jira token command again, %d runs in all; want the one run ahead", runs)
	}

	if signed := jiraStandIn.authorization.Load(); signed != "Bearer "+recordedToken {
		t.Errorf("Jira was sent Authorization %q, want the command's token", signed)
	}
}

func TestResolvingAheadRetriesAFailedJiraTokenCommandOnFirstUse(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newFailingTokenCommand(t)
	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: newStandInJira(t).url, TokenCommand: tokens.command}
	deps, resolveAhead := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)
	resolveAhead()

	// Act
	_, err := deps.Jira.Search("assignee = currentUser()", 0)

	// Assert
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Search = %v, want jira.ErrNoCredential", err)
	}

	if runs := tokens.runs(); runs != 2 {
		t.Errorf("the Jira token command ran %d times, want once ahead and once more on first use", runs)
	}
}

func TestResolvingAheadLeavesTheTokenCommandOfAJiraNotInUseUnrun(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no base URL the forge's issues are the tracker, so Jira's token is
	// never needed.
	tokens := newTokenCommand(t)
	cfg := config.Default()
	cfg.Jira = config.Jira{TokenCommand: tokens.command}
	_, resolveAhead := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	resolveAhead()

	// Assert
	if runs := tokens.runs(); runs != 0 {
		t.Errorf("resolving ahead ran the token command of a Jira not in use %d times, want none", runs)
	}
}
