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

// The reasons a link to an issue is refused, in the words the answer carries.
var (
	errNotAnIssue     = errors.New("name an issue: a Jira key like PROJ-7, or a forge number like #42")
	errNoBranchToLink = errors.New("no branch is checked out to link; check one out first")
	errCannotLink     = errors.New("linking a branch is not available here")
)

// LinkBranchIssue links the checked-out branch to an issue, for work begun
// outside workflow, and with update_pull adds the line naming the issue to
// the open pull request's description, unless it names the issue already.
func (s *server) LinkBranchIssue(
	_ context.Context, request api.LinkBranchIssueRequestObject,
) (api.LinkBranchIssueResponseObject, error) {
	ref, known := convention.RefOf(strings.TrimSpace(request.Body.Key))
	if !known || s.deps.Git.LinkIssue == nil {
		return api.LinkBranchIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			refusal(known, errNotAnIssue, errCannotLink))), nil
	}

	branch, err := s.linkableBranch()
	if errors.Is(err, errNoBranchToLink) {
		return api.LinkBranchIssue409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error())), nil
	}

	if err == nil {
		err = s.linkIssue(branch.Name, ref, request.Body.UpdatePull)
	}

	if err != nil {
		return problemAnswer[api.LinkBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.LinkBranchIssue200JSONResponse(branchDTO(s.branchAfter(branch))), nil
}

// UnlinkBranchIssue forgets the issue the checked-out branch was linked to.
func (s *server) UnlinkBranchIssue(
	_ context.Context, _ api.UnlinkBranchIssueRequestObject,
) (api.UnlinkBranchIssueResponseObject, error) {
	if s.deps.Git.UnlinkIssue == nil {
		return api.UnlinkBranchIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			errCannotLink.Error())), nil
	}

	branch, err := s.linkableBranch()
	if errors.Is(err, errNoBranchToLink) {
		return api.UnlinkBranchIssue409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error())), nil
	}

	if err == nil {
		err = s.deps.Git.UnlinkIssue(branch.Name)
	}

	if err != nil {
		return problemAnswer[api.UnlinkBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.UnlinkBranchIssue200JSONResponse(branchDTO(s.branchAfter(branch))), nil
}

// PreviewBranchIssue is what linking the checked-out branch to an issue would
// write in its open pull request's description.
func (s *server) PreviewBranchIssue(
	_ context.Context, request api.PreviewBranchIssueRequestObject,
) (api.PreviewBranchIssueResponseObject, error) {
	ref, known := convention.RefOf(strings.TrimSpace(request.Params.Key))
	if !known {
		return api.PreviewBranchIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			errNotAnIssue.Error())), nil
	}

	preview := api.BranchIssuePreview{Key: ref.Key}

	pull, open, err := s.openPull()
	if err != nil {
		return problemAnswer[api.PreviewBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	if open {
		preview.Pull = pull.Number
		preview.Body, preview.Changes = convention.WithIssueLine(pull.Body, ref.Key, s.browseURL(jira.Key(ref.Key)))
	}

	return api.PreviewBranchIssue200JSONResponse(preview), nil
}

// refusal is ok's reason when ok, and otherwise not's.
func refusal(ok bool, reason, not error) string {
	if ok {
		return reason.Error()
	}

	return not.Error()
}

// linkableBranch is the checked-out branch, or errNoBranchToLink when HEAD is
// on none.
func (s *server) linkableBranch() (gitrepo.Branch, error) {
	if s.deps.Git.Branch == nil {
		return gitrepo.Branch{}, errNoBranchToLink
	}

	branch, err := s.deps.Git.Branch()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	if branch.Detached || branch.Name == "" {
		return gitrepo.Branch{}, errNoBranchToLink
	}

	return branch, nil
}

// linkIssue adds the issue's line to the open pull request's description when
// asked and when it does not name the issue, then keeps the link. The edit
// goes first so a forge that refuses it leaves the branch as it was; a link
// git then cannot keep is answered as such, and asking again is safe, since
// the description names the issue by then.
func (s *server) linkIssue(branch string, ref convention.IssueRef, updatePull bool) error {
	if updatePull {
		err := s.namePullIssue(ref)
		if err != nil {
			return err
		}
	}

	return s.deps.Git.LinkIssue(branch, ref.Key)
}

// namePullIssue adds the issue's line to the open pull request's description,
// unless there is none, no way to edit it, or it names the issue already. The
// line is added to the description as the forge holds it, not to the one
// shown, whose controls and marks are neutralized, so nothing else of it
// changes.
func (s *server) namePullIssue(ref convention.IssueRef) error {
	if s.deps.Forge.RewriteDescription == nil {
		return nil
	}

	pull, open, err := s.openPull()
	if err != nil || !open {
		return err
	}

	issueURL := s.browseURL(jira.Key(ref.Key))
	_, err = s.deps.Forge.RewriteDescription(pull, func(body string) (string, bool) {
		return convention.WithIssueLine(body, ref.Key, issueURL)
	})

	return err
}

// openPull is the pull request open from the checked-out branch, and whether
// there is one: none when HEAD is on no branch or there is no forge to ask.
// A branch git could not read is that failure, not the absence of one.
func (s *server) openPull() (forge.PullRequest, bool, error) {
	if s.deps.Forge.FindPullRequest == nil {
		return forge.PullRequest{}, false, nil
	}

	branch, err := s.linkableBranch()
	if errors.Is(err, errNoBranchToLink) {
		return forge.PullRequest{}, false, nil
	}

	if err != nil {
		return forge.PullRequest{}, false, err
	}

	pull, found, err := s.deps.Forge.FindPullRequest(branch.Name)
	if err != nil {
		return forge.PullRequest{}, false, err
	}

	return pull, found && pull.IsOpen(), nil
}
