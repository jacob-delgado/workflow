// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The forge seams' success arms — the branch each takes once the connection is
// made — are reached black-box by pointing the forge at its CLI and answering
// through a stand-in gh, rather than injecting the unexported connect. Each seam
// runs in a subtest of its own, against a stand-in of its own, so a failure
// names the seam and the recorded gh calls are that seam's alone.

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Every seam gets past its connect guard and returns the forge's answer, which
// the connection-failure tests never reach.
func TestTheForgeSeamsReturnTheForgesAnswerThroughTheCLI(t *testing.T) {
	cases := []struct {
		seam string
		act  func(seams tui.ForgeDeps) (string, error)
		want string
	}{
		{
			seam: "FindPullRequest",
			act: func(seams tui.ForgeDeps) (string, error) {
				_, found, err := seams.FindPullRequest("feat/x")

				return fmt.Sprintf("found=%t", found), err
			},
			want: "found=false",
		},
		{
			seam: "CreatePullRequest",
			act: func(seams tui.ForgeDeps) (string, error) {
				created, err := seams.CreatePullRequest(forge.NewPullRequest{Title: "fix: token"})

				return fmt.Sprintf("#%d", created.Number), err
			},
			want: "#43",
		},
		{
			seam: "CheckStatus",
			act: func(seams tui.ForgeDeps) (string, error) {
				ci, err := seams.CheckStatus(forge.PullRequest{Number: 43}, "abc123")

				return fmt.Sprintf("%d checks", len(ci.Checks)), err
			},
			want: "0 checks",
		},
		{
			seam: "ReviewRequests",
			act: func(seams tui.ForgeDeps) (string, error) {
				reviews, err := seams.ReviewRequests()

				return fmt.Sprintf("%d requests", len(reviews)), err
			},
			want: "0 requests",
		},
		{
			seam: "Author",
			act: func(seams tui.ForgeDeps) (string, error) {
				return seams.Author()
			},
			want: "octo",
		},
	}

	for _, seamCase := range cases {
		t.Run(seamCase.seam, func(t *testing.T) {
			// Arrange
			installForgeCLI(t, "gh", forgeReplies{
				create: `{"number":43,"html_url":"https://github.com/owner/repo/pull/43","title":"fix: token","state":"open"}`,
			})

			cfg, where := githubCLIWorkspace(t)

			seams := wiring.Deps(t.Context(), cfg, where, nil).Forge

			// Act
			got, err := seamCase.act(seams)

			// Assert
			if err != nil || got != seamCase.want {
				t.Errorf("%s = %q, %v; want %q and no error", seamCase.seam, got, err, seamCase.want)
			}
		})
	}
}

// Every write seam gets past its connect guard, hands gh its request — the
// endpoint, the method, and the body on standard input — and returns the
// forge's answer: a repository that allows no merge method, and a head with no
// failed run. The read seams leave wantStdin empty: a read sends gh nothing on
// standard input, and the check holds them to that.
func TestTheForgeWriteSeamsHandTheForgeTheirRequestThroughTheCLI(t *testing.T) {
	pull := forge.PullRequest{Number: 43}

	cases := []struct {
		seam      string
		act       func(seams tui.ForgeDeps) (string, error)
		want      string
		wantArgs  []string
		wantStdin string
	}{
		{
			seam: "EditPullRequest",
			act: func(seams tui.ForgeDeps) (string, error) {
				_, err := seams.EditPullRequest(pull, forge.PullRequestEdit{Title: "retitled"})

				return "", err
			},
			wantArgs:  []string{"PATCH", "https://api.github.com/repos/owner/repo/pulls/43"},
			wantStdin: `{"title":"retitled","body":""}`,
		},
		{
			seam: "Merge",
			act: func(seams tui.ForgeDeps) (string, error) {
				return "", seams.Merge(pull, forge.MergeSquash)
			},
			wantArgs:  []string{"PUT", "https://api.github.com/repos/owner/repo/pulls/43/merge"},
			wantStdin: `{"merge_method":"squash"}`,
		},
		{
			seam: "MergeMethods",
			act: func(seams tui.ForgeDeps) (string, error) {
				methods, err := seams.MergeMethods()

				return fmt.Sprintf("%d methods", len(methods)), err
			},
			want:     "0 methods",
			wantArgs: []string{"https://api.github.com/repos/owner/repo"},
		},
		{
			seam: "Rerun",
			act: func(seams tui.ForgeDeps) (string, error) {
				reran, err := seams.Rerun(pull, "abc123")

				return fmt.Sprintf("reran=%t", reran), err
			},
			want:     "reran=false",
			wantArgs: []string{"https://api.github.com/repos/owner/repo/actions/runs?head_sha=abc123&page=1&per_page=100"},
		},
	}

	for _, seamCase := range cases {
		t.Run(seamCase.seam, func(t *testing.T) {
			// Arrange
			ghStub := installForgeCLI(t, "gh", forgeReplies{})
			cfg, where := githubCLIWorkspace(t)

			seams := wiring.Deps(t.Context(), cfg, where, nil).Forge

			// Act
			got, err := seamCase.act(seams)

			// Assert
			if err != nil || got != seamCase.want {
				t.Errorf("%s = %q, %v; want %q and no error", seamCase.seam, got, err, seamCase.want)
			}

			if args := ghStub.args(); !containsAll(args, seamCase.wantArgs...) {
				t.Errorf("gh was called as %v, want it asked with %v", args, seamCase.wantArgs)
			}

			if body := ghStub.stdin(); body != seamCase.wantStdin {
				t.Errorf("gh received %q on standard input, want %q", body, seamCase.wantStdin)
			}
		})
	}
}

// Without Jira the forge's issues back the tracker, so the tracker and the
// forge seams reach one forge through one connection: the token is looked up
// once for both, not once for each.
func TestTheTrackerAndForgeSeamsLookUpTheForgeTokenOnce(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{})
	t.Setenv("GITHUB_TOKEN", "")

	cfg, where := githubCLIWorkspace(t)
	deps := wiring.Deps(t.Context(), cfg, where, nil)

	// Act
	_, searchErr := deps.Jira.Search("", 0)
	_, _, findErr := deps.Forge.FindPullRequest(featureBranch)

	// Assert
	if searchErr != nil || findErr != nil {
		t.Fatalf("Search = %v, FindPullRequest = %v; want both answered", searchErr, findErr)
	}

	if lookups := ghStub.tokenLookups(); lookups != 1 {
		t.Errorf("gh auth token ran %d times across Search and FindPullRequest, want once", lookups)
	}
}
