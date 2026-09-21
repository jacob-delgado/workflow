// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// editTitle and editBody are what an edit sends, shared so the request the test
// asserts on and the value it passes cannot drift apart.
const (
	editTitle = "fix: redact tokens"
	editBody  = "why"
)

func TestEditPullRequestOnGitHubPatchesTitleAndBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeConversation(t, map[string]string{
		githubPullsPath + "/43": `{"number":43,"html_url":"https://x/43","title":"` + editTitle + `","body":"why"}`,
	}, nil)

	// Act
	edited, err := client.EditPullRequest(t.Context(), githubRepo(),
		forge.PullRequest{Number: 43}, forge.PullRequestEdit{Title: editTitle, Body: editBody})

	// Assert
	if err != nil || edited.Title != editTitle {
		t.Fatalf("EditPullRequest = %+v, %v", edited, err)
	}

	request := requestTo(*seen, githubPullsPath+"/43")
	if request.method != http.MethodPatch || request.body["title"] != editTitle ||
		request.body["body"] != editBody {
		t.Errorf("edit request = %+v, want a PATCH of the title and body", request)
	}
}

func TestEditPullRequestOnGitLabPutsTitleAndDescription(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeConversation(t, map[string]string{
		gitlabMergesPath + "/43": `{"iid":43,"web_url":"https://x/43","title":"` + editTitle + `","description":"why"}`,
	}, nil)

	// Act
	edited, err := client.EditPullRequest(t.Context(), gitlabRepo(),
		forge.PullRequest{Number: 43}, forge.PullRequestEdit{Title: editTitle, Body: editBody})

	// Assert
	if err != nil || edited.Title != editTitle {
		t.Fatalf("EditPullRequest = %+v, %v", edited, err)
	}

	request := requestTo(*seen, gitlabMergesPath+"/43")
	if request.method != http.MethodPut || request.body["title"] != editTitle ||
		request.body["description"] != editBody {
		t.Errorf("edit request = %+v, want a PUT of the title and description", request)
	}
}

func TestEditPullRequestReportsARefusal(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo forge.Repo
		path string
	}{
		github: {repo: githubRepo(), path: githubPullsPath + "/43"},
		gitlab: {repo: gitlabRepo(), path: gitlabMergesPath + "/43"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeConversation(t, nil, map[string]bool{tt.path: true})

			// Act
			_, err := client.EditPullRequest(t.Context(), tt.repo,
				forge.PullRequest{Number: 43}, forge.PullRequestEdit{Title: editTitle})

			// Assert
			if !errors.Is(err, forge.ErrRefused) {
				t.Errorf("EditPullRequest error = %v, want ErrRefused", err)
			}
		})
	}
}

func TestEditPullRequestOnAnUnknownForgeIsUnknown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := forgeConversation(t, nil, nil)

	// Act
	_, err := client.EditPullRequest(t.Context(), unknownForge(),
		forge.PullRequest{Number: 1}, forge.PullRequestEdit{Title: editTitle})

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) {
		t.Errorf("EditPullRequest error = %v, want ErrUnknownForge", err)
	}
}

func TestFindPullRequestReadsTheBody(t *testing.T) {
	t.Parallel()

	// Arrange
	// The body is what an edit composer opens on, so a find must read it.
	client, _ := forgeRouting(t, map[string]string{
		githubPullsPath: `[{"number":9,"html_url":"https://x/9","title":"t","body":"the description"}]`,
	})

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || !found || pull.Body != "the description" {
		t.Errorf("FindPullRequest body = %q (found %v, err %v), want it read", pull.Body, found, err)
	}
}
