// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// A fork's clone on github.com: origin is the contributor's fork, upstream the
// repository it was forked from, and forkBranch a branch pushed to the fork.
const (
	forkOrigin   = "https://github.com/contributor/repo.git"
	forkUpstream = "https://github.com/owner/repo.git"
	forkBranch   = "feat/x"
)

// forkWorkspace is a fork's clone read as Locate reads it, its own git
// configuration kept out.
func forkWorkspace(t *testing.T) (wiring.Workspace, string) {
	t.Helper()

	isolateGit(t)

	root := repository(t)
	git(t, root, "remote", "add", "origin", forkOrigin)
	git(t, root, "remote", "add", upstreamRemote, forkUpstream)

	return processEnvironment().Locate(t.Context(), root), root
}

// forkForge is the forge seams over a fork's clone, reached through a stand-in
// gh, and the stand-in's recordings.
func forkForge(t *testing.T, configure ...[]string) (*forgeCLI, wiring.Workspace) {
	t.Helper()

	fake := installForgeCLI(t, "gh", forgeReplies{})
	where, root := forkWorkspace(t)

	for _, setting := range configure {
		git(t, root, append([]string{"config"}, setting...)...)
	}

	return fake, where
}

// forkRequest is a pull request from forkBranch into main.
func forkRequest() forge.NewPullRequest {
	return forge.NewPullRequest{Title: "work", Head: forge.Head{Branch: forkBranch}, Base: mainBranch}
}

// githubCLI is the configuration that reaches github.com through gh.
func githubCLI() config.Config {
	return config.Config{Forge: config.Forge{CLI: true, Kind: githubKind, Host: hostGitHub}}
}

func TestLocateReadsTheBaseRemoteBesideOrigin(t *testing.T) {
	// Act
	where, _ := forkWorkspace(t)

	// Assert
	want := gitrepo.Remote{Name: upstreamRemote, URL: forkUpstream}
	if where.Remote != forkOrigin || where.Base != want {
		t.Errorf("Locate = %+v; want origin %s and the base %+v", where, forkOrigin, want)
	}
}

func TestLocateTakesTheRemoteGhMarkedAsTheBase(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	git(t, root, "remote", "add", "origin", forkOrigin)
	git(t, root, "remote", "add", "parent", forkUpstream)
	git(t, root, "config", "remote.parent.gh-resolved", "base")

	// Act
	where := processEnvironment().Locate(t.Context(), root)

	// Assert
	if want := (gitrepo.Remote{Name: "parent", URL: forkUpstream}); where.Base != want {
		t.Errorf("Locate's base = %+v, want %+v", where.Base, want)
	}
}

func TestTheForgeIsAskedAboutTheBaseFromTheFork(t *testing.T) {
	// Arrange
	fake, where := forkForge(t)

	// Act
	_, _, err := wired(t, githubCLI(), where, nil).Forge.FindPullRequest(t.Context(), forkBranch)

	// Assert
	asked := strings.Join(fake.args(), " ")
	if err != nil || !strings.Contains(asked, "repos/owner/repo/pulls?") ||
		!strings.Contains(asked, "head=contributor%3Afeat%2Fx") {
		t.Errorf("FindPullRequest = %v asking %q; want upstream's pulls headed contributor:%s", err, asked, forkBranch)
	}
}

func TestAPullRequestOpensOnTheBaseFromTheFork(t *testing.T) {
	// Arrange
	fake, where := forkForge(t)
	request := forkRequest()

	// Act
	_, err := wired(t, githubCLI(), where, nil).Forge.CreatePullRequest(t.Context(), request)

	// Assert
	if err != nil || !slices.Contains(fake.args(), "https://api.github.com/repos/owner/repo/pulls") ||
		!strings.Contains(fake.stdin(), `"head":"contributor:feat/x"`) {
		t.Errorf("CreatePullRequest = %v, asking %q with %s; want it on upstream from contributor:%s",
			err, fake.args(), fake.stdin(), forkBranch)
	}
}

