// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The CLI transport translates a forge request into a gh/glab `api` invocation
// and parses the reply. These drive it through wiring.Deps against a stand-in
// gh or glab that records what it was asked to do, so both the invocation and
// the transport's failure handling are observed without a real forge tool.

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheForgeCLIReadsThroughGHWithTheFullURL(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, found, err := seams.FindPullRequest("feat/x")

	// Assert
	if err != nil || found {
		t.Fatalf("FindPullRequest = %v, %v; want the fake's empty answer and no error", found, err)
	}

	args := ghStub.args()
	if len(args) == 0 || args[0] != "api" {
		t.Fatalf("gh was called as %v, want an `api` invocation", args)
	}

	if last := args[len(args)-1]; !strings.HasPrefix(last, "https://api.github.com/repos/owner/repo/pulls?") {
		t.Errorf("gh read %q, want the full GitHub URL passed through unchanged", last)
	}
}

func TestTheForgeCLISendsAWriteBodyOnStandardInput(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, err := seams.CreatePullRequest(forge.NewPullRequest{Title: "fix: token"})
	// Assert
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}

	if args := ghStub.args(); !containsAll(args, "-X", http.MethodPost, "--input", "-") {
		t.Errorf("gh was called as %v, want a POST reading its body from standard input", args)
	}

	if body := ghStub.stdin(); !strings.Contains(body, `"fix: token"`) {
		t.Errorf("gh received %q on standard input, want the pull request payload", body)
	}
}

func TestTheForgeCLITrimsTheBaseForGitLab(t *testing.T) {
	// Arrange
	glab := installForgeCLI(t, "glab", forgeReplies{})
	cfg := config.Config{Forge: config.Forge{CLI: true}}
	where := wiring.Workspace{Root: t.TempDir(), Remote: "https://gitlab.com/owner/repo.git"}

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, _, err := seams.FindPullRequest("feat/x")
	// Assert
	if err != nil {
		t.Fatalf("FindPullRequest through glab: %v", err)
	}

	args := glab.args()
	if len(args) == 0 {
		t.Fatalf("glab was never called")
	}

	last := args[len(args)-1]

	underRoot := strings.HasPrefix(last, "projects/") && strings.Contains(last, "merge_requests")
	if strings.HasPrefix(last, "https://") || !underRoot {
		t.Errorf("glab endpoint = %q, want the path under its API root with the base trimmed off", last)
	}
}

func TestTheForgeCLIReportsAnUnreadableReply(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{garbage: true})
	cfg, where := githubCLIWorkspace(t)

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, _, err := seams.FindPullRequest("feat/x")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "unreadable response") {
		t.Errorf("FindPullRequest = %v, want an unreadable-response error from the CLI transport", err)
	}
}

func TestTheForgeCLIReportsACommandFailure(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{fail: true})
	cfg, where := githubCLIWorkspace(t)

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, _, err := seams.FindPullRequest("feat/x")

	// Assert
	// The run's own failure is reported, not the parse of the garbage it left.
	saidWhyItFailed := err != nil && strings.Contains(err.Error(), "boom on stderr")
	if !saidWhyItFailed || strings.Contains(err.Error(), "unreadable response") {
		t.Errorf("FindPullRequest = %v, want the CLI command's own failure surfaced", err)
	}
}

func TestTheForgeFallsBackToHTTPWhenTheCLIToolIsMissing(t *testing.T) {
	// Arrange
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no gh here

	cfg := config.Config{Forge: config.Forge{CLI: true, Kind: githubKind, Host: hostGitHub}}
	where := wiring.Workspace{Root: t.TempDir(), Remote: remoteGitHub}

	seams := wired(t, cfg, where, nil).Forge

	// Act
	_, _, err := seams.FindPullRequest("feat/x")

	// Assert
	// The CLI was asked for but is not installed, so HTTP is used — and HTTP
	// needs a token the CLI would have carried itself.
	if !errors.Is(err, forge.ErrNoToken) {
		t.Errorf("FindPullRequest = %v, want %v", err, forge.ErrNoToken)
	}
}
