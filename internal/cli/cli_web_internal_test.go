// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// The --web flag serves the web interface instead of opening the TUI, and
// serveWeb binds a port. These inject a fake server in its place so the flag's
// wiring — that it routes to the server, carries dry run, and maps the seams —
// is tested without a real listener.

import (
	"context"
	"io"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestTheWebFlagServesInsteadOfOpeningTheTUI(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotInfo webserver.Info

	served := false
	serve := func(_ context.Context, _ config.Config, _ webserver.Deps, info webserver.Info, _ io.Writer) error {
		served = true
		gotInfo = info

		return nil
	}
	run := func(context.Context, tui.Model, io.Writer) error {
		t.Error("the TUI ran; --web should serve the web interface instead")

		return nil
	}

	root := newRootCmd(Prompt{}, run, serve)
	root.SetArgs([]string{"--web", "--dry-run"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	// Act
	err := root.Execute()
	// Assert
	if err != nil {
		t.Fatalf("the --web command returned %v, want it to serve", err)
	}

	if !served {
		t.Error("the --web flag did not start the web server")
	}

	if !gotInfo.DryRun || gotInfo.Version != buildinfo.Current() {
		t.Errorf("info = %+v, want dry run and the build version to reach the server", gotInfo)
	}
}

func TestWebDepsWiresEverySeam(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := tui.Deps{
		Jira: tui.JiraDeps{
			Search: func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, nil },
			Issue:  func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, nil },
		},
		Git: tui.GitDeps{
			Branch:  func() (gitrepo.Branch, error) { return gitrepo.Branch{}, nil },
			Changes: func() ([]gitrepo.Change, error) { return nil, nil },
		},
		Forge: tui.ForgeDeps{
			FindPullRequest: func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil },
			CheckStatus:     func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{}, nil },
			Author:          func() (string, error) { return "", nil },
		},
	}

	// Act
	web := webDeps(deps)

	// Assert
	if web.Search == nil || web.Issue == nil || web.Branch == nil || web.Changes == nil {
		t.Error("webDeps left a Jira or Git seam unwired")
	}

	if web.FindPull == nil || web.CheckCI == nil || web.Author == nil {
		t.Error("webDeps left a forge seam unwired")
	}
}

func TestWebDepsLeavesUnconfiguredSeamsNil(t *testing.T) {
	t.Parallel()

	// Act
	web := webDeps(tui.Deps{})

	// Assert
	if web.Search != nil || web.Issue != nil || web.Branch != nil || web.Changes != nil {
		t.Error("webDeps invented a Jira or Git seam the interface did not provide")
	}

	if web.FindPull != nil || web.CheckCI != nil || web.Author != nil {
		t.Error("webDeps invented a forge seam the interface did not provide")
	}
}
