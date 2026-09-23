// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errDirtyTree is the web's words for loop.ErrDirtyTree: a checkout refused for
// the uncommitted work it would carry onto another branch, by the same guard the
// terminal interface's task switcher applies.
var errDirtyTree = errors.New("the working tree has uncommitted changes; commit or stash them before switching")

// errSwitchRefused is git declining the switch. Its own words stay off the
// wire: in a partial clone a switch fetches the files the branch needs, and a
// fetch that fails names the remote.
var errSwitchRefused = errors.New("git refused the switch")

// Checkout switches the working tree to the requested branch. It refuses a dirty
// tree with 409 so a switch never carries work in progress onto another branch,
// and answers a branch it cannot check out with 422 — saying, when git refused,
// how to see git's reason. On success it returns the branch now in effect; the
// event stream re-pushes the rest of the state.
func (s *server) Checkout(
	_ context.Context, request api.CheckoutRequestObject,
) (api.CheckoutResponseObject, error) {
	if request.Body == nil || request.Body.Branch == "" {
		return unprocessable("a branch is required"), nil
	}

	if s.deps.Checkout == nil || s.deps.Branch == nil {
		return unprocessable("checking out is not available"), nil
	}

	branch, err := s.switchTo(request.Body.Branch)

	switch {
	case err == nil:
		return api.Checkout200JSONResponse(branchDTO(branch)), nil
	case errors.Is(err, loop.ErrDirtyTree):
		return api.Checkout409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errDirtyTree.Error())), nil
	case errors.Is(err, errSwitchRefused):
		return unprocessable("git would not switch to " + request.Body.Branch +
			"; switch from a terminal to see git's reason"), nil
	default:
		return unprocessable("the branch could not be checked out; try again, or switch from a terminal to see why"), nil
	}
}

// switchTo refuses a dirty working tree, then checks out name and returns the
// branch now in effect.
func (s *server) switchTo(name string) (gitrepo.Branch, error) {
	err := s.refuseADirtyTree()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	err = s.deps.Checkout(name)
	if err != nil {
		return gitrepo.Branch{}, fmt.Errorf("%w: checking out %s: %w", errSwitchRefused, name, err)
	}

	return s.deps.Branch()
}

// refuseADirtyTree refuses a tree that carries any change, so a switch never
// moves uncommitted work onto another branch. A missing changes seam reads as
// clean rather than blocking the switch.
func (s *server) refuseADirtyTree() error {
	if s.deps.Changes == nil {
		return nil
	}

	changes, err := s.deps.Changes()
	if err != nil {
		return err
	}

	return loop.RefuseDirty(changes)
}

// unprocessable is the 422 response for a checkout the server will not attempt.
func unprocessable(message string) api.Checkout422ApplicationProblemPlusJSONResponse {
	return api.Checkout422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}
