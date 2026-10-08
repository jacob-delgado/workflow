// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// The person and the team that own the changes.
const (
	ownedUser = "ana"
	ownedTeam = "acme/control-plane"
)

func TestGetPullRequestDraftProposesTheCodeOwnersAsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	var read []string

	deps := openableDeps()
	deps.Git.ChangedPaths = func(base string) ([]string, error) {
		read = append(read, base)

		return []string{"api/pull.go"}, nil
	}
	deps.Git.CodeOwnersAt = func(base string) (codeowners.File, bool, error) {
		read = append(read, base)

		return codeowners.Parse("* @"+ownedUser+" @Me @"+ownedTeam, codeowners.GitHub), true, nil
	}
	deps.Forge.Author = func() (string, error) { return "me", nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	draft := decode[api.PullRequestDraft](t, recorder)
	if want := []string{ownedUser, ownedTeam}; !slices.Equal(draft.Reviewers, want) {
		t.Errorf("reviewers = %q, want %q", draft.Reviewers, want)
	}

	if !slices.Equal(read, []string{prBase, prBase}) {
		t.Errorf("owners read at %q, want the base twice", read)
	}
}

func TestGetPullRequestDraftProposesNoReviewersWithoutCodeOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	// The field is required, so nobody is an empty list rather than null.
	if body := recorder.Body.String(); !strings.Contains(body, `"reviewers":[]`) {
		t.Errorf("draft = %s, want an empty reviewers list", body)
	}
}

func TestOpenPullRequestRequestsTeamsAsTeams(t *testing.T) {
	t.Parallel()

	// Arrange
	var request forge.NewPullRequest

	deps := openableDeps()
	deps.Forge.CreatePullRequest = func(newPull forge.NewPullRequest) (forge.PullRequest, error) {
		request = newPull

		return forge.PullRequest{Number: 7, URL: prURL, Title: newPull.Title}, nil
	}

	body := `{"title":"` + prTitle + `","base":"` + prBase + `","reviewers":["` + ownedUser + `"," ` + ownedTeam + ` "]}`

	// Act
	recorder := doOpen(t, deps, body)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !slices.Equal(request.Reviewers, []string{ownedUser}) || !slices.Equal(request.TeamReviewers, []string{ownedTeam}) {
		t.Errorf("reviewers = %q, teams = %q; want ana and %s", request.Reviewers, request.TeamReviewers, ownedTeam)
	}
}
