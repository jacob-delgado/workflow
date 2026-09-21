// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// errNothingToOpen refuses opening a pull request when there is nothing to open
// one from — no branch, no commits, or one is already open.
var errNothingToOpen = errors.New("there is nothing to open a pull request for")

// GetPullRequestDraft composes the pull request that would be opened for the
// checked-out branch, without opening it, so the browser can edit it before
// confirming. It is a 409 when there is nothing to open.
func (s *server) GetPullRequestDraft(
	_ context.Context, _ api.GetPullRequestDraftRequestObject,
) (api.GetPullRequestDraftResponseObject, error) {
	draft, branch, ok := s.composePullRequest()
	if !ok {
		return api.GetPullRequestDraft409JSONResponse{Code: api.Conflict, Message: errNothingToOpen.Error()}, nil
	}

	return api.GetPullRequestDraft200JSONResponse(draftDTO(draft, branch)), nil
}

// OpenPullRequest opens a pull request from the checked-out branch with the
// given title, body, base, and draft flag, pushing the branch first when it is
// not yet published. It is a 409 when there is nothing to open, and a 422 when
// the request is incomplete or the push or the open fails.
func (s *server) OpenPullRequest(
	_ context.Context, request api.OpenPullRequestRequestObject,
) (api.OpenPullRequestResponseObject, error) {
	if request.Body == nil {
		return openUnprocessable("a request body is required"), nil
	}

	if !s.canOpenPull() {
		return openUnprocessable("opening a pull request is not available"), nil
	}

	_, branch, ok := s.composePullRequest()
	if !ok {
		return api.OpenPullRequest409JSONResponse{Code: api.Conflict, Message: errNothingToOpen.Error()}, nil
	}

	newPull, ok := pullFromRequest(*request.Body, branch)
	if !ok {
		return openUnprocessable("a title and a base branch are required"), nil
	}

	err := s.ensurePushed(branch)
	if err != nil {
		return openUnprocessable(err.Error()), nil
	}

	pull, err := s.deps.CreatePull(newPull)
	if err != nil && !pull.Opened() {
		return openUnprocessable(err.Error()), nil
	}

	// A pull that opened but whose reviewers, assignees or labels could not all
	// be added is reported open, with a warning, rather than lost to a failure.
	opened := api.OpenedPullRequest{Pull: pullDTO(pull)}

	if err != nil {
		warning := "the pull request opened, but its reviewers, assignees or labels could not all be added: " +
			err.Error()
		opened.Warning = &warning
	}

	return api.OpenPullRequest200JSONResponse(opened), nil
}

// canOpenPull reports whether the seams the open needs are wired: creating the
// pull request, reading the branch, and pushing it first when it is not yet up.
func (s *server) canOpenPull() bool {
	return s.deps.CreatePull != nil && s.deps.Branch != nil && s.deps.Push != nil
}

// composePullRequest builds the pull request to propose for the checked-out
// branch from its commits, the branch's issue, and the repository's template.
// It reports false when there is nothing to open: the tree is not on a branch,
// the branch has no commits, or a pull request is already open for it.
func (s *server) composePullRequest() (forge.NewPullRequest, gitrepo.Branch, bool) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, false
	}

	branch, err := s.deps.Branch()
	if err != nil || branch.Name == "" || len(branch.Commits) == 0 {
		return forge.NewPullRequest{}, gitrepo.Branch{}, false
	}

	_, found, err := s.deps.FindPull(branch.Name)
	if err != nil || found {
		return forge.NewPullRequest{}, gitrepo.Branch{}, false
	}

	return s.draftFor(branch), branch, true
}

// draftFor composes the pull request for branch: a title and body from its
// commits, the issue, and the repository's first template, and the base branch
// it would merge into.
func (s *server) draftFor(branch gitrepo.Branch) forge.NewPullRequest {
	subjects := commitSubjects(branch.Commits)
	key, _ := convention.IssueKey(branch.Name, s.config().Jira.Project)
	issueKey := jira.Key(key)

	return forge.NewPullRequest{
		Title: convention.PullRequestTitle(subjects, key, s.issueSummary(issueKey)),
		Body:  convention.PullRequestBody(s.template(), subjects, key, s.issueURL(issueKey)),
		Head:  branch.Name,
		Base:  branch.BaseName(),
		Draft: false,
	}
}

// commitSubjects is the subject line of each commit, oldest first, for the title
// and body proposals.
func commitSubjects(commits []gitrepo.Commit) []string {
	subjects := make([]string, 0, len(commits))
	for _, commit := range commits {
		subjects = append(subjects, commit.Subject)
	}

	return subjects
}

// template is the first repository pull request template's body, or empty when
// there is none — the body then falls back to the branch's commits.
func (s *server) template() string {
	if s.deps.Templates == nil {
		return ""
	}

	templates := s.deps.Templates()
	if len(templates) == 0 {
		return ""
	}

	return templates[0].Body
}

// ensurePushed publishes the branch when it is not yet on its remote, since a
// pull request cannot open from an unpushed branch. A pushed branch is a no-op.
func (s *server) ensurePushed(branch gitrepo.Branch) error {
	if branch.Pushed() {
		return nil
	}

	return s.pushBranch(branch.Name)
}

// pullFromRequest builds the pull request to open from the request body and the
// checked-out branch, whose name is always the head — the caller does not choose
// it. It reports false when the title or the base is missing.
func pullFromRequest(body api.OpenPullRequestRequest, branch gitrepo.Branch) (forge.NewPullRequest, bool) {
	title := strings.TrimSpace(body.Title)
	base := strings.TrimSpace(body.Base)

	if title == "" || base == "" {
		return forge.NewPullRequest{}, false
	}

	return forge.NewPullRequest{
		Title:     title,
		Body:      orZero(body.Body),
		Head:      branch.Name,
		Base:      base,
		Draft:     orZero(body.Draft),
		Reviewers: trimmedList(body.Reviewers),
		Assignees: trimmedList(body.Assignees),
		Labels:    trimmedList(body.Labels),
	}, true
}

// trimmedList reads an optional list of names into its trimmed, non-empty
// entries, so a stray comma in the web form never sends the forge a blank
// reviewer, assignee or label.
func trimmedList(list *[]string) []string {
	var cleaned []string

	for _, entry := range orZero(list) {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	return cleaned
}

// draftDTO maps the composed draft and its branch onto the wire.
func draftDTO(draft forge.NewPullRequest, branch gitrepo.Branch) api.PullRequestDraft {
	return api.PullRequestDraft{
		Title:     draft.Title,
		Body:      draft.Body,
		Base:      draft.Base,
		Head:      draft.Head,
		Draft:     draft.Draft,
		NeedsPush: !branch.Pushed(),
	}
}

// openUnprocessable is the 422 response for a pull request the server will not
// open.
func openUnprocessable(message string) api.OpenPullRequest422JSONResponse {
	return api.OpenPullRequest422JSONResponse{Code: api.Unprocessable, Message: message}
}
