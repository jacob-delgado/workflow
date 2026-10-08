// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestTheBranchNamesTheTrackerItsLinkedIssueLivesIn(t *testing.T) {
	t.Parallel()

	forgeTracker, jiraTracker := api.IssueTrackerForge, api.IssueTrackerJira

	// The page shows a forge issue's number after a #, and keeps a Jira issue
	// selected across a switch of directory, by the tracker the server reads
	// from the key, so it holds no rule of its own for the key's shape.
	cases := map[string]struct {
		link string
		want *api.IssueTracker
	}{
		"a forge issue":   {link: "42", want: &forgeTracker},
		"a Jira issue":    {link: "PROJ-412", want: &jiraTracker},
		"no linked issue": {link: "", want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{Name: unnamedBranch, IssueLink: tt.link}, nil
			}

			// Act
			answer := send(t, serve(t, deps, config.Default()), http.MethodGet, "/api/branch", "")

			// Assert
			got := decode[api.Branch](t, answer).IssueLinkTracker
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("issue_link_tracker = %v, want %v", got, tt.want)
			}
		})
	}
}
