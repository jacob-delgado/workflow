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
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Why a staging request was not carried out.
var (
	// errNoStagingTarget refuses a request that names neither a changed file nor
	// all of them, or names both.
	errNoStagingTarget = errors.New("name a changed file's path, or all — one or the other")
	// errNotAChange refuses a path the working tree's changes do not list.
	errNotAChange = errors.New("no change is at that path")
	// errGitRefused is git declining to move a change. Its own words stay off the
	// wire: unstaging in a partial clone fetches what the index needs, and a
	// fetch that fails names the remote.
	errGitRefused = errors.New("git refused")
)

// stagingTarget is what a staging request names: one changed file, by the path
// the working tree's changes list it under, or all of them.
type stagingTarget struct {
	path string
	all  bool
}

// words names the target in a sentence.
func (t stagingTarget) words() string {
	if t.all {
		return "every change"
	}

	return t.path
}

// stagingDirection is one way changes move between the work tree and the
// index: the verb, the seam that moves one change, and loop's rule for all.
type stagingDirection struct {
	verb string
	one  func(gitrepo.Change) error
	all  func([]gitrepo.Change, func(gitrepo.Change) error) error
}

// Stage takes the named change into the index — the terminal's space — or,
// with all, every change the index does not hold yet, by the rule the
// terminal's "stage all" follows. It answers the working tree as it now stands.
func (s *server) Stage(_ context.Context, request api.StageRequestObject) (api.StageResponseObject, error) {
	direction := stagingDirection{verb: "stage", one: s.deps.Stage, all: loop.StageAll}

	target, changes, err := s.moveChanges(*request.Body, direction)
	if err != nil {
		failure := stagingProblem(err, direction.verb, target)

		return api.StagedefaultApplicationProblemPlusJSONResponse{Body: failure, StatusCode: failure.Status}, nil
	}

	return api.Stage200JSONResponse(changesDTO(changes)), nil
}

// Unstage takes the named change out of the index, leaving the work tree as it
// is, or with all every staged change. It answers the working tree as it now
// stands.
func (s *server) Unstage(_ context.Context, request api.UnstageRequestObject) (api.UnstageResponseObject, error) {
	direction := stagingDirection{verb: "unstage", one: s.deps.Unstage, all: loop.UnstageAll}

	target, changes, err := s.moveChanges(*request.Body, direction)
	if err != nil {
		failure := stagingProblem(err, direction.verb, target)

		return api.UnstagedefaultApplicationProblemPlusJSONResponse{Body: failure, StatusCode: failure.Status}, nil
	}

	return api.Unstage200JSONResponse(changesDTO(changes)), nil
}

// moveChanges moves what the request names in direction and returns the
// working tree as it then stands. The named file is found among the changes
// the server reads for itself, and it is that change — never the request's own
// path — that reaches git. One request moves changes at a time, the rest
// waiting their turn, and each reads the tree once the one before it is done.
func (s *server) moveChanges(
	request api.StagingRequest, direction stagingDirection,
) (stagingTarget, []gitrepo.Change, error) {
	target := stagingTarget{path: orZero(request.Path), all: orZero(request.All)}

	if direction.one == nil || s.deps.Changes == nil {
		return target, nil, loop.ErrStagingUnavailable
	}

	if target.all == (target.path != "") {
		return target, nil, errNoStagingTarget
	}

	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	before, err := s.deps.Changes()
	if err != nil {
		return target, nil, fmt.Errorf("reading the changes: %w", err)
	}

	err = move(before, target, direction)
	if err != nil {
		return target, nil, err
	}

	return target, s.changesAfter(before), nil
}

// move hands git the target's changes in direction, from the changes read.
func move(changes []gitrepo.Change, target stagingTarget, direction stagingDirection) error {
	var err error

	if target.all {
		err = direction.all(changes, direction.one)
	} else {
		index := slices.IndexFunc(changes, func(change gitrepo.Change) bool { return change.Path == target.path })
		if index < 0 {
			return errNotAChange
		}

		err = direction.one(changes[index])
	}

	if err != nil {
		return fmt.Errorf("%w: %w", errGitRefused, err)
	}

	return nil
}

// changesAfter re-reads the working tree once a change has moved. A re-read
// that fails does not undo the move, so the tree as read before is answered
// rather than a completed write reported as failed; the event stream brings
// the rest.
func (s *server) changesAfter(before []gitrepo.Change) []gitrepo.Change {
	after, err := s.deps.Changes()
	if err != nil {
		return before
	}

	return after
}

// stagingProblem words a staging request that was not carried out: a 404 for a
// path the changes do not list, a 422 that says what to do for one the server
// will not or git would not move, and the curated fault for a tree it could not
// read.
func stagingProblem(err error, verb string, target stagingTarget) api.Problem {
	switch {
	case errors.Is(err, loop.ErrStagingUnavailable):
		return problem(api.Unprocessable, "staging is not available")
	case errors.Is(err, errNoStagingTarget):
		return problem(api.Unprocessable, errNoStagingTarget.Error())
	case errors.Is(err, errNotAChange):
		return problem(api.NotFound, "the working tree lists no change at "+target.path)
	case errors.Is(err, errGitRefused):
		return problem(api.Unprocessable,
			"git would not "+verb+" "+target.words()+"; "+verb+" from a terminal to see git's reason")
	default:
		failure, _ := fault(err)

		return failure
	}
}
