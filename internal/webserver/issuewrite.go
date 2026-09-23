// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Why the checked-out branch's pull request cannot be linked on an issue.
var (
	// errNotTheBranchIssue refuses a link on an issue the checked-out branch
	// does not name: the pull request would be recorded against the wrong one.
	errNotTheBranchIssue = errors.New("the checked-out branch does not name that issue")
	// errNoPullToLink refuses a link when the forge has no pull request for the
	// checked-out branch.
	errNoPullToLink = errors.New("the checked-out branch has no pull request to link")
)

// LinkPullRequest records the checked-out branch's pull request as a link on
// the issue the branch names — the first offer the terminal and `workflow pr`
// make once a pull request is open. The pull request is found here, from the
// branch; the request names only the issue, so a caller can never have a URL of
// its choosing recorded on the tracker.
func (s *server) LinkPullRequest(
	_ context.Context, request api.LinkPullRequestRequestObject,
) (api.LinkPullRequestResponseObject, error) {
	if s.deps.LinkPullRequest == nil || s.deps.Branch == nil || s.deps.FindPull == nil {
		return api.LinkPullRequest422ApplicationProblemPlusJSONResponse(
			problem(api.Unprocessable, "linking a "+s.noun()+" on an issue is not available")), nil
	}

	issueKey := jira.Key(request.Key)

	pull, err := s.branchPull(issueKey)
	if err != nil {
		return s.linkRefusal(err, issueKey), nil
	}

	err = s.deps.LinkPullRequest(issueKey, pull.URL, pull.Title)
	if err != nil {
		body, code := fault(err)

		return api.LinkPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.LinkPullRequest200JSONResponse(pullDTO(pull)), nil
}

// branchPull is the pull request the forge has for the checked-out branch, when
// that branch names issueKey.
func (s *server) branchPull(issueKey jira.Key) (forge.PullRequest, error) {
	branch, err := s.deps.Branch()
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("reading the branch: %w", err)
	}

	named, ok := s.branchIssue(branch)
	if !ok || named != issueKey {
		return forge.PullRequest{}, errNotTheBranchIssue
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("finding the branch's pull request: %w", err)
	}

	if !found {
		return forge.PullRequest{}, errNoPullToLink
	}

	return pull, nil
}

// linkRefusal answers a link that was not made: a 409 when the branch does not
// name the issue or has nothing to link, and the curated fault otherwise.
func (s *server) linkRefusal(err error, issueKey jira.Key) api.LinkPullRequestResponseObject {
	switch {
	case errors.Is(err, errNotTheBranchIssue):
		return api.LinkPullRequest409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"the checked-out branch does not name "+string(issueKey)+"; switch to its branch to link it"))
	case errors.Is(err, errNoPullToLink):
		return api.LinkPullRequest409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"the checked-out branch has no "+s.noun()+" to link on "+string(issueKey)))
	default:
		body, code := fault(err)

		return api.LinkPullRequestdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}

// TransitionIssue moves an issue to the configured review status — the second
// offer after opening — and nowhere else, without fields. It is not a general
// transition: a move Jira wants fields for belongs to the terminal's status
// picker, which asks for them.
func (s *server) TransitionIssue(
	_ context.Context, request api.TransitionIssueRequestObject,
) (api.TransitionIssueResponseObject, error) {
	if s.deps.Transitions == nil || s.deps.Transition == nil {
		return api.TransitionIssue422ApplicationProblemPlusJSONResponse(
			problem(api.Unprocessable, "moving an issue is not available")), nil
	}

	issueKey := jira.Key(request.Key)
	status := s.config().Jira.ReviewStatus

	move, err := loop.FindReviewTransition(s.deps.Transitions, issueKey, status)
	if err != nil {
		return transitionRefusal(err, issueKey, status), nil
	}

	err = s.deps.Transition(issueKey, move, nil)
	if err != nil {
		body, code := fault(err)

		return api.TransitionIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.TransitionIssue200JSONResponse{Key: request.Key, Status: move.ToStatus}, nil
}

// transitionRefusal answers a move to the review status that was not made, in
// words that say what to do; a tracker that could not be read is the curated
// fault, which never carries the tracker's address.
func transitionRefusal(err error, issueKey jira.Key, status string) api.TransitionIssueResponseObject {
	switch {
	case errors.Is(err, loop.ErrNoReviewStatus):
		return api.TransitionIssue422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable,
			"no review status is configured; set jira.review_status to move an issue there"))
	case errors.Is(err, loop.ErrReviewNeedsFields):
		return api.TransitionIssue409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"Jira wants fields filled to move "+string(issueKey)+" to "+status+
				"; move it from the terminal interface, which asks for them"))
	case errors.Is(err, loop.ErrNoReviewTransition):
		return api.TransitionIssue409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"Jira offers no move of "+string(issueKey)+" to "+status+" from where it stands"))
	default:
		body, code := fault(err)

		return api.TransitionIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}
}

// followUps are what the page can offer once a pull request is open, in the
// order the terminal and `workflow pr` offer them: to link it on the issue the
// branch names, then to move that issue to the review status. A branch that
// names no Jira issue is offered neither, and a tracker that cannot be read
// offers no move — the pull request is already open either way. The list is
// never nil, so it goes to the page as [] rather than null.
func (s *server) followUps(branch gitrepo.Branch) []api.FollowUp {
	offers := []api.FollowUp{}

	issueKey, named := s.branchIssue(branch)
	if !named {
		return offers
	}

	if s.deps.LinkPullRequest != nil {
		offers = append(offers, api.FollowUp{Action: api.Link, IssueKey: string(issueKey)})
	}

	if s.deps.Transition == nil {
		return offers
	}

	move, ok := loop.ReviewTransition(s.deps.Transitions, issueKey, s.config().Jira.ReviewStatus)
	if ok {
		offers = append(offers, api.FollowUp{Action: api.Transition, IssueKey: string(issueKey), Status: &move.ToStatus})
	}

	return offers
}

// branchIssue is the Jira issue a branch names, and whether it names one: a key
// with its project, PROJ-42 — never the bare forge issue number a branch can
// carry too, which Jira would refuse or read as an unrelated issue's id.
func (s *server) branchIssue(branch gitrepo.Branch) (jira.Key, bool) {
	key, named := convention.IssueKey(branch.Name, s.config().Jira.Project)
	if !named || !strings.Contains(key, "-") {
		return "", false
	}

	return jira.Key(key), true
}
