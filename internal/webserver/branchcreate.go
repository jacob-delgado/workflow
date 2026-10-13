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
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// errBranchExists refuses creating a branch for an issue that already has one —
// that branch should be checked out, not recreated.
var errBranchExists = errors.New("a branch for this issue already exists")

// errCreateRefused is git declining to create or switch to the new branch. Its
// own words stay off the wire: switching checks the branch's tree out, which in
// a partial clone fetches from the remote, and a fetch that fails names it.
var errCreateRefused = errors.New("git refused the new branch")

// errWorktreeRefused is git declining to make the new worktree, its own words
// kept off the wire as errCreateRefused's are.
var errWorktreeRefused = errors.New("git refused the new worktree")

// CreateBranch names a branch for an issue by the branch-name convention,
// creates it off the base branch, and switches to it — how a not-started issue
// is picked up. A branch that already exists is a 409, git refusing the branch
// is a 422, which says how to see git's reason, and an issue the tracker could
// not read or a branch list git could not give is answered by fault. On success
// it returns the branch now in effect, or the created branch by its name alone
// when it cannot be read back.
func (s *server) CreateBranch(
	ctx context.Context, request api.CreateBranchRequestObject,
) (api.CreateBranchResponseObject, error) {
	if s.deps.Git.CreateBranch == nil || s.deps.Jira.Issue == nil || s.deps.Git.Branch == nil {
		return createBranchUnprocessable("creating a branch is not available"), nil
	}

	branch, err := s.startWork(ctx, request.Body.IssueKey, s.fetchFor(request.Body.Fetch))
	if err != nil {
		return s.createBranchFailure(err, request.Body.IssueKey), nil
	}

	return api.CreateBranch200JSONResponse(branchDTO(branch)), nil
}

// createBranchFailure answers a start of work that made no branch, as
// startRefusal words it.
func (s *server) createBranchFailure(err error, key string) api.CreateBranchResponseObject {
	return problemAnswer[api.CreateBranchdefaultApplicationProblemPlusJSONResponse](s.startRefusal(err, key))
}

// startRefusal words a start of work that made nothing: git's refusal, saying
// how to see its reason, or anything else — a branch already there, a read
// that failed, the issue from the tracker, the branch list from git —
// classified by fault, so neither read's own words reach the wire.
func (s *server) startRefusal(err error, key string) api.Problem {
	switch {
	case errors.Is(err, loop.ErrFetchFailed):
		return problem(api.ProblemCodeFetchFailed, "the base remote could not be fetched, so nothing was made for "+key+
			"; start work from what you have, or run git fetch in a terminal to see why")
	case errors.Is(err, errCreateRefused):
		return problem(api.ProblemCodeUnprocessable, "git would not create the branch for "+key+
			"; run workflow branch "+key+" from a terminal to see git's reason")
	case errors.Is(err, errWorktreeRefused):
		return problem(api.ProblemCodeUnprocessable, "git would not make a worktree for "+key+
			"; make it with ctrl+w in the terminal interface's branch creator to see git's reason")
	default:
		return s.fault(err)
	}
}

// startWork names, creates and switches to a branch for the issue, after
// fetch when there is one, refusing when one already exists, and returns the
// branch now in effect — or, when that cannot be read back, one of the
// created name, since the branch read before the create is the one it left.
// The tracker is asked under ctx.
func (s *server) startWork(
	ctx context.Context, issueKey string, fetch func(context.Context) error,
) (gitrepo.Branch, error) {
	name, err := s.branchNameFor(ctx, issueKey)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	err = s.createUnlessTaken(ctx, name, errCreateRefused, fetch, func(base string) error {
		return s.deps.Git.CreateBranch(ctx, name, base)
	})
	if err != nil {
		return gitrepo.Branch{}, err
	}

	return s.branchAfter(ctx, gitrepo.Branch{Name: name}), nil
}