func TestABranchPushedToTheBaseOpensFromTheBase(t *testing.T) {
	// Arrange
	fake, where := forkForge(t, []string{"branch." + forkBranch + ".pushRemote", upstreamRemote})
	request := forkRequest()

	// Act
	_, err := wired(t, githubCLI(), where, nil).Forge.CreatePullRequest(t.Context(), request)

	// Assert
	if err != nil || !strings.Contains(fake.stdin(), `"head":"feat/x"`) {
		t.Errorf("CreatePullRequest = %v with %s; want the bare branch as the head", err, fake.stdin())
	}
}

func TestAPushRemoteThatNamesNoForgeIsRefused(t *testing.T) {
	// Arrange
	_, where := forkForge(t,
		[]string{"remote.mirror.url", "/srv/mirror.git"},
		[]string{"branch." + forkBranch + ".pushRemote", "mirror"})

	// Act
	_, _, err := wired(t, githubCLI(), where, nil).Forge.FindPullRequest(t.Context(), forkBranch)

	// Assert
	if !errors.Is(err, forge.ErrNotARemote) || !strings.Contains(err.Error(), "mirror") {
		t.Errorf("FindPullRequest = %v, want %v naming mirror", err, forge.ErrNotARemote)
	}
}

// What workflow keeps is keyed by origin, which a fork's clone shares with
// any other clone of the fork, whatever its base remote says.
func TestAForkCloneKeepsItsStoreUnderOrigin(t *testing.T) {
	// Arrange
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	fork, _ := forkWorkspace(t)
	merged := loop.Announced{Pull: 7, Moment: messaging.MomentMerged}

	err := wired(t, config.Default(), fork, nil).Store.RecordAnnounce(t.Context(), merged)
	if err != nil {
		t.Fatalf("recording the announcement: %v", err)
	}

	sameOrigin := wiring.Workspace{Root: t.TempDir(), Remote: forkOrigin, Base: gitrepo.Remote{}}

	// Act
	posts := processEnvironment().ReadOnlyStore(config.Default(), sameOrigin).Announced(t.Context())

	// Assert
	if len(posts) != 1 || posts[0] != merged {
		t.Errorf("a clone of the same origin read %+v, want the one announcement the fork's clone kept", posts)
	}
}

func TestHereNamesAForksBase(t *testing.T) {
	// Arrange
	where, _ := forkWorkspace(t)
	where.Repository = true

	// Act
	here := wired(t, config.Default(), where, nil).Repositories.Here

	// Assert
	if here.Remote.Path != "contributor/repo" || here.Base.Path != "owner/repo" {
		t.Errorf("Here = %+v; want origin contributor/repo and the base owner/repo", here)
	}
}

func TestHereNamesNoBaseOutsideAFork(t *testing.T) {
	// Act
	here := wired(t, config.Default(), onOrigin(t.TempDir(), remoteGitHub), nil).Repositories.Here

	// Assert
	if here.Base != (forge.Repo{}) {
		t.Errorf("Here.Base = %+v, want none where the base remote is origin", here.Base)
	}
}

func TestAPullRequestFromAPushRemoteNamingNoForgeIsNotOpened(t *testing.T) {
	// Arrange
	fake, where := forkForge(t,
		[]string{"remote.mirror.url", "/srv/mirror.git"},
		[]string{"branch." + forkBranch + ".pushRemote", "mirror"})
	request := forkRequest()

	// Act
	_, err := wired(t, githubCLI(), where, nil).Forge.CreatePullRequest(t.Context(), request)

	// Assert
	if !errors.Is(err, forge.ErrNotARemote) || fake.stdin() != "" {
		t.Errorf("CreatePullRequest = %v having sent %q; want %v and nothing sent", err, fake.stdin(), forge.ErrNotARemote)
	}
}
