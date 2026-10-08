// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

// A read the forge refuses is reported as refused, on either forge, rather
// than read as an empty answer: an issue that cannot be read, a listing of
// merge requests, or GitLab's assigned issues once it has said who is asking.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestReadIssueReportsAnIssueTheForgeRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		number int
		path   string
	}{
		"on GitHub": {repo: githubRepo(), number: 42, path: githubIssuePath},
		"on GitLab": {repo: gitlabRepo(), number: 7, path: gitlabIssuePath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, conversation(nil, map[string]bool{tt.path: true}))

			// Act
			detail, err := client.ReadIssue(t.Context(), tt.repo, tt.number)

			// Assert
			if !errors.Is(err, forge.ErrRefused) || detail != (forge.IssueDetail{}) {
				t.Errorf("ReadIssue = %+v, %v; want nothing read and ErrRefused", detail, err)
			}
		})
	}
}

func TestFindPullRequestReportsAListingTheForgeRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo forge.Repo
		path string
	}{
		"on GitHub": {repo: githubRepo(), path: githubPullsPath},
		"on GitLab": {repo: gitlabRepo(), path: gitlabMergesPath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, conversation(nil, map[string]bool{tt.path: true}))

			// Act
			_, found, err := client.FindPullRequest(t.Context(), tt.repo, featureBranch)

			// Assert
			if !errors.Is(err, forge.ErrRefused) || found {
				t.Errorf("FindPullRequest = %v, %v; want ErrRefused, not a branch with nothing open", found, err)
			}
		})
	}
}

func TestAssignedIssuesOnGitLabReportsAListRefusedAfterTheViewerIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab says who is asking, then will not list that person's issues.
	client, seen := recordingForge(t, conversation(
		map[string]string{gitlabUserPath: gitlabWhoami},
		map[string]bool{gitlabIssuesList: true}))

	// Act
	issues, err := client.AssignedIssues(t.Context(), gitlabRepo())

	// Assert
	if !errors.Is(err, forge.ErrRefused) || issues != nil {
		t.Errorf("AssignedIssues = %+v, %v; want nothing listed and ErrRefused", issues, err)
	}

	if requestTo(*seen, gitlabIssuesList).method == "" {
		t.Error("the issue list was never asked for, so its refusal was not what was reported")
	}
}
