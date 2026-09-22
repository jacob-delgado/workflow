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
)

var (
	// errNothingStaged refuses a commit when the index is empty, the same guard
	// the terminal composer applies.
	errNothingStaged = errors.New("nothing is staged to commit")
	// errCommitFailed carries the commit's own output — a failing hook, most
	// often — so the caller learns why it did not land.
	errCommitFailed = errors.New("the commit failed")
)

// Commit commits the staged changes with a Conventional Commit message built
// from the request and a Refs trailer for the branch's issue. An empty index is
// a 409; a message that is not a valid Conventional Commit, or a commit its
// hooks reject, is a 422. On success it returns the branch with the new commit.
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

	switch {
	case err == nil:
		return api.Commit200JSONResponse(branchDTO(branch)), nil
	case errors.Is(err, errNothingStaged):
		return api.Commit409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errNothingStaged.Error())), nil
	default:
		return commitUnprocessable(err.Error()), nil
	}
}

// commitConvention is the team's configured commit convention: their own types,
// subject limit and issue trailer where set, and the built-in defaults otherwise.
func (s *server) commitConvention() convention.CommitConvention {
	commit := s.config().Commit

	return convention.NewCommitConvention(commit.Types, commit.SubjectLimit, commit.RefsTrailer)
}

// commitStaged refuses an empty index, then commits it and returns the branch
// now carrying the commit.
func (s *server) commitStaged(
	conv convention.CommitConvention, subject convention.Subject, body string,
) (gitrepo.Branch, error) {
	staged, err := s.stagedCount()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	if staged == 0 {
		return gitrepo.Branch{}, errNothingStaged
	}

	message := conv.Message(subject, body, s.currentIssueKey())

	err = s.runCommit(message)
	if err != nil {
		return gitrepo.Branch{}, err
	}

	return s.deps.Branch()
}

// runCommit runs the commit, draining its output so the hooks run to completion,
// and reports a failing commit with the output that explains why.
func (s *server) runCommit(message string) error {
	output, err := s.deps.Commit(message)
	if err != nil {
		return fmt.Errorf("starting the commit: %w", err)
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

// stagedCount is how many changes are staged, so a commit with nothing staged
// can be refused before it runs.
func (s *server) stagedCount() (int, error) {
	changes, err := s.deps.Changes()
	if err != nil {
		return 0, err
	}

	count := 0

	for _, change := range changes {
		if change.IsStaged() {
			count++
		}
	}

	return count, nil
}

// currentIssueKey is the issue the checked-out branch is named for, for the Refs
// trailer, or empty when the branch names none or cannot be read.
func (s *server) currentIssueKey() string {
	branch, err := s.deps.Branch()
	if err != nil {
		return ""
	}

	key, _ := convention.IssueKey(branch.Name, s.config().Jira.Project)

	return key
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
