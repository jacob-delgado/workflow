// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		want   error
	}{
		"no remote":          {remote: "", kind: "", want: forge.ErrNotARemote},
		"an unnamed host":    {remote: unnamedHost, kind: "", want: forge.ErrUnknownForge},
		"a kind that is not": {remote: unnamedHost, kind: "bitbucket", want: forge.ErrUnknownForge},
		"no token anywhere":  {remote: githubRemote, kind: "", want: forge.ErrNoToken},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Forge.Kind = tt.kind

			seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: tt.remote}).Forge

			_, _, findErr := seams.FindPullRequest("x")
			_, createErr := seams.CreatePullRequest(forge.NewPullRequest{})
			_, checkErr := seams.CheckStatus(forge.PullRequest{}, "abc")
			_, authorErr := seams.Author()

			for seam, err := range map[string]error{
				"FindPullRequest": findErr, "CreatePullRequest": createErr, "CheckStatus": checkErr, "Author": authorErr,
			} {
				if !errors.Is(err, tt.want) {
					t.Errorf("%s = %v, want %v", seam, err, tt.want)
				}
			}
		})
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

	github := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: root, Remote: githubRemote})
	if found := github.Forge.Templates(); len(found) != 1 || found[0].Body != "## What\n" {
		t.Errorf("Templates = %+v, want the repository's", found)
	}

	for _, where := range []wiring.Workspace{
		{Root: root, Remote: ""},
		{Root: root, Remote: unnamedHost},
	} {
		cfg := config.Default()
		if where.Remote != "" {
			cfg.Forge.Kind = "bogus"
		}

		if found := wiring.Deps(t.Context(), cfg, where).Forge.Templates(); len(found) != 0 {
			t.Errorf("Templates for %+v = %+v, want none", where, found)
		}
	}
}

func TestTheJiraSeamsReachTheConfiguredJira(t *testing.T) {
	t.Parallel()

	var asked []string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked = append(asked, request.Method+" "+request.URL.Path)

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"total":0,"issues":[],"transitions":[]}`))
	}))
	t.Cleanup(server.Close)

	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: server.URL, Token: "a-token-for-tests", User: ""}

	seams := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}).Jira

	_, err := seams.Search()
	if err != nil {
		t.Errorf("Search: %v", err)
	}

	_, _ = seams.Issue("OPS-1")
	_, _ = seams.Transitions("OPS-1")
	_ = seams.Transition("OPS-1", jira.Transition{ID: "1"}, nil)
	_, _ = seams.Comment("OPS-1", "hello")

	want := []string{
		"GET /rest/api/2/search", "GET /rest/api/2/issue/OPS-1", "GET /rest/api/2/issue/OPS-1/transitions",
		"POST /rest/api/2/issue/OPS-1/transitions", "POST /rest/api/2/issue/OPS-1/comment",
	}
	if strings.Join(asked, "|") != strings.Join(want, "|") {
		t.Errorf("asked %q, want %q", asked, want)
	}

	if link := seams.BrowseURL("OPS-1"); link != server.URL+"/browse/OPS-1" {
		t.Errorf("BrowseURL = %q", link)
	}
}

func TestTheSlackSeamPostsThroughTheConfiguredTransport(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Slack = config.Slack{Token: "", WebhookURL: "http://hooks.example.com/services/x", Channel: ""}

	err := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}).Slack.Post("hi")
	if !errors.Is(err, slack.ErrInsecureWebhook) {
		t.Errorf("Post = %v, want the webhook refused before anything is sent", err)
	}
}

func TestTheEditorSeamsHandOverTheTerminal(t *testing.T) {
	t.Parallel()

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}).Editor

	if seams.Edit("text", "help", nil) == nil || seams.Open("main.go", 1, nil) == nil {
		t.Error("an editor seam built no command")
	}
}
