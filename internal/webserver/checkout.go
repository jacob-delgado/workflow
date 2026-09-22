// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errDirtyTree refuses a checkout that would carry uncommitted work onto another
// branch, the same guard the terminal interface's task switcher applies.
var errDirtyTree = errors.New("the working tree has uncommitted changes; commit or stash them before switching")

// Checkout switches the working tree to the requested branch. It refuses a dirty
// tree with 409 so a switch never carries work in progress onto another branch,
// and answers a branch it cannot check out with 422. On success it returns the
// branch now in effect; the event stream re-pushes the rest of the state.
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
	case errors.Is(err, errDirtyTree):
		return api.Checkout409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errDirtyTree.Error())), nil
	default:
		return unprocessable("the branch could not be checked out"), nil
	}
}

// switchTo refuses a dirty working tree, then checks out name and returns the
// branch now in effect.
func (s *server) switchTo(name string) (gitrepo.Branch, error) {
	dirty, err := s.workingTreeDirty()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	if dirty {
		return gitrepo.Branch{}, errDirtyTree
	}

	err = s.deps.Checkout(name)
	if err != nil {
		return gitrepo.Branch{}, fmt.Errorf("checking out %s: %w", name, err)
	}

	return s.deps.Branch()
}

// workingTreeDirty reports whether the tree carries any change, so a switch can
// be refused before it moves uncommitted work onto another branch. A missing
// changes seam reads as clean rather than blocking the switch.
func (s *server) workingTreeDirty() (bool, error) {
	if s.deps.Changes == nil {
		return false, nil
	}

	changes, err := s.deps.Changes()
	if err != nil {
		return false, err
	}

	return len(changes) > 0, nil
}

// unprocessable is the 422 response for a checkout the server will not attempt.
func unprocessable(message string) api.Checkout422ApplicationProblemPlusJSONResponse {
	return api.Checkout422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}
