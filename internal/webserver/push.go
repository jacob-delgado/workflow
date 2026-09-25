// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errPushFailed carries the push's own output, so the caller learns why it did
// not reach the remote.
var errPushFailed = errors.New("the push failed")

// Push publishes the checked-out branch to its remote, setting upstream if it
// has none — the first outward step toward a pull request. There is nothing to
// push, so a 409, when the tree is not on a branch or the branch is not ahead of
// its upstream on the push remote; a push that fails is a 422 carrying the
// output. On success it returns the branch as published.
func (s *server) Push(_ context.Context, _ api.PushRequestObject) (api.PushResponseObject, error) {
	if s.deps.Push == nil || s.deps.Branch == nil {
		return pushUnprocessable("pushing is not available"), nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		//nolint:nilerr // the read failure is answered with a 422 response, not a returned error
		return pushUnprocessable("the branch could not be read"), nil
	}

	if nothingToPush(branch) {
		return api.Push409ApplicationProblemPlusJSONResponse(problem(api.Conflict, "there is nothing to push")), nil
	}

	err = loop.Push(s.deps.Push, branch.Name)
	if err != nil {
		return pushUnprocessable(pushFailure(err)), nil
	}

	return api.Push200JSONResponse(branchDTO(s.publishedBranch(branch))), nil
}

// nothingToPush reports whether the branch has nothing to send: the tree is not
// on a branch (a detached HEAD, which cannot be pushed), or the branch is not
// ahead of an upstream on the remote a push goes to. A branch with no upstream
// there can always be published, so its commit count — which the base may make
// unknowable — is not consulted.
func nothingToPush(branch gitrepo.Branch) bool {
	if branch.Name == "" {
		return true
	}

	return strings.HasPrefix(branch.Upstream, branch.PushRemote+"/") && branch.Ahead == 0
}

// pushFailure words a push that did not publish the branch: the push's own
// output when it ran and failed, and why it could not start otherwise.
func pushFailure(err error) string {
	if failed, ok := errors.AsType[loop.PushFailedError](err); ok {
		return fmt.Errorf("%w:\n%s", errPushFailed, strings.Join(failed.Output, "\n")).Error()
	}

	return fmt.Errorf("starting the push: %w", err).Error()
}

// publishedBranch re-reads the branch after a successful push, for the upstream
// the push set. A re-read that fails does not undo the push, so the pre-push
// branch is returned rather than reporting a completed, outward push as failed;
// the event stream refreshes the rest.
func (s *server) publishedBranch(before gitrepo.Branch) gitrepo.Branch {
	after, err := s.deps.Branch()
	if err != nil {
		return before
	}

	return after
}

// pushUnprocessable is the 422 response for a push the server will not make.
func pushUnprocessable(message string) api.Push422ApplicationProblemPlusJSONResponse {
	return api.Push422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}
