// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"cmp"
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

// linking records what the link endpoints asked of git and the forge: each
// description rewritten, and each edit of a title and description whole. held
// is the description as the forge holds it, empty for the one shown.
type linking struct {
	linked, unlinked []string
	edited           []string
	editedWhole      []string
	held             string
}

// linkingDeps is filledDeps on a branch begun outside workflow, with an open
// pull request whose description names no issue, recording each link and
// edit.
func linkingDeps(record *linking, detached bool) webserver.Deps {
	deps := filledDeps()
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: unnamedBranch, Detached: detached}, nil
	}
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, Title: "Speed up search", Body: "Speeds it up.\n"}, true, nil
	}
	deps.Git.LinkIssue = func(branch, issueKey string) error {
		record.linked = append(record.linked, branch+" "+issueKey)

		return nil
	}
	deps.Git.UnlinkIssue = func(branch string) error {
		record.unlinked = append(record.unlinked, branch)

		return nil
	}
	deps.Forge.EditPullRequest = func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
		record.editedWhole = append(record.editedWhole, edit.Body)

		return pull, nil
	}
	deps.Forge.RewriteDescription = func(pull forge.PullRequest, rewrite func(string) (string, bool)) (bool, error) {
		body, changed := rewrite(cmp.Or(record.held, pull.Body))
		if changed {
			record.edited = append(record.edited, body)
		}

		return changed, nil
	}

	return deps
}

// linkWrite is a request to link the branch to an issue, or to forget its link.
type linkWrite struct {
	method, body string
}

// linkAndUnlink are a link to PROJ-7 that leaves the pull request, and an
// unlink, by name.
func linkAndUnlink() map[string]linkWrite {
	return map[string]linkWrite{
		"linking":   {method: http.MethodPut, body: `{"key":"PROJ-7","update_pull":false}`},
		"unlinking": {method: http.MethodDelete},
	}
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

func TestLinkingAddsTheIssueLineToTheDescriptionTheForgeHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// What is shown of a description has its controls and bidirectional marks
	// replaced and its carriage returns dropped; the forge holds it as written.
	record := &linking{held: "Speeds it up.\r\n\u202Eright to left\u202C\r\n"}
	handler := serve(t, linkingDeps(record, false), config.Default())

	// Act
	answer := send(t, handler, http.MethodPut, "/api/branch/issue", `{"key":"#42","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusOK || len(record.edited) != 1 ||
		record.edited[0] != "Speeds it up.\r\n\u202Eright to left\u202C\r\n\nCloses #42\n" {
		t.Errorf("status %d, rewrote %q; want the closing line added to the description as written",
			answer.Code, record.edited)
	}

	if len(record.editedWhole) != 0 {
		t.Errorf("edited %q whole, want the title and the shown description left alone", record.editedWhole)
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

func TestLinkingOrUnlinkingADetachedHeadIsAConflict(t *testing.T) {
	t.Parallel()

	for name, tt := range linkAndUnlink() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := &linking{}
			handler := serve(t, linkingDeps(record, true), config.Default())

			// Act
			answer := send(t, handler, tt.method, "/api/branch/issue", tt.body)

			// Assert
			if answer.Code != http.StatusConflict || len(record.linked)+len(record.unlinked) != 0 {
				t.Errorf("status %d, linked %v, unlinked %v; want 409 with no branch to link",
					answer.Code, record.linked, record.unlinked)
			}
		})
	}
}

func TestLinkingOrUnlinkingABranchGitCouldNotReadIsInternal(t *testing.T) {
	t.Parallel()

	for name, tt := range linkAndUnlink() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := &linking{}
			deps := linkingDeps(record, false)
			deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

			// Act
			answer := send(t, serve(t, deps, config.Default()), tt.method, "/api/branch/issue", tt.body)

			// Assert
			failure := decode[api.Problem](t, answer)
			if answer.Code != http.StatusInternalServerError || failure.Code != api.ProblemCodeInternal ||
				len(record.linked)+len(record.unlinked) != 0 {
				t.Errorf("status/code %d/%s, linked %v, unlinked %v; want 500/internal and nothing changed",
					answer.Code, failure.Code, record.linked, record.unlinked)
			}
		})
	}
}

func TestThePreviewOfADetachedHeadNamesNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, linkingDeps(&linking{}, true), config.Default())

	// Act
	recorder := get(t, handler, "/api/branch/issue/preview?key=%2342")

	// Assert
	preview := decode[api.BranchIssuePreview](t, recorder)
	if recorder.Code != http.StatusOK || preview.Pull != 0 || preview.Body != "" || preview.Key != "42" {
		t.Errorf("status %d, preview %+v; want 200 naming 42 and no pull request", recorder.Code, preview)
	}
}

func TestLinkingWhenThePullRequestCannotBeFoundLeavesTheBranchUnlinked(t *testing.T) {
	t.Parallel()

	// Arrange
	record := &linking{}
	deps := linkingDeps(record, false)
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)
	}

	// Act
	answer := send(t, serve(t, deps, config.Default()), http.MethodPut, "/api/branch/issue",
		`{"key":"PROJ-7","update_pull":true}`)

	// Assert
	if answer.Code != http.StatusBadGateway || len(record.linked)+len(record.edited) != 0 {
		t.Errorf("status %d, linked %v, edited %v; want 502 and nothing changed",
			answer.Code, record.linked, record.edited)
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
	deps.Git.Branch = func() (gitrepo.Branch, error) {
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

	for name, tt := range linkAndUnlink() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := linkingDeps(&linking{}, false)
			refused := fmt.Errorf("%w: exit status 4", gitrepo.ErrIssueLinkNotSaved)
			deps.Git.LinkIssue = func(string, string) error { return refused }
			deps.Git.UnlinkIssue = func(string) error { return refused }

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
	deps.Forge.RewriteDescription = func(forge.PullRequest, func(string) (string, bool)) (bool, error) {
		return false, fmt.Errorf("editing #9: %w", forge.ErrUnreachable)
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
	deps.Git.LinkIssue = func(string, string) error { return gitrepo.ErrIssueLinkNotSaved }

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

	for name, tt := range linkAndUnlink() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := linkingDeps(&linking{}, false)
			deps.Git.LinkIssue, deps.Git.UnlinkIssue = nil, nil

			// Act
			answer := send(t, serve(t, deps, config.Default()), tt.method, "/api/branch/issue", tt.body)

			// Assert
			if answer.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d: %s; want 422, linking not available", answer.Code, answer.Body.String())
			}
		})
	}
}
