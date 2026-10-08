// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// linking records what the link endpoints asked of git and the forge.
type linking struct {
	linked, unlinked []string
	edited           []string
}

// linkingDeps is filledDeps on a branch begun outside workflow, with an open
// pull request whose description names no issue, recording each link and
// edit.
func linkingDeps(record *linking, detached bool) webserver.Deps {
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: unnamedBranch, Detached: detached}, nil
	}
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, Title: "Speed up search", Body: "Speeds it up.\n"}, true, nil
	}
	deps.LinkIssue = func(branch, issueKey string) error {
		record.linked = append(record.linked, branch+" "+issueKey)

		return nil
	}
	deps.UnlinkIssue = func(branch string) error {
		record.unlinked = append(record.unlinked, branch)

		return nil
	}
	deps.EditPull = func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
		record.edited = append(record.edited, edit.Body)

		return pull, nil
	}

	return deps
}

func TestLinkingTheBranchKeepsTheLinkAndUpdatesThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	handler := serve(t, linkingDeps(record, false), config.Default())

	// Act
	answer := send(t, handler, http.MethodPut, "/api/branch/issue", `{"key":"#42","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusOK || len(record.linked) != 1 || record.linked[0] != "my-thing 42" {
		t.Fatalf("status %d, linked %v: %s; want my-thing linked to 42", answer.Code, record.linked, answer.Body.String())
	}

	if len(record.edited) != 1 || record.edited[0] != "Speeds it up.\n\nCloses #42\n" {
		t.Errorf("edited %q, want the closing line added to #9's description", record.edited)
	}
}

func TestLinkingWithoutUpdatingLeavesThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	handler := serve(t, linkingDeps(record, false), config.Default())

	// Act
	answer := send(t, handler, http.MethodPut, "/api/branch/issue", `{"key":"PROJ-7","update_pull":false}`)

	// Assert
	if answer.Code != http.StatusOK || len(record.linked) != 1 || len(record.edited) != 0 {
		t.Errorf("status %d, linked %v, edited %v; want the link alone", answer.Code, record.linked, record.edited)
	}
}

func TestLinkingRefusesAKeyThatNamesNoIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	handler := serve(t, linkingDeps(record, false), config.Default())

	// Act
	answer := send(t, handler, http.MethodPut, "/api/branch/issue", `{"key":"soon","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusUnprocessableEntity || len(record.linked) != 0 {
		t.Errorf("status %d, linked %v; want 422 and nothing linked", answer.Code, record.linked)
	}
}

func TestLinkingADetachedHeadIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	handler := serve(t, linkingDeps(record, true), config.Default())

	// Act
	answer := send(t, handler, http.MethodPut, "/api/branch/issue", `{"key":"PROJ-7","update_pull":false}`)

	// Assert
	if answer.Code != http.StatusConflict || len(record.linked) != 0 {
		t.Errorf("status %d, linked %v; want 409 with no branch to link", answer.Code, record.linked)
	}
}

func TestThePreviewShowsTheDescriptionWithTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, linkingDeps(&linking{}, false), config.Default())

	// Act
	preview := decode[api.BranchIssuePreview](t, get(t, handler, "/api/branch/issue/preview?key=%2342"))

	// Assert
	if preview.Pull != 9 || !preview.Changes || !strings.HasSuffix(preview.Body, "Closes #42\n") || preview.Key != "42" {
		t.Errorf("preview = %+v, want #9's description with Closes #42 added", preview)
	}
}

func TestThePreviewAnswersABranchGitCouldNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := linkingDeps(&linking{}, false)
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{}, fmt.Errorf("reading HEAD in %s: %w", repoPath, errSeam)
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/branch/issue/preview?key=%2342")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusInternalServerError || failure.Code != api.ProblemCodeInternal {
		t.Errorf("status/code = %d/%s, want 500/internal rather than no pull request to update",
			recorder.Code, failure.Code)
	}
}

func TestUnlinkingForgetsTheLink(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	handler := serve(t, linkingDeps(record, false), config.Default())

	// Act
	answer := send(t, handler, http.MethodDelete, "/api/branch/issue", "")

	// Assert
	if answer.Code != http.StatusOK || len(record.unlinked) != 1 || record.unlinked[0] != "my-thing" {
		t.Errorf("status %d, unlinked %v; want my-thing's link forgotten", answer.Code, record.unlinked)
	}
}

func TestALinkGitCannotKeepIsUnprocessable(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, body string
	}{
		"linking":   {method: http.MethodPut, body: `{"key":"PROJ-7","update_pull":false}`},
		"unlinking": {method: http.MethodDelete},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := linkingDeps(&linking{}, false)
			refused := fmt.Errorf("%w: exit status 4", gitrepo.ErrIssueLinkNotSaved)
			deps.LinkIssue = func(string, string) error { return refused }
			deps.UnlinkIssue = func(string) error { return refused }

			// Act
			answer := send(t, serve(t, deps, config.Default()), tt.method, "/api/branch/issue", tt.body)

			// Assert
			if answer.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d: %s; want 422", answer.Code, answer.Body.String())
			}
		})
	}
}

func TestAPullRequestThatCannotBeEditedLeavesTheBranchUnlinked(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	deps := linkingDeps(record, false)
	deps.EditPull = func(forge.PullRequest, forge.PullRequestEdit) (forge.PullRequest, error) {
		return forge.PullRequest{}, fmt.Errorf("editing #9: %w", forge.ErrUnreachable)
	}

	// Act
	answer := send(t, serve(t, deps, config.Default()), http.MethodPut, "/api/branch/issue",
		`{"key":"PROJ-7","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusBadGateway || len(record.linked) != 0 {
		t.Errorf("status %d, linked %v; want 502 and the branch left unlinked", answer.Code, record.linked)
	}
}

func TestALinkNotKeptAfterThePullRequestIsEditedIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	deps := linkingDeps(record, false)
	deps.LinkIssue = func(string, string) error { return gitrepo.ErrIssueLinkNotSaved }

	// Act
	answer := send(t, serve(t, deps, config.Default()), http.MethodPut, "/api/branch/issue",
		`{"key":"PROJ-7","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusUnprocessableEntity || len(record.edited) != 1 {
		t.Errorf("status %d, edited %v; want 422 after the one edit", answer.Code, record.edited)
	}
}

func TestLinkingAndUnlinkingWithoutTheSeamAnswerAlike(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, body string
	}{
		"linking":   {method: http.MethodPut, body: `{"key":"PROJ-7","update_pull":false}`},
		"unlinking": {method: http.MethodDelete},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := linkingDeps(&linking{}, false)
			deps.LinkIssue, deps.UnlinkIssue = nil, nil

			// Act
			answer := send(t, serve(t, deps, config.Default()), tt.method, "/api/branch/issue", tt.body)

			// Assert
			if answer.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d: %s; want 422, linking not available", answer.Code, answer.Body.String())
			}
		})
	}
}
