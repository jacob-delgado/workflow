// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// unnamedBranch is a branch begun outside workflow, named for no issue.
const unnamedBranch = "my-thing"

// reviewOn is filledDeps on a branch begun outside workflow, linked to link,
// with a pull request whose description is body.
func reviewOn(link, body string) webserver.Deps {
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: unnamedBranch, IssueLink: link}, nil
	}
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, Title: "Speed up search", Body: body}, true, nil
	}
	deps.BrowseURL = func(key jira.Key) string { return "https://tracker.example/" + string(key) }

	return deps
}

func TestTheReviewNamesTheIssueTheBranchWasLinkedTo(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("PROJ-7", ""), config.Default()), "/api/review"))

	// Assert
	issue := review.Issue
	if issue == nil || issue.Key != "PROJ-7" || issue.Tracker != api.Jira ||
		issue.URL != "https://tracker.example/PROJ-7" || issue.Origin != api.LinkedIssueOriginByHand {
		t.Errorf("issue = %+v, want PROJ-7 from the link, with its page", issue)
	}
}

func TestTheReviewFindsTheIssueThePullRequestNames(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("", "Speeds it up.\n\nCloses #42"), config.Default()),
		"/api/review"))

	// Assert
	issue := review.Issue
	if issue == nil || issue.Key != "42" || issue.Tracker != api.Forge ||
		issue.Origin != api.LinkedIssueOriginPullRequest {
		t.Errorf("issue = %+v, want the forge's 42 from the pull request", issue)
	}
}

func TestAReviewWithNoIssueNamesNone(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("", "Speeds it up."), config.Default()), "/api/review"))

	// Assert
	if review.Issue != nil {
		t.Errorf("issue = %+v, want none", review.Issue)
	}
}
