// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Why a write to the branch's pull request was not made.
var (
	errReviewWriteUnavailable = errors.New("that is not available here")
	errNoOpenPull             = errors.New("the branch has no open pull request")
	errNotMergeable           = errors.New("it cannot be merged yet: it must be open, ready for review, " +
		"free of conflicts, approved with no changes asked for, and its CI passed")
	errNotFinishable = errors.New("this branch cannot be finished: its pull request must have merged, " +
		"with a base to return to and no commit origin lacks, which deleting the branch would lose")
	errNothingToRerun = errors.New("there is nothing to re-run: only failed CI on an open pull request is")
	errNoMergeMethod  = errors.New("the repository permits no merge method")
	errTitleRequired  = errors.New("a title is required")
)

// pullRead is the checked-out branch, its pull request, whether there is
// one, and its CI, each read afresh, so a write goes only where the forge
// says it can now — never on what a frame showed an interval ago.
type pullRead struct {
	branch gitrepo.Branch
	pull   forge.PullRequest
	found  bool
	ci     forge.CI
}

// readPull reads the branch and its pull request afresh, and its CI when asked
// and there is a seam to read it with.
func (s *server) readPull(withCI bool) (pullRead, error) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return pullRead{}, nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return pullRead{}, err
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		return pullRead{}, err
	}

	read := pullRead{branch: branch, pull: pull, found: found}
	if withCI && found && s.deps.CheckCI != nil {
		read.ci, err = s.deps.CheckCI(pull, branch.Head)
	}

	return read, err
}

// refusal is the problem a review write is refused with: a conflict for what
// the pull request is not ready for, a 422 for what is not available or not
// permitted, and anything else — a read or a write that failed — classified
// by fault, which tells a token turned down in the forge's words.
func (s *server) refusal(err error) api.Problem {
	switch {
	case errors.Is(err, errNoOpenPull), errors.Is(err, errNotFinishable), errors.Is(err, errNothingToRerun):
		return problem(api.ProblemCodeConflict, err.Error())
	case errors.Is(err, errFinishRefused):
		return problem(api.ProblemCodeUnprocessable, "git would not finish the branch; "+
			"finish it with F in the terminal's Review pane to see git's reason")
	case errors.Is(err, errNotMergeable):
		return problem(api.ProblemCodeConflict, "the "+s.noun()+" "+err.Error())
	case errors.Is(err, errReviewWriteUnavailable), errors.Is(err, errNoMergeMethod),
		errors.Is(err, errTitleRequired), errors.Is(err, errMethodNotPermitted):
		return problem(api.ProblemCodeUnprocessable, err.Error())
	default:
		failure := s.fault(err)

		return failure
	}
}

// branchOpenPull is the branch's open pull request, read afresh, or errNoOpenPull.
func (s *server) branchOpenPull() (forge.PullRequest, error) {
	read, err := s.readPull(false)
	if err != nil {
		return forge.PullRequest{}, err
	}

	if !read.found || !read.pull.IsOpen() {
		return forge.PullRequest{}, errNoOpenPull
	}

	return read.pull, nil
}

