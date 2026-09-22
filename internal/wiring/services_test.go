// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/slack"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheForgeSeamsExplainAForgeThatCannotBeReached(t *testing.T) {
	// No token source at all: nothing in the environment, and no gh on PATH to
	// ask — so nothing here can reach a real forge.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())

	cases := map[string]struct {
		remote string
		kind   string
		host   string
		want   error
	}{
		"no remote":                {remote: "", kind: "", host: "", want: forge.ErrNotARemote},
		"an unnamed host":          {remote: unnamedHost, kind: "", host: "", want: forge.ErrUnknownForge},
		"a kind that is not":       {remote: unnamedHost, kind: "bitbucket", host: "", want: forge.ErrUnknownForge},
		"a kind without its host":  {remote: unnamedHost, kind: githubKind, host: "", want: forge.ErrKindNeedsHost},
		"a kind for another host":  {remote: unnamedHost, kind: githubKind, host: "other.host", want: forge.ErrUnknownForge},
		"no token anywhere":        {remote: githubRemote, kind: "", host: "", want: forge.ErrNoToken},
		"no token for a named one": {remote: unnamedHost, kind: githubKind, host: unnamedHostName, want: forge.ErrNoToken},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cfg := config.Default()
			cfg.Forge.Kind, cfg.Forge.Host = tt.kind, tt.host

			seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: tt.remote}, nil).Forge

			// Act
			// One behavior seen through each seam: every one of them connects to
			// the forge first, and must say why it could not.
			_, _, findErr := seams.FindPullRequest("x")
			_, createErr := seams.CreatePullRequest(forge.NewPullRequest{})
			_, editErr := seams.EditPullRequest(forge.PullRequest{}, forge.PullRequestEdit{})
			_, checkErr := seams.CheckStatus(forge.PullRequest{}, "abc")
			_, authorErr := seams.Author()

			// Assert
			for seam, err := range map[string]error{
				"FindPullRequest": findErr, "CreatePullRequest": createErr, "EditPullRequest": editErr,
				"CheckStatus": checkErr, "Author": authorErr,
			} {
				if !errors.Is(err, tt.want) {
					t.Errorf("%s = %v, want %v", seam, err, tt.want)
				}
			}
		})
	}
}

func TestTheForgeSeamsOfferGitHubsOwnVariableOnlyToGitHubsOwnHosts(t *testing.T) {
	// Arrange
	// A host the configuration names as GitHub, with only GitHub's own variable
	// set: that variable is for github.com, so there is no token for this host,
	// and the seam says so without asking anyone.
	t.Setenv("GITHUB_TOKEN", "forge-token-for-tests")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GH_HOST", "")
	t.Setenv("PATH", t.TempDir())

	cfg := config.Default()
	cfg.Forge.Kind, cfg.Forge.Host = githubKind, unnamedHostName

	seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: unnamedHost}, nil).Forge

	// Act
	_, _, err := seams.FindPullRequest("x")

	// Assert
	if !errors.Is(err, forge.ErrNoToken) {
		t.Errorf("FindPullRequest = %v, want %v", err, forge.ErrNoToken)
	}
}

func TestTemplatesAreReadFromTheRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	err := os.MkdirAll(filepath.Join(root, ".github"), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, ".github", "PULL_REQUEST_TEMPLATE.md"), "## What\n", 0o600)

	cases := map[string]struct {
		remote string
		kind   string
		want   []string
	}{
		"a github repository's":          {remote: githubRemote, kind: "", want: []string{"## What\n"}},
		"none without a remote":          {remote: "", kind: "", want: nil},
		"none on a host naming no forge": {remote: unnamedHost, kind: "bogus", want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Forge.Kind = tt.kind

			seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: root, Remote: tt.remote}, nil).Forge

			// Act
			found := seams.Templates()

			// Assert
			bodies := make([]string, 0, len(found))
			for _, template := range found {
				bodies = append(bodies, template.Body)
			}

			if !slices.Equal(bodies, tt.want) {
				t.Errorf("Templates = %q, want %q", bodies, tt.want)
			}
		})
	}
}

func TestTheJiraSeamsReachTheConfiguredJira(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		lock  sync.Mutex
		asked []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lock.Lock()

		asked = append(asked, request.Method+" "+request.URL.Path)

		lock.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"total":0,"issues":[],"transitions":[]}`))
	}))
	t.Cleanup(server.Close)

	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: server.URL, Token: "a-token-for-tests", User: ""}

	seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Jira

	// Act
	_, searchErr := seams.Search("assignee = currentUser()", 0)
	_, issueErr := seams.Issue("OPS-1")
	_, listErr := seams.Transitions("OPS-1")
	moveErr := seams.Transition("OPS-1", jira.Transition{ID: "1"}, nil)
	_, commentErr := seams.Comment("OPS-1", "hello")
	link := seams.BrowseURL("OPS-1")

	// Assert
	for seam, err := range map[string]error{
		"Search": searchErr, "Issue": issueErr, "Transitions": listErr, "Transition": moveErr, "Comment": commentErr,
	} {
		if err != nil {
			t.Errorf("%s: %v", seam, err)
		}
	}

	want := []string{
		"GET /rest/api/2/search", "GET /rest/api/2/issue/OPS-1", "GET /rest/api/2/issue/OPS-1/transitions",
		"POST /rest/api/2/issue/OPS-1/transitions", "POST /rest/api/2/issue/OPS-1/comment",
	}

	lock.Lock()
	defer lock.Unlock()

	if !slices.Equal(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}

	if link != server.URL+"/browse/OPS-1" {
		t.Errorf("BrowseURL = %q", link)
	}
}

func TestTheSlackSeamRefusesAnInsecureWebhookBeforeSending(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging = config.Messaging{Token: "", WebhookURL: "http://hooks.example.com/services/x", Channel: ""}

	seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Slack

	// Act
	err := seams.Post("", "hi")

	// Assert
	if !errors.Is(err, slack.ErrInsecureWebhook) {
		t.Errorf("Post = %v, want the webhook refused before anything is sent", err)
	}
}

// installedEditor is set as the editor: a program every machine running these
// tests has. The command is built and handed over, never run.
func installedEditor(t *testing.T) {
	t.Helper()
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "go")
}

func TestTheEditSeamWritesADraftAndHandsOverTheTerminal(t *testing.T) {
	// Arrange
	installedEditor(t)

	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Editor
	finished := false

	// Act
	msg := seams.Edit("text", "help", func(string, error) tea.Msg {
		finished = true

		return nil
	})()

	// Assert
	if msg == nil || finished {
		t.Errorf("Edit returned %v and finished %v, want a handover and nothing reported yet", msg, finished)
	}

	written, _ := filepath.Glob(filepath.Join(drafts, "workflow-*.md"))
	if len(written) != 1 {
		t.Errorf("found drafts %q, want exactly one, in $TMPDIR", written)
	}
}

func TestTheOpenSeamHandsOverTheTerminal(t *testing.T) {
	// Arrange
	installedEditor(t)

	root := t.TempDir()

	err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: root, Remote: ""}, nil).Editor
	finished := false

	// Act
	msg := seams.Open("main.go", 1, func(error) tea.Msg {
		finished = true

		return nil
	})()

	// Assert
	if msg == nil || finished {
		t.Errorf("Open returned %v and finished %v, want a handover and nothing reported yet", msg, finished)
	}
}
