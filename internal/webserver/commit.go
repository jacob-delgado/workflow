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
func (s *server) Commit(_ context.Context, request api.CommitRequestObject) (api.CommitResponseObject, error) {
	if request.Body == nil {
		return commitUnprocessable("a commit message is required"), nil
	}

	if s.deps.Commit == nil || s.deps.Changes == nil || s.deps.Branch == nil {
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

	branch, err := s.commitStaged(conv, subject, orZero(request.Body.Body))
	if err != nil {
		return commitFailure(err), nil
	}

	s.rememberScope(subject.Scope)

	return api.Commit200JSONResponse(branchDTO(branch)), nil
}

// commitFailure answers a commit that did not land: an empty index, a commit
// its hooks refused — with their output, the one cause worth forwarding — one
// that could not start, saying how to see why, and a working tree that could
// not be read, classified by fault.
func commitFailure(err error) api.CommitResponseObject {
	switch {
	case errors.Is(err, loop.ErrNothingStaged):
		return api.Commit409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errNothingStaged.Error()))
	case errors.Is(err, errCommitFailed):
		return commitUnprocessable(err.Error())
	case errors.Is(err, errCommitNotStarted):
		return commitUnprocessable(errCommitNotStarted.Error() + "; commit from a terminal to see why")
	default:
		body, code := fault(err)

		return api.CommitdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
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
	conv convention.CommitConvention, subject convention.Subject, body string,
) (gitrepo.Branch, error) {
	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	changes, err := s.deps.Changes()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	err = loop.RefuseNothingStaged(changes)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	before := s.branchBeforeCommit()

	err = s.runCommit(conv.Message(subject, body, s.issueKeyOf(before)))
	if err != nil {
		return gitrepo.Branch{}, err
	}

	return s.branchAfter(withoutCommits(before)), nil
}

// branchBeforeCommit is the checked-out branch as a commit finds it, or an empty
// one when it cannot be read: the commit goes ahead with no issue to refer to.
func (s *server) branchBeforeCommit() gitrepo.Branch {
	branch, err := s.deps.Branch()
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
func (s *server) runCommit(message string) error {
	output, err := s.deps.Commit(message)
	if err != nil {
		return fmt.Errorf("%w: %w", errCommitNotStarted, err)
	}

	var lines []string
	for line := range output.Lines {
		lines = append(lines, line)
	}

	err = output.Wait()
	if err != nil {
		return fmt.Errorf("%w:\n%s", errCommitFailed, strings.Join(lines, "\n"))
	}

	return nil
}

// issueKeyOf is the issue a branch is named for, for the Refs trailer, or empty
// when it names none.
func (s *server) issueKeyOf(branch gitrepo.Branch) string {
	key, _ := convention.IssueKey(branch.Name, s.config().Jira.Project)

	return key
}

// suggestedScope is the scope a new commit opens on — the terminal composer's
// rule: the scope last used in this repository, else commit.default_scope,
// else none.
func (s *server) suggestedScope() string {
	learned, found := s.learnedScope()
	if found {
		return learned
	}

	return s.config().Commit.DefaultScope
}

// learnedScope is the scope last used in this repository, read from the store
// the first time it is asked for. A dry run opens no store, so it never reads
// the learned scope and suggests the configured default.
func (s *server) learnedScope() (string, bool) {
	s.scope.mu.Lock()
	defer s.scope.mu.Unlock()

	if !s.scope.read && s.deps.LastScope != nil && !s.info.DryRun {
		s.scope.value, s.scope.found = s.deps.LastScope()
	}

	s.scope.read = true

	return s.scope.value, s.scope.found
}

// rememberScope records the scope a commit just used, as its message wrote it,
// and has the next frame read the store once for what it kept — nothing, with
// the store off, which leaves commit.default_scope to open the form, as it
// does in the terminal. A blank scope is not recorded: it would erase the one
// learned and hide the configured default, as the terminal's composer knows.
// The record comes first, so a frame reading in between caches no older scope.
func (s *server) rememberScope(scope string) {
	written := strings.TrimSpace(scope)
	if written == "" || s.deps.RecordScope == nil {
		return
	}

	s.deps.RecordScope(written)

	s.scope.mu.Lock()
	s.scope.read = false
	s.scope.mu.Unlock()
}

// commitUnprocessable is the 422 response for a commit the server will not make.
func commitUnprocessable(message string) api.Commit422ApplicationProblemPlusJSONResponse {
	return api.Commit422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}

// orZero dereferences an optional field, or returns its zero value when absent.
func orZero[T any](pointer *T) T {
	if pointer == nil {
		var zero T

		return zero
	}

	return *pointer
}