// branchNameFor is the branch the convention names for the issue, from its type
// and summary read through the tracker under ctx.
func (s *server) branchNameFor(ctx context.Context, issueKey string) (string, error) {
	detail, err := s.deps.Jira.Issue(ctx, jira.Key(issueKey))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", issueKey, err)
	}

	return s.config().Branch.Naming().Name(detail.Issue.Type, string(detail.Issue.Key), detail.Issue.Summary), nil
}

// branchExists reports whether a local branch already goes by name. Without a
// branch-list seam it cannot tell, and reads as absent so the create is tried.
func (s *server) branchExists(ctx context.Context, name string) (bool, error) {
	if s.deps.Git.Branches == nil {
		return false, nil
	}

	names, err := s.deps.Git.Branches(ctx)
	if err != nil {
		return false, err
	}

	return slices.Contains(names, name), nil
}

// currentBranchBase is the base a new branch starts from — the checked-out
// branch's base (the base remote's default), or empty to branch from HEAD.
func (s *server) currentBranchBase(ctx context.Context) string {
	branch, err := s.deps.Git.Branch(ctx)
	if err != nil {
		return ""
	}

	return branch.Base
}

// fetchFor is the fetch a start of work makes first: the base remote's, unless
// the request asks to branch from what you have, or there is no fetch seam.
func (s *server) fetchFor(asked *bool) func(context.Context) error {
	if asked != nil && !*asked {
		return nil
	}

	return s.deps.Git.Fetch
}

// createBranchUnprocessable is the 422 response for a branch the server will not
// create.
func createBranchUnprocessable(message string) api.CreateBranch422ApplicationProblemPlusJSONResponse {
	return api.CreateBranch422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, message))
}

// createUnlessTaken makes the branch name from the base with create, unless a
// branch already goes by it, after fetch when there is a base to refresh,
// holding the index throughout: a stage never meets the new branch, and two
// starts of work on one issue at once make it once, the other refused as
// taken. A fetch that fails makes nothing and is loop.ErrFetchFailed; git
// refusing the create is refused, wrapped in refused.
func (s *server) createUnlessTaken(
	ctx context.Context, name string, refused error, fetch func(context.Context) error, create func(base string) error,
) error {
	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	exists, err := s.branchExists(ctx, name)
	if err != nil {
		return err
	}

	if exists {
		return errBranchExists
	}

	base := s.currentBranchBase(ctx)
	if base == "" {
		fetch = nil
	}

	return loop.FetchThen(ctx, fetch, func() error {
		made := create(base)
		if made != nil {
			return fmt.Errorf("%w: creating %s: %w", refused, name, made)
		}

		return nil
	})
}

// CreateWorktree names a branch for an issue as CreateBranch does and creates
// it off the base branch in a new worktree beside the repository, leaving the
// server's own checkout where it is: switching there is SwitchRepository's.
// It is refused as CreateBranch is, and answers the worktree's directory.
func (s *server) CreateWorktree(
	ctx context.Context, request api.CreateWorktreeRequestObject,
) (api.CreateWorktreeResponseObject, error) {
	key := request.Body.IssueKey

	if s.deps.Git.CreateWorktree == nil || s.deps.Jira.Issue == nil || s.deps.Git.Branch == nil {
		return createWorktreeUnprocessable("creating a worktree is not available"), nil
	}

	var dir string

	name, err := s.branchNameFor(ctx, key)
	if err == nil {
		err = s.createUnlessTaken(ctx, name, errWorktreeRefused, s.fetchFor(request.Body.Fetch), func(base string) error {
			var made error

			dir, made = s.deps.Git.CreateWorktree(ctx, name, base)

			return made
		})
	}

	if err == nil {
		shown := workdirs.Shown(dir, s.deps.Repositories.Home)

		return api.CreateWorktree200JSONResponse{Dir: dir, Shown: shown, Branch: name}, nil
	}

	return problemAnswer[api.CreateWorktreedefaultApplicationProblemPlusJSONResponse](s.startRefusal(err, key)), nil
}

// createWorktreeUnprocessable is the 422 response for a worktree the server
// will not create.
func createWorktreeUnprocessable(message string) api.CreateWorktree422ApplicationProblemPlusJSONResponse {
	return api.CreateWorktree422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, message))
}
