// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The forge seams' success arms — the branch each takes once the connection is
// made — are reached black-box by pointing the forge at its CLI and answering
// through a stand-in gh, rather than injecting the unexported connect.

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheForgeSeamsReturnTheForgesAnswerThroughTheCLI(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{
		create: `{"number":43,"html_url":"https://github.com/owner/repo/pull/43","title":"fix: token","state":"open"}`,
	})

	cfg, where := githubCLIWorkspace(t)

	seams := wiring.Deps(t.Context(), cfg, where, nil).Forge

	// Act
	_, found, findErr := seams.FindPullRequest("feat/x")
	created, createErr := seams.CreatePullRequest(forge.NewPullRequest{Title: "fix: token"})
	_, statusErr := seams.CheckStatus(forge.PullRequest{Number: 43}, "abc123")
	reviews, reviewErr := seams.ReviewRequests()
	author, authorErr := seams.Author()

	// Assert
	// Every seam gets past its connect guard and returns the forge's answer,
	// which the connection-failure tests never reach.
	if findErr != nil || found {
		t.Errorf("FindPullRequest = %v, %v; want no pull found and no error", found, findErr)
	}

	if createErr != nil || created.Number != 43 {
		t.Errorf("CreatePullRequest = %+v, %v; want the created pull #43", created, createErr)
	}

	if statusErr != nil {
		t.Errorf("CheckStatus returned %v, want the forge's answer", statusErr)
	}

	if reviewErr != nil || len(reviews) != 0 {
		t.Errorf("ReviewRequests = %+v, %v; want an empty list and no error", reviews, reviewErr)
	}

	if authorErr != nil || author != "octo" {
		t.Errorf("Author = %q, %v; want the login the forge reported", author, authorErr)
	}
}

func TestTheForgeWriteSeamsHandTheForgeTheirRequestThroughTheCLI(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)

	seams := wiring.Deps(t.Context(), cfg, where, nil).Forge
	pull := forge.PullRequest{Number: 43}

	// Act
	_, editErr := seams.EditPullRequest(pull, forge.PullRequestEdit{Title: "retitled"})
	mergeErr := seams.Merge(pull, forge.MergeSquash)
	methods, methodsErr := seams.MergeMethods()
	reran, rerunErr := seams.Rerun(pull, "abc123")

	// Assert
	// Every seam gets past its connect guard and returns the forge's answer:
	// a repository that allows no merge method, and a head with no failed run.
	for seam, err := range map[string]error{
		"EditPullRequest": editErr, "Merge": mergeErr, "MergeMethods": methodsErr, "Rerun": rerunErr,
	} {
		if err != nil {
			t.Errorf("%s returned %v, want the forge's answer", seam, err)
		}
	}

	if len(methods) != 0 || reran {
		t.Errorf("MergeMethods = %v, Rerun = %v; want no methods and nothing re-run", methods, reran)
	}

	// Each seam's request reached gh: the edit, the merge, the repository's
	// merge settings, and the head commit's workflow runs.
	args := ghStub.args()
	for _, asked := range []string{
		"https://api.github.com/repos/owner/repo/pulls/43",
		"https://api.github.com/repos/owner/repo/pulls/43/merge",
		"https://api.github.com/repos/owner/repo",
		"https://api.github.com/repos/owner/repo/actions/runs?head_sha=abc123&per_page=100",
	} {
		if !slices.Contains(args, asked) {
			t.Errorf("gh was never asked for %s; it was called as %v", asked, args)
		}
	}

	if !containsAll(args, "PATCH", "PUT") {
		t.Errorf("gh was called as %v, want the edit sent as a PATCH and the merge as a PUT", args)
	}

	if body := ghStub.stdin(); !strings.Contains(body, `"merge_method":"squash"`) {
		t.Errorf("gh received %q on standard input, want the squash merge asked for", body)
	}
}