// GetPullRequestText reads the branch's open pull request's title and
// description afresh, for the editor the terminal's e opens.
func (s *server) GetPullRequestText(
	context.Context, api.GetPullRequestTextRequestObject,
) (api.GetPullRequestTextResponseObject, error) {
	pull, err := s.branchOpenPull()
	if err != nil {
		return problemAnswer[api.GetPullRequestTextdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	return api.GetPullRequestText200JSONResponse(api.PullRequestText{Title: pull.Title, Body: pull.Body}), nil
}

// EditPullRequest saves a new title and description on the branch's open pull
// request, as the terminal's editor does.
func (s *server) EditPullRequest(
	_ context.Context, request api.EditPullRequestRequestObject,
) (api.EditPullRequestResponseObject, error) {
	edited, err := s.editPull(forge.PullRequestEdit{Title: request.Body.Title, Body: request.Body.Body})
	if err != nil {
		return problemAnswer[api.EditPullRequestdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	return api.EditPullRequest200JSONResponse(pullDTO(edited)), nil
}

// editPull checks the edit and makes it on the branch's open pull request.
func (s *server) editPull(edit forge.PullRequestEdit) (forge.PullRequest, error) {
	if s.deps.EditPull == nil {
		return forge.PullRequest{}, errReviewWriteUnavailable
	}

	if strings.TrimSpace(edit.Title) == "" {
		return forge.PullRequest{}, errTitleRequired
	}

	pull, err := s.branchOpenPull()
	if err != nil {
		return forge.PullRequest{}, err
	}

	edited, err := s.deps.EditPull(pull, edit)
	if err != nil {
		return forge.PullRequest{}, err
	}

	s.forgeAnswer.drop()

	return edited, nil
}

// errMethodNotPermitted refuses a merge by a method the repository does not
// permit.
var errMethodNotPermitted = errors.New("the repository does not permit merging by that method")

// mergeablePull is the branch's pull request, read afresh with its CI, when it
// can be merged by loop.CanMerge — the terminal's own rule — or errNotMergeable.
func (s *server) mergeablePull() (forge.PullRequest, error) {
	if s.deps.Merge == nil || s.deps.MergeMethods == nil {
		return forge.PullRequest{}, errReviewWriteUnavailable
	}

	read, err := s.readPull(true)
	if err != nil {
		return forge.PullRequest{}, err
	}

	if !read.found || !loop.CanMerge(read.pull, read.ci) {
		return forge.PullRequest{}, errNotMergeable
	}

	return read.pull, nil
}

// permittedMethods is the merge methods the repository permits, or
// errNoMergeMethod when it permits none.
func (s *server) permittedMethods() ([]forge.MergeMethod, error) {
	methods, err := s.deps.MergeMethods()
	if err != nil {
		return nil, err
	}

	if len(methods) == 0 {
		return nil, errNoMergeMethod
	}

	return methods, nil
}

// GetMergeMethods is the merge's preview: the pull request it would merge and
// the methods the repository permits, once it can be merged.
func (s *server) GetMergeMethods(
	context.Context, api.GetMergeMethodsRequestObject,
) (api.GetMergeMethodsResponseObject, error) {
	pull, err := s.mergeablePull()

	var methods []forge.MergeMethod
	if err == nil {
		methods, err = s.permittedMethods()
	}

	if err != nil {
		return problemAnswer[api.GetMergeMethodsdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	offered := make([]api.MergeMethod, 0, len(methods))
	for _, method := range methods {
		offered = append(offered, api.MergeMethod(method))
	}

	return api.GetMergeMethods200JSONResponse(api.MergeOffer{Pull: pullDTO(pull), Methods: offered}), nil
}

// MergePullRequest merges the branch's pull request by the method asked, once
// it can still be merged and the repository permits that method.
func (s *server) MergePullRequest(
	_ context.Context, request api.MergePullRequestRequestObject,
) (api.MergePullRequestResponseObject, error) {
	pull, err := s.merge(forge.MergeMethod(request.Body.Method))
	if err != nil {
		return problemAnswer[api.MergePullRequestdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	return api.MergePullRequest200JSONResponse(pullDTO(pull)), nil
}

// merge merges the branch's pull request by method, and answers it merged.
func (s *server) merge(method forge.MergeMethod) (forge.PullRequest, error) {
	pull, err := s.mergeablePull()
	if err != nil {
		return forge.PullRequest{}, err
	}

	methods, err := s.permittedMethods()
	if err != nil {
		return forge.PullRequest{}, err
	}

	if !slices.Contains(methods, method) {
		return forge.PullRequest{}, errMethodNotPermitted
	}

	err = s.deps.Merge(pull, method)
	if err != nil {
		return forge.PullRequest{}, err
	}

	s.forgeAnswer.drop()

	pull.State = forge.StateMerged

	return pull, nil
}

// FinishBranch finishes a merged branch as the terminal's F does: switch to
// the base, catch it up, delete the branch. git's own words on a refusal stay
// off the wire, since a pull names origin; Unexpected hears them.
func (s *server) FinishBranch(context.Context, api.FinishBranchRequestObject) (api.FinishBranchResponseObject, error) {
	branch, err := s.finish()
	if err != nil {
		return problemAnswer[api.FinishBranchdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	return api.FinishBranch200JSONResponse(branchDTO(branch)), nil
}

// errFinishRefused is git declining a step of the finish.
var errFinishRefused = errors.New("git refused the finish")

// finish runs the finish once loop.CanFinish allows it, and answers the
// branch now checked out.
func (s *server) finish() (gitrepo.Branch, error) {
	if s.deps.Finish == nil {
		return gitrepo.Branch{}, errReviewWriteUnavailable
	}

	read, err := s.readPull(false)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	if !read.found || !loop.CanFinish(read.pull, read.branch) {
		return gitrepo.Branch{}, errNotFinishable
	}

	base := read.branch.BaseName()

	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	err = s.deps.Finish(read.branch.Name, base)
	if err != nil {
		s.unexpected(err)

		return gitrepo.Branch{}, errFinishRefused
	}

	s.forgeAnswer.drop()

	return s.branchAfter(gitrepo.Branch{Name: base}), nil
}

// RerunChecks asks the forge to re-run the failed CI of the branch's open pull
// request, read afresh, and says whether anything was re-run.
func (s *server) RerunChecks(context.Context, api.RerunChecksRequestObject) (api.RerunChecksResponseObject, error) {
	reran, err := s.rerun()
	if err != nil {
		return problemAnswer[api.RerunChecksdefaultApplicationProblemPlusJSONResponse](s.refusal(err)), nil
	}

	return api.RerunChecks200JSONResponse(api.Rerun{Reran: reran}), nil
}

// rerun re-runs the failed CI once loop.CanRerun allows it.
func (s *server) rerun() (bool, error) {
	if s.deps.Rerun == nil {
		return false, errReviewWriteUnavailable
	}

	read, err := s.readPull(true)
	if err != nil {
		return false, err
	}

	if !read.found || !loop.CanRerun(read.pull, read.ci) {
		return false, errNothingToRerun
	}

	reran, err := s.deps.Rerun(read.pull, read.branch.Head)
	if err != nil {
		return false, err
	}

	s.forgeAnswer.drop()

	return reran, nil
}
