// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strconv"
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
	_ context.Context, request api.GetPullRequestDraftRequestObject,
) (api.GetPullRequestDraftResponseObject, error) {
	template := orZero(request.Params.Template)

	draft, branch, err := s.composePullRequest(template)
	if errors.Is(err, loop.ErrNoSuchTemplate) {
		return api.GetPullRequestDraft404ApplicationProblemPlusJSONResponse(problem(api.NotFound,
			"the repository has no pull request template of that name")), nil
	}

	if err != nil {
		return s.draftRefusal(err), nil
	}

	answer := draftDTO(draft, branch)
	answer.Templates, answer.Template = s.templateNames(template)

	return api.GetPullRequestDraft200JSONResponse(answer), nil
}

// templateNames is the repository's pull request templates by name, and the
// one a draft asked to start from chosen starts from: the first, when it
// asked for none.
func (s *server) templateNames(chosen string) ([]string, string) {
	names := []string{}

	if s.deps.Templates != nil {
		for _, template := range s.deps.Templates() {
			names = append(names, template.Name)
		}
	}

	if chosen == "" && len(names) > 0 {
		chosen = names[0]
	}

	return names, chosen
}

// OpenPullRequest opens a pull request from the checked-out branch with the
// given title, body, base, and draft flag, pushing the branch first when it is
// not yet published. It is a 409 when there is nothing to open, and a 422 when
// the request is incomplete, the push fails, or the forge turns the pull
// request down; a branch that cannot be read, and any other failed open, is
// classified by fault.
func (s *server) OpenPullRequest(
	_ context.Context, request api.OpenPullRequestRequestObject,
) (api.OpenPullRequestResponseObject, error) {
	if !s.canOpenPull() {
		return openUnprocessable("opening a " + s.noun() + " is not available"), nil
	}

	_, branch, err := s.composePullRequest("")
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
		return s.openFailure(err), nil
	}

	// The stream's held forge answer predates this pull request: the page
	// shows it from the next frame, not an interval later.
	s.forgeAnswer.drop()

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
	return s.forgeKindNow().Noun()
}

// openConflict is the 409 for a composition the loop refused, worded by its
// cause — the pull request already open, named by its number the way the forge
// marks it, or no branch with commits to propose — and false for any other
// failure, which is a read that fault classifies. The open one is not named by
// its address, which carries the forge's host.
func (s *server) openConflict(err error) (api.Problem, bool) {
	if open, ok := errors.AsType[loop.PullAlreadyOpenError](err); ok {
		number := s.forgeKindNow().Sigil() + strconv.Itoa(open.Pull.Number)

		return problem(api.Conflict, number+" is already open for this branch"), true
	}

	if errors.Is(err, loop.ErrNothingToOpen) {
		return problem(api.Conflict, "there is no branch with commits to open a "+s.noun()+" for"), true
	}

	return api.Problem{}, false
}

