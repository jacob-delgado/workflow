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
	"github.com/jacob-delgado/workflow/internal/loop"
)

// GetPullRequestDraft composes the pull request that would be opened for the
// checked-out branch, without opening it, so the browser can edit it before
// confirming. It is a 409 when there is nothing to open; a branch that cannot
// be read is classified by fault.
func (s *server) GetPullRequestDraft(
	_ context.Context, _ api.GetPullRequestDraftRequestObject,
) (api.GetPullRequestDraftResponseObject, error) {
	draft, branch, err := s.composePullRequest()
	if err != nil {
		return s.draftRefusal(err), nil
	}

	return api.GetPullRequestDraft200JSONResponse(draftDTO(draft, branch)), nil
}

// OpenPullRequest opens a pull request from the checked-out branch with the
// given title, body, base, and draft flag, pushing the branch first when it is
// not yet published. It is a 409 when there is nothing to open, a branch that
// cannot be read is classified by fault, and it is a 422 when the request is
// incomplete or the push or the open fails.
func (s *server) OpenPullRequest(
	_ context.Context, request api.OpenPullRequestRequestObject,
) (api.OpenPullRequestResponseObject, error) {
	if request.Body == nil {
		return openUnprocessable("a request body is required"), nil
	}

	if !s.canOpenPull() {
		return openUnprocessable("opening a " + s.noun() + " is not available"), nil
	}

	_, branch, err := s.composePullRequest()
	if err != nil {
		return s.openRefusal(err), nil
	}

	newPull, ok := pullFromRequest(*request.Body, branch)
	if !ok {
		return openUnprocessable("a title and a base branch are required"), nil
	}

	err = loop.EnsurePushed(s.deps.Push, branch)
	if err != nil {
		return openUnprocessable(pushFailure(err)), nil
	}

	pull, err := s.deps.CreatePull(newPull)
	if err != nil && !pull.Opened() {
		return openFailure(err), nil
	}

	// A pull that opened but whose reviewers, assignees or labels could not all
	// be added is reported open, with a warning, rather than lost to a failure.
	opened := api.OpenedPullRequest{Pull: pullDTO(pull), FollowUps: s.followUps(branch)}

	if err != nil {
		warning := "the " + s.noun() + " opened, but its reviewers, assignees or labels could not all be added"
		opened.Warning = &warning
	}

	return api.OpenPullRequest200JSONResponse(opened), nil
}

// noun is what the forge calls a proposed change — a merge request on GitLab —
// so what the page is told names it as the page itself does.
func (s *server) noun() string {
	return s.info.ForgeKind.Noun()
}

// nothingToOpen is the 409 for a composition the loop refused because there is
// nothing to open — no branch, no commits, or one is already open — and false
// for any other failure, which is a read that fault classifies.
func (s *server) nothingToOpen(err error) (api.Problem, bool) {
	if !errors.Is(err, loop.ErrNothingToOpen) && !errors.Is(err, loop.ErrPullAlreadyOpen) {
		return api.Problem{}, false
	}

	return problem(api.Conflict, "there is nothing to open a "+s.noun()+" for"), true
}

// draftRefusal answers a draft that could not be composed.
func (s *server) draftRefusal(err error) api.GetPullRequestDraftResponseObject {
	if conflict, ok := s.nothingToOpen(err); ok {
		return api.GetPullRequestDraft409ApplicationProblemPlusJSONResponse(conflict)
	}

	body, code := fault(err)

	return api.GetPullRequestDraftdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// openRefusal answers an open whose pull request could not be composed.
func (s *server) openRefusal(err error) api.OpenPullRequestResponseObject {
	if conflict, ok := s.nothingToOpen(err); ok {
		return api.OpenPullRequest409ApplicationProblemPlusJSONResponse(conflict)
	}

	body, code := fault(err)

	return api.OpenPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// canOpenPull reports whether the seams the open needs are wired: creating the
// pull request, reading the branch, and pushing it first when it is not yet up.
func (s *server) canOpenPull() bool {
	return s.deps.CreatePull != nil && s.deps.Branch != nil && s.deps.Push != nil
}

// composePullRequest builds the pull request to propose for the checked-out
// branch from its commits, the branch's issue, and the repository's template.
// It fails with the loop's refusal when there is nothing to open — the tree is
// not on a branch, the branch has no commits, or a pull request is already open
// for it — and with the read's own error when the branch cannot be read.
func (s *server) composePullRequest() (forge.NewPullRequest, gitrepo.Branch, error) {
	cfg := s.config()

	draft, branch, err := loop.ComposePull(loop.PullSeams{
		Branch:    s.deps.Branch,
		FindPull:  s.deps.FindPull,
		Templates: s.deps.Templates,
		Issue:     s.deps.Issue,
		BrowseURL: s.deps.BrowseURL,
	}, loop.PullOptions{
		Project:     cfg.Jira.Project,
		TitleSource: convention.TitleSource(cfg.PullRequest.TitleSource),
	})

	return draft, branch, err
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
func openUnprocessable(message string) api.OpenPullRequest422ApplicationProblemPlusJSONResponse {
	return api.OpenPullRequest422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}

// openFailure answers an open the forge did not make. Two failures carry a host
// in their error and never reach the detail: an unreachable forge, which goes
// through the curated fault mapping (a 502), and a remote whose forge cannot be
// told apart, which the wiring words with its host so a terminal can say which
// one. A forge rejection carries its own reason, which is safe and useful to show.
func openFailure(err error) api.OpenPullRequestResponseObject {
	switch {
	case errors.Is(err, forge.ErrUnreachable):
		body, code := fault(err)

		return api.OpenPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	case errors.Is(err, forge.ErrUnknownForge):
		return openUnprocessable("cannot tell which forge this repository is on; set forge.kind and forge.host")
	default:
		return openUnprocessable(err.Error())
	}
}
