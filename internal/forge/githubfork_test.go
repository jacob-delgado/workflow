// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// contributorFork is a fork of githubRepo under another owner, and
// sameOwnerFork one under githubRepo's own.
const (
	contributorFork = "contributor/repo"
	sameOwnerFork   = "example/repo-fork"
)

// forkPulls answers the base's pull listing with two open pull requests from
// one branch name: number 3 from someone else's fork, number 4 from fork. A
// pull request opened is number 4.
func forkPulls(fork string) func(recorded) (int, string) {
	return func(asked recorded) (int, string) {
		switch {
		case asked.method == http.MethodPost:
			return http.StatusCreated, `{"number":4,"html_url":"https://x/4"}`
		case asked.path != githubPullsPath:
			return http.StatusOK, "{}"
		}

		return http.StatusOK, `[` +
			`{"number":3,"html_url":"https://x/3","title":"theirs","head":{"repo":{"full_name":"someone/repo"}}},` +
			`{"number":4,"html_url":"https://x/4","title":"ours","head":{"repo":{"full_name":"` + fork + `"}}}]`
	}
}

func TestGitHubFindsAForksPullRequestByItsOwner(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, forkPulls(contributorFork))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), githubRepo(),
		forge.Head{Branch: featureBranch, Fork: contributorFork})

	// Assert
	if err != nil || !found || pull.Number != 4 {
		t.Fatalf("FindPullRequest = %+v, %v, %v; want the fork's #4", pull, found, err)
	}

	query, _ := url.ParseQuery((*seen)[0].query)
	if (*seen)[0].path != githubPullsPath || query.Get("head") != "contributor:"+featureBranch {
		t.Errorf("asked %s?%s, want the base's pulls with head contributor:%s",
			(*seen)[0].path, (*seen)[0].query, featureBranch)
	}
}

func TestGitHubOpensAForksPullRequestOnTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusCreated, `{"number":4,"html_url":"https://x/4"}`))
	request := forge.NewPullRequest{
		Title: prTitle, Head: forge.Head{Branch: featureBranch, Fork: contributorFork}, Base: baseBranch,
	}

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), request)

	// Assert
	sent := (*seen)[0]
	_, namesRepo := sent.body["head_repo"]

	if err != nil || sent.method != http.MethodPost || sent.path != githubPullsPath ||
		sent.body["head"] != "contributor:"+featureBranch || sent.body["base"] != baseBranch || namesRepo {
		t.Errorf("CreatePullRequest = %v, sent %s %s %v; want the base's pulls with head contributor:%s",
			err, sent.method, sent.path, sent.body, featureBranch)
	}
}

// GitHub cannot tell two repositories of one owner apart by an owner:branch
// head, so a fork in the base's own organization names itself in head_repo.
func TestGitHubNamesAForkInTheBasesOwnOrganization(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, forkPulls(sameOwnerFork))
	request := forge.NewPullRequest{
		Title: prTitle, Head: forge.Head{Branch: featureBranch, Fork: sameOwnerFork}, Base: baseBranch,
	}

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), request)

	// Assert
	sent := (*seen)[0].body
	if err != nil || sent["head"] != "example:"+featureBranch || sent["head_repo"] != sameOwnerFork {
		t.Errorf("CreatePullRequest = %v, sent %v; want head example:%s and head_repo %s",
			err, sent, featureBranch, sameOwnerFork)
	}
}

// The base's owner's pulls from the branch include the base's own; only the
// fork's is the head's.
func TestGitHubFindsOnlyTheForksPullInTheBasesOwnOrganization(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, forkPulls(sameOwnerFork))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), githubRepo(),
		forge.Head{Branch: featureBranch, Fork: sameOwnerFork})

	// Assert
	if err != nil || !found || pull.Number != 4 {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the fork's #4", pull, found, err)
	}
}

func TestAHeadWhoseForkIsTheBaseIsNoFork(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusCreated, `{"number":4,"html_url":"https://x/4"}`))
	head := forge.Head{Branch: featureBranch, Fork: "Example/Repo"}

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(),
		forge.NewPullRequest{Title: prTitle, Head: head, Base: baseBranch})

	// Assert
	if sent := (*seen)[0].body; err != nil || sent["head"] != featureBranch {
		t.Errorf("CreatePullRequest = %v, sent %v; want the bare branch as the head", err, sent)
	}
}

// The base's owner can hold a fork of it too, with a branch of the same name;
// a head on the base itself is not that fork's pull request.
func TestGitHubFindPassesOverASameOwnerForksPullForAHeadOnTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, forkPulls("example/repo"))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), githubRepo(), forge.Head{Branch: featureBranch})

	// Assert
	if err != nil || !found || pull.Number != 4 {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the base's own #4", pull, found, err)
	}
}