// draftRefusal answers a draft that could not be composed.
func (s *server) draftRefusal(err error) api.GetPullRequestDraftResponseObject {
	if conflict, ok := s.openConflict(err); ok {
		return api.GetPullRequestDraft409ApplicationProblemPlusJSONResponse(conflict)
	}

	body, code := s.fault(err)

	return api.GetPullRequestDraftdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// openRefusal answers an open whose pull request could not be composed.
func (s *server) openRefusal(err error) api.OpenPullRequestResponseObject {
	if conflict, ok := s.openConflict(err); ok {
		return api.OpenPullRequest409ApplicationProblemPlusJSONResponse(conflict)
	}

	body, code := s.fault(err)

	return api.OpenPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// canOpenPull reports whether the seams the open needs are wired: creating the
// pull request, reading the branch, and pushing it first when it is not yet up.
func (s *server) canOpenPull() bool {
	return s.deps.CreatePull != nil && s.deps.Branch != nil && s.deps.Push != nil
}

// composePullRequest builds the pull request to propose for the checked-out
// branch from its commits, the branch's issue, the repository's template, and
// the code owners of its changes.
// It fails with the loop's refusal when there is nothing to open — the tree is
// not on a branch, the branch has no commits, or a pull request is already open
// for it — and with the read's own error when the branch cannot be read.
func (s *server) composePullRequest(template string) (forge.NewPullRequest, gitrepo.Branch, error) {
	cfg := s.config()

	draft, branch, err := loop.ComposePull(loop.PullSeams{
		Branch:    s.deps.Branch,
		FindPull:  s.deps.FindPull,
		Templates: s.deps.Templates,
		Issue:     s.deps.Issue,
		BrowseURL: s.deps.BrowseURL,
		Owners:    s.ownerSeams(),
	}, loop.PullOptions{
		Project:     cfg.Jira.Project,
		TitleSource: convention.TitleSource(cfg.PullRequest.TitleSource),
		Template:    template,
	})

	return draft, branch, err
}

// ownerSeams read the code owners of the branch's changes, leaving out the
// author the server already knows once it has asked.
func (s *server) ownerSeams() loop.OwnerSeams {
	owners := loop.OwnerSeams{
		ChangedPaths: s.deps.ChangedPaths, CodeOwnersAt: s.deps.CodeOwnersAt, Author: nil, IsGroup: nil,
	}
	if s.deps.Author != nil {
		owners.Author = s.cachedAuthor
	}

	return owners
}

// pullFromRequest builds the pull request to open from the request body and the
// checked-out branch, whose name is always the head — the caller does not choose
// it. A reviewer named org/team is requested as a team. It reports false when
// the title or the base is missing.
func pullFromRequest(body api.OpenPullRequestRequest, branch gitrepo.Branch) (forge.NewPullRequest, bool) {
	title := strings.TrimSpace(body.Title)
	base := strings.TrimSpace(body.Base)

	if title == "" || base == "" {
		return forge.NewPullRequest{}, false
	}

	users, teams := loop.SplitReviewers(trimmedList(body.Reviewers))

	return forge.NewPullRequest{
		Title:         title,
		Body:          orZero(body.Body),
		Head:          branch.Name,
		Base:          base,
		Draft:         orZero(body.Draft),
		Reviewers:     users,
		TeamReviewers: teams,
		Assignees:     trimmedList(body.Assignees),
		Labels:        trimmedList(body.Labels),
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

// draftDTO maps the composed draft and its branch onto the wire, its reviewers
// people then teams, as one list the page edits.
func draftDTO(draft forge.NewPullRequest, branch gitrepo.Branch) api.PullRequestDraft {
	return api.PullRequestDraft{
		Title:     draft.Title,
		Body:      draft.Body,
		Base:      draft.Base,
		Head:      draft.Head,
		Draft:     draft.Draft,
		NeedsPush: !branch.Pushed(),
		Templates: []string{}, Template: "",
		Reviewers: append(append(make([]string, 0, len(draft.Reviewers)+len(draft.TeamReviewers)),
			draft.Reviewers...), draft.TeamReviewers...),
	}
}

// openUnprocessable is the 422 response for a pull request the server will not
// open.
func openUnprocessable(message string) api.OpenPullRequest422ApplicationProblemPlusJSONResponse {
	return api.OpenPullRequest422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}

// openFailure answers an open the forge did not make. A pull request the forge
// turned down keeps the forge's reason, which the caller can act on; a
// reviewer or assignee it does not know never stops one opening. Every other
// failure is classified by fault, whose detail names no host: an unreachable
// forge, a redirect the client refused and a missing repository all carry one
// in their text, and a forge that cannot be told is not set up.
func (s *server) openFailure(err error) api.OpenPullRequestResponseObject {
	switch {
	case errors.Is(err, forge.ErrRejected):
		return openUnprocessable(err.Error())
	default:
		body, code := s.fault(err)

		return api.OpenPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}
