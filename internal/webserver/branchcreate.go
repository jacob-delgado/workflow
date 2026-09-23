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

// CreateBranch names a branch for an issue by the branch-name convention,
// creates it off the base branch, and switches to it — how a not-started issue
// is picked up. A branch that already exists is a 409; anything else that stops
// the creation is a 422. On success it returns the branch now in effect.
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

	switch {
	case err == nil:
		return api.CreateBranch200JSONResponse(branchDTO(branch)), nil
	case errors.Is(err, errBranchExists):
		return api.CreateBranch409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errBranchExists.Error())), nil
	default:
		return createBranchUnprocessable("the branch could not be created"), nil
	}
}

// startWork names, creates and switches to a branch for the issue, refusing when
// one already exists, and returns the branch now in effect.
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

	err = s.deps.CreateBranch(name, s.currentBranchBase())
	if err != nil {
		return gitrepo.Branch{}, fmt.Errorf("creating %s: %w", name, err)
	}

	return s.deps.Branch()
}

// branchNameFor is the branch the convention names for the issue, from its type
// and summary read through the tracker.
func (s *server) branchNameFor(issueKey string) (string, error) {
	detail, err := s.deps.Issue(jira.Key(issueKey))
	if err != nil {
		return "", err
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
