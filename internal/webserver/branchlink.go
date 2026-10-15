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
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
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
	ctx context.Context, request api.LinkBranchIssueRequestObject,
) (api.LinkBranchIssueResponseObject, error) {
	ref, known := convention.RefOf(strings.TrimSpace(request.Body.Key))
	if !known || s.deps.Git.LinkIssue == nil {
		return api.LinkBranchIssue422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			refusal(known, errNotAnIssue, errCannotLink))), nil
	}

	branch, err := s.linkableBranch(ctx)
	if errors.Is(err, errNoBranchToLink) {
		return api.LinkBranchIssue409ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeConflict, err.Error())), nil
	}

	if err == nil {
		err = s.linkIssue(ctx, branch.Name, ref, request.Body.UpdatePull)
	}

	if err != nil {
		return problem.Answer[api.LinkBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.LinkBranchIssue200JSONResponse(branchDTO(s.branchAfter(ctx, branch))), nil
}

// UnlinkBranchIssue forgets the issue the checked-out branch was linked to.
func (s *server) UnlinkBranchIssue(
	ctx context.Context, _ api.UnlinkBranchIssueRequestObject,
) (api.UnlinkBranchIssueResponseObject, error) {
	if s.deps.Git.UnlinkIssue == nil {
		return api.UnlinkBranchIssue422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			errCannotLink.Error())), nil
	}

	branch, err := s.linkableBranch(ctx)
	if errors.Is(err, errNoBranchToLink) {
		return api.UnlinkBranchIssue409ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeConflict, err.Error())), nil
	}

	if err == nil {
		err = s.deps.Git.UnlinkIssue(ctx, branch.Name)
	}

	if err != nil {
		return problem.Answer[api.UnlinkBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.UnlinkBranchIssue200JSONResponse(branchDTO(s.branchAfter(ctx, branch))), nil
}

// PreviewBranchIssue is what linking the checked-out branch to an issue would
// write in its open pull request's description.
func (s *server) PreviewBranchIssue(
	ctx context.Context, request api.PreviewBranchIssueRequestObject,
) (api.PreviewBranchIssueResponseObject, error) {
	ref, known := convention.RefOf(strings.TrimSpace(request.Params.Key))
	if !known {
		return api.PreviewBranchIssue422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			errNotAnIssue.Error())), nil
	}

	preview := api.BranchIssuePreview{Key: ref.Key}

	pull, open, err := s.openPull(ctx)
	if err != nil {
		return problem.Answer[api.PreviewBranchIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
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
func (s *server) linkableBranch(ctx context.Context) (gitrepo.Branch, error) {
	if s.deps.Git.Branch == nil {
		return gitrepo.Branch{}, errNoBranchToLink
	}

	branch, err := s.deps.Git.Branch(ctx)
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
func (s *server) linkIssue(ctx context.Context, branch string, ref convention.IssueRef, updatePull bool) error {
	if updatePull {
		err := s.namePullIssue(ctx, ref)
		if err != nil {
			return err
		}
	}

	return s.deps.Git.LinkIssue(ctx, branch, ref.Key)
}

// namePullIssue adds the issue's line to the open pull request's description,
// unless there is none, no way to edit it, or it names the issue already. The
// line is added to the description as the forge holds it, not to the one
// shown, whose controls and marks are neutralized, so nothing else of it
// changes.
func (s *server) namePullIssue(ctx context.Context, ref convention.IssueRef) error {
	if s.deps.Forge.RewriteDescription == nil {
		return nil
	}

	pull, open, err := s.openPull(ctx)
	if err != nil || !open {
		return err
	}

	issueURL := s.browseURL(jira.Key(ref.Key))
	_, err = s.deps.Forge.RewriteDescription(ctx, pull, func(body string) (string, bool) {
		return convention.WithIssueLine(body, ref.Key, issueURL)
	})

	return err
}

// openPull is the pull request open from the checked-out branch, and whether
// there is one: none when HEAD is on no branch or there is no forge to ask.
// A branch git could not read is that failure, not the absence of one.
func (s *server) openPull(ctx context.Context) (forge.PullRequest, bool, error) {
	if s.deps.Forge.FindPullRequest == nil {
		return forge.PullRequest{}, false, nil
	}

	branch, err := s.linkableBranch(ctx)
	if errors.Is(err, errNoBranchToLink) {
		return forge.PullRequest{}, false, nil
	}

	if err != nil {
		return forge.PullRequest{}, false, err
	}

	pull, found, err := s.deps.Forge.FindPullRequest(ctx, branch.Name)
	if err != nil {
		return forge.PullRequest{}, false, err
	}

	return pull, found && pull.IsOpen(), nil
}
