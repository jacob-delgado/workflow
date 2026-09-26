// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// errBranchExists refuses creating a branch for an issue that already has one —
// that branch should be checked out, not recreated.
var errBranchExists = errors.New("a branch for this issue already exists")

// errIssueUnread is the tracker failing to read the issue the branch is named
// for. Its cause is answered as fault classifies it — the tracker's own words
// can carry its address.
var errIssueUnread = errors.New("the issue could not be read")

// errCreateRefused is git declining to create or switch to the new branch. Its
// own words stay off the wire: switching checks the branch's tree out, which in
// a partial clone fetches from the remote, and a fetch that fails names it.
var errCreateRefused = errors.New("git refused the new branch")

// CreateBranch names a branch for an issue by the branch-name convention,
// creates it off the base branch, and switches to it — how a not-started issue
// is picked up. A branch that already exists is a 409, and an issue the tracker
// could not read is answered by fault; anything else that stops the creation is
// a 422, which says how to see why. On success it returns the branch now in
// effect, or the created branch by its name alone when it cannot be read back.
func (s *server) CreateBranch(
	_ context.Context, request api.CreateBranchRequestObject,
) (api.CreateBranchResponseObject, error) {
	if request.Body == nil || request.Body.IssueKey == "" {
		return createBranchUnprocessable("an issue is required"), nil
	}

	if s.deps.CreateBranch == nil || s.deps.Issue == nil || s.deps.Branch == nil {
		return createBranchUnprocessable("creating a branch is not available"), nil
	}

	branch, err := s.startWork(request.Body.IssueKey)
	if err != nil {
		return createBranchFailure(err, request.Body.IssueKey), nil
	}

	return api.CreateBranch200JSONResponse(branchDTO(branch)), nil
}

// createBranchFailure answers a start of work that made no branch: a branch
// already there, an issue the tracker could not read — classified by fault, so
// its address stays off the wire — git's refusal, or anything else, each saying
// what to do next.
func createBranchFailure(err error, key string) api.CreateBranchResponseObject {
	switch {
	case errors.Is(err, errBranchExists):
		return api.CreateBranch409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errBranchExists.Error()))
	case errors.Is(err, errIssueUnread):
		body, code := fault(err)

		return api.CreateBranchdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	case errors.Is(err, errCreateRefused):
		return createBranchUnprocessable("git would not create the branch for " + key +
			"; run workflow branch " + key + " from a terminal to see git's reason")
	default:
		return createBranchUnprocessable("the branch for " + key + " could not be created; try again, or run " +
			"workflow branch " + key + " from a terminal to see why")
	}
}

// startWork names, creates and switches to a branch for the issue, refusing when
// one already exists, and returns the branch now in effect — or, when that
// cannot be read back, one of the created name, since the branch read before
// the create is the one it left.
func (s *server) startWork(issueKey string) (gitrepo.Branch, error) {
	name, err := s.branchNameFor(issueKey)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	exists, err := s.branchExists(name)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	if exists {
		return gitrepo.Branch{}, errBranchExists
	}

	err = s.createAndSwitch(name)
	if err != nil {
		return gitrepo.Branch{}, fmt.Errorf("%w: creating %s: %w", errCreateRefused, name, err)
	}

	return s.branchAfter(gitrepo.Branch{Name: name}), nil
}

// branchNameFor is the branch the convention names for the issue, from its type
// and summary read through the tracker.
func (s *server) branchNameFor(issueKey string) (string, error) {
	detail, err := s.deps.Issue(jira.Key(issueKey))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errIssueUnread, err)
	}

	return s.config().Branch.Naming().Name(detail.Issue.Type, string(detail.Issue.Key), detail.Issue.Summary), nil
}

// branchExists reports whether a local branch already goes by name. Without a
// branch-list seam it cannot tell, and reads as absent so the create is tried.
func (s *server) branchExists(name string) (bool, error) {
	if s.deps.Branches == nil {
		return false, nil
	}

	names, err := s.deps.Branches()
	if err != nil {
		return false, err
	}

	return slices.Contains(names, name), nil
}

// currentBranchBase is the base a new branch starts from — the checked-out
// branch's base (origin's default), or empty to branch from HEAD.
func (s *server) currentBranchBase() string {
	branch, err := s.deps.Branch()
	if err != nil {
		return ""
	}

	return branch.Base
}

// createBranchUnprocessable is the 422 response for a branch the server will not
// create.
func createBranchUnprocessable(message string) api.CreateBranch422ApplicationProblemPlusJSONResponse {
	return api.CreateBranch422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}

// createAndSwitch creates name from the base and switches to it, holding the
// index so it never meets a stage under way.
func (s *server) createAndSwitch(name string) error {
	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	return s.deps.CreateBranch(name, s.currentBranchBase())
}
