// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errPushFailed carries the push's own output, so the caller learns why it did
// not reach the remote. Unlike errGitRefused, whose words stay off the wire,
// the output is forwarded: the reason is in it — a ref the remote rejected, a
// hook's refusal — and nothing else would tell it. git names the remote around
// that reason, so its URLs and user@host:path addresses are taken out first; a
// bare host git prints is kept.
var errPushFailed = errors.New("the push failed")

// pushNotStarted answers a push that never ran. Its cause stays off the wire:
// what git says about its repository can name where that is on disk.
const pushNotStarted = "the push could not be started; push from a terminal to see why"

// Push publishes the checked-out branch to its remote, setting upstream if it
// has none — the first outward step toward a pull request. There is nothing to
// push, so a 409, when the tree is not on a branch or the branch is not ahead of
// its upstream on the push remote; a push that fails is a 422 carrying its
// output with its URLs and user@host:path addresses taken out, one that cannot
// start a 422 saying how to see why, and a branch that cannot be read is
// answered by fault. On success it returns the branch as published.
func (s *server) Push(_ context.Context, _ api.PushRequestObject) (api.PushResponseObject, error) {
	if s.deps.Push == nil || s.deps.Branch == nil {
		return pushUnprocessable("pushing is not available"), nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		body, code := s.fault(err)

		return api.PushdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	if nothingToPush(branch) {
		return api.Push409ApplicationProblemPlusJSONResponse(problem(api.Conflict, "there is nothing to push")), nil
	}

	err = loop.Push(s.deps.Push, branch.Name)
	if err != nil {
		return pushUnprocessable(pushFailure(err)), nil
	}

	return api.Push200JSONResponse(branchDTO(s.branchAfter(branch))), nil
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
// output when it ran and failed, and how to see why when it could not start.
func pushFailure(err error) string {
	if failed, ok := errors.AsType[loop.PushFailedError](err); ok {
		return fmt.Errorf("%w:\n%s", errPushFailed, strings.Join(withoutAddresses(failed.Output), "\n")).Error()
	}

	return pushNotStarted
}

// addressPlaceholder stands where an address was taken out of a push's output.
const addressPlaceholder = "<address>"

// withoutAddresses is a push's output with every URL and user@host:path address
// in it replaced and every line kept, so the reason between the lines naming
// the remote survives.
func withoutAddresses(lines []string) []string {
	address := remoteAddress()

	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		kept = append(kept, address.ReplaceAllLiteralString(line, addressPlaceholder))
	}

	return kept
}

// remoteAddress matches an address as git prints a remote's: a URL, or an ssh
// remote's scp-style user@host:path. A ref name cannot hold a colon, so a ref
// with an at sign in it is never taken for one.
func remoteAddress() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s'"]+|[^\s'"@]+@[^\s'"@:/]+:[^\s'"]+`)
}

// pushUnprocessable is the 422 response for a push the server will not make.
func pushUnprocessable(message string) api.Push422ApplicationProblemPlusJSONResponse {
	return api.Push422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}
