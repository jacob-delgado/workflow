// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// ReachForge is how doctor --online and every forge seam reach the forge: over
// HTTP with the token it found, or through gh or glab, which sign each request
// themselves and are handed its body on standard input.

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubAPI is the API base of github.com, which no test here ever reaches.
const githubAPI = "https://api.github.com"

// errBodyGone is a request body that fails as it is read.
var errBodyGone = errors.New("the body went away")

// githubOwnerRepo is the repository remoteGitHub names, as the forge reads it.
func githubOwnerRepo() forge.Repo {
	return forge.Repo{Kind: forge.KindGitHub, Host: hostGitHub, Path: "owner/repo"}
}

func TestReachingAForgeOverHTTPCarriesTheTokenItFound(t *testing.T) {
	// Arrange
	// GitHub's own variable holds the token, and there is no gh to ask.
	const token = "forge-token-for-tests"

	t.Setenv("GITHUB_TOKEN", token)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())

	// Act
	process := processEnvironment()

	access, err := process.ReachForge(t.Context(), config.Forge{}, githubOwnerRepo(), githubAPI, http.DefaultClient.Do)

	// Assert
	if err != nil || access.Token.Secret() != token {
		t.Fatalf("ReachForge = %v, carrying %v; want the token from the environment", err, access.Token)
	}

	if access.Via != "token from the environment" {
		t.Errorf("ReachForge says it reached the forge %q, want by the token from the environment", access.Via)
	}
}

func TestTheForgeCLIRunsNothingForARequestBodyItCannotRead(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	cfg, _ := githubCLIWorkspace(t)

	process := processEnvironment()

	access, err := process.ReachForge(t.Context(), cfg.Forge, githubOwnerRepo(), githubAPI, http.DefaultClient.Do)
	if err != nil {
		t.Fatalf("ReachForge through gh: %v", err)
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		githubAPI+"/repos/owner/repo/pulls", iotest.ErrReader(errBodyGone))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	//nolint:bodyclose // no request leaves, so there is no answer to close; the Assert requires it nil.
	response, err := access.Doer(request)

	// Assert
	if response != nil || !errors.Is(err, errBodyGone) || !strings.Contains(err.Error(), "reading the request body") {
		t.Errorf("Doer = %v, %v; want no answer and the body's own failure", response, err)
	}

	if args := ghStub.args(); len(args) != 0 {
		t.Errorf("gh was run as %v for a request whose body could not be read", args)
	}
}

func TestReachingAForgeThroughItsCLIAsksForNoToken(t *testing.T) {
	// Arrange
	// No token in the environment, so finding one would ask gh for it: the CLI
	// signs every request itself, and that ask would be for nothing.
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	t.Setenv("GITHUB_TOKEN", "")

	cfg, _ := githubCLIWorkspace(t)

	// Act
	process := processEnvironment()

	access, err := process.ReachForge(t.Context(), cfg.Forge, githubOwnerRepo(), githubAPI, http.DefaultClient.Do)

	// Assert
	if err != nil || access.Via != "through gh" {
		t.Fatalf("ReachForge = %+v, %v; want the forge reached through gh", access, err)
	}

	if lookups := ghStub.tokenLookups(); lookups != 0 {
		t.Errorf("gh auth token ran %d times, want none: the CLI carries its own login", lookups)
	}
}
