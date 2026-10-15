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
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

var (
	// errNothingStaged is the web's words for loop.ErrNothingStaged: a commit
	// refused for an empty index, by the same guard the terminal composer applies.
	errNothingStaged = errors.New("nothing is staged to commit")
	// errCommitFailed carries the commit's own output — a failing hook, most
	// often — so the caller learns why it did not land.
	errCommitFailed = errors.New("the commit failed")
	// errCommitNotStarted is a commit that never ran. Its cause stays off the
	// wire: what git says about its repository can name where that is on disk.
	errCommitNotStarted = errors.New("the commit could not be started")
)

// Commit commits the staged changes with a Conventional Commit message built
// from the request and a Refs trailer for the branch's issue. An empty index is
// a 409; a message that is not a valid Conventional Commit, a commit its hooks
// reject — whose output the answer carries — or one that cannot be started is a
// 422; a working tree that cannot be read is answered by fault. On success it
// returns the branch with the new commit, or, when the branch cannot be read
// back, the branch as it stood with no head or commits named: the commit landed
// all the same.
func (s *server) Commit(ctx context.Context, request api.CommitRequestObject) (api.CommitResponseObject, error) {
	if s.deps.Git.Commit == nil || s.deps.Git.Changes == nil || s.deps.Git.Branch == nil {
		return commitUnprocessable("committing is not available"), nil
	}

	subject := convention.Subject{
		Type:        request.Body.Type,
		Scope:       orZero(request.Body.Scope),
		Description: request.Body.Subject,
		Breaking:    orZero(request.Body.Breaking),
	}

	conv := s.commitConvention()

	err := conv.Validate(subject)
	if err != nil {
		return commitUnprocessable(err.Error()), nil
	}

	branch, err := s.commitStaged(ctx, conv, subject, orZero(request.Body.Body))
	if err != nil {
		return s.commitFailure(err), nil
	}

	s.rememberScope(ctx, subject.Scope)

	return api.Commit200JSONResponse(branchDTO(branch)), nil
}

// commitFailure answers a commit that did not land: an empty index, a commit
// its hooks refused — with their output, the one cause worth forwarding — one
// that could not start, saying how to see why, and a working tree that could
// not be read, classified by fault.
func (s *server) commitFailure(err error) api.CommitResponseObject {
	switch {
	case errors.Is(err, loop.ErrNothingStaged):
		return api.Commit409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, errNothingStaged.Error()))
	case errors.Is(err, errCommitFailed):
		return commitUnprocessable(err.Error())
	case errors.Is(err, errCommitNotStarted):
		return commitUnprocessable(errCommitNotStarted.Error() + "; commit from a terminal to see why")
	default:
		return problemAnswer[api.CommitdefaultApplicationProblemPlusJSONResponse](s.fault(err))
	}
}

// commitConvention is the team's configured commit convention: their own types,
// subject limit and issue trailer where set, and the built-in defaults otherwise.
func (s *server) commitConvention() convention.CommitConvention {
	commit := s.config().Commit

	return convention.NewCommitConvention(commit.Types, commit.SubjectLimit, commit.RefsTrailer)
}

// commitStaged refuses an empty index, then commits it and returns the branch
// now carrying the commit. It holds the index from the read to the commit, so a
// stage under way finishes first and the commit takes the whole of it.
func (s *server) commitStaged(
	ctx context.Context, conv convention.CommitConvention, subject convention.Subject, body string,
) (gitrepo.Branch, error) {
	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	changes, err := s.deps.Git.Changes(ctx)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	err = loop.RefuseNothingStaged(changes)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	before := s.branchBeforeCommit(ctx)

	err = s.runCommit(ctx, conv.Message(subject, body, s.issueKeyOf(before)))
	if err != nil {
		return gitrepo.Branch{}, err
	}

	return s.branchAfter(ctx, withoutCommits(before)), nil
}

// branchBeforeCommit is the checked-out branch as a commit finds it, or an empty
// one when it cannot be read: the commit goes ahead with no issue to refer to.
func (s *server) branchBeforeCommit(ctx context.Context) gitrepo.Branch {
	branch, err := s.deps.Git.Branch(ctx)
	if err != nil {
		return gitrepo.Branch{}
	}

	return branch
}

// withoutCommits is the branch as read before a commit, less its head and
// commits: they no longer describe it, and the newest would be named as the
// commit just made.
func withoutCommits(before gitrepo.Branch) gitrepo.Branch {
	after := before
	after.Head, after.Commits = "", nil

	return after
}

// runCommit runs the commit, draining its output so the hooks run to completion,
// and reports a failing commit with the output that explains why.
func (s *server) runCommit(ctx context.Context, message string) error {
	output, err := s.deps.Git.Commit(ctx, message)
	if err != nil {
		return fmt.Errorf("%w: %w", errCommitNotStarted, err)
	}

	lines, err := output.Drain()
	if err != nil {
		return fmt.Errorf("%w:\n%s", errCommitFailed, strings.Join(lines, "\n"))
	}

	return nil
}

// issueKeyOf is the issue a branch is for, by its link or its name, for the
// Refs trailer, or empty when it is for none.
func (s *server) issueKeyOf(branch gitrepo.Branch) string {
	key, _ := loop.IssueOf(branch, s.config().Jira.Project)

	return key.Key
}

// suggestedScope is the scope a new commit opens on — the terminal composer's
// rule: the scope last used in this repository, else commit.default_scope,
// else none. The store is read under ctx.
func (s *server) suggestedScope(ctx context.Context) string {
	learned, found := s.learnedScope(ctx)
	if found {
		return learned
	}

	return s.config().Commit.DefaultScope
}

// learnedScope is the scope last used in this repository, read from the store
// the first time it is asked for. A dry run reads nothing from the cache, only
// kept.db, read-only, so it never reads the learned scope and suggests the
// configured default. Every stream reads the one held, and the store's read
// takes a failure for none, so it runs to its end whoever asked it leaves, and
// ends when the server stops.
func (s *server) learnedScope(ctx context.Context) (string, bool) {
	s.scope.mu.Lock()
	defer s.scope.mu.Unlock()

	if !s.scope.read && s.deps.Store.LastScope != nil && !s.info.DryRun {
		s.scope.value, s.scope.found = s.readLastScope(ctx)
	}

	s.scope.read = true

	return s.scope.value, s.scope.found
}

// readLastScope is the store's read of the scope last used, under a context
// that goes on when ctx ends and ends when the server stops.
func (s *server) readLastScope(ctx context.Context) (string, bool) {
	shared, release := s.worlds.outliving(ctx)
	defer release()

	return s.deps.Store.LastScope(shared)
}

// rememberScope records the scope a commit just used through the loop's rule,
// the terminal's too, and when it recorded one has the next frame read the
// store once for what it kept — nothing, with the store off, which leaves
// commit.default_scope to open the form, as it does in the terminal. The record
// comes first, so a frame reading in between caches no older scope. A store
// that could not keep it is noted, since the commit it followed was made. It
// is recorded under ctx.
func (s *server) rememberScope(ctx context.Context, scope string) {
	recorded, err := loop.RememberScope(ctx, s.deps.Store.RecordScope, scope)
	if err != nil {
		s.unexpected(err)
	}

	if !recorded {
		return
	}

	s.scope.mu.Lock()
	s.scope.read = false
	s.scope.mu.Unlock()
}

// commitUnprocessable is the 422 response for a commit the server will not make.
func commitUnprocessable(message string) api.Commit422ApplicationProblemPlusJSONResponse {
	return api.Commit422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, message))
}
