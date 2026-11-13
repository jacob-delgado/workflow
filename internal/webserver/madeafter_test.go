// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// madeSince makes deps' branch hold a commit made after its pull request
// merged, pushed (P005).
func madeSince(deps webserver.Deps) webserver.Deps {
	deps.Git.MadeAfter = func(context.Context, forge.PullRequest) (gitrepo.AfterMerge, error) {
		return gitrepo.AfterMerge{Latest: []gitrepo.Commit{{Hash: "c0ffee1", Subject: "feat: after the merge"}}, Own: 1}, nil
	}

	return deps
}

// unfetchedHost is the remote's host a fetch's failure names, which stays
// off the wire.
const unfetchedHost = "git.internal.example"

// unfetched makes deps' branch one whose pull request merged at a commit this
// clone lacks and could not fetch (Q27), failing in git's words, which name
// the remote.
func unfetched(deps webserver.Deps) webserver.Deps {
	deps.Git.MadeAfter = func(context.Context, forge.PullRequest) (gitrepo.AfterMerge, error) {
		return gitrepo.AfterMerge{}, fmt.Errorf("the commit it merged at, c0ffee2, %w: "+
			"fetching refs/pull/7/head from origin: fatal: unable to access 'https://"+unfetchedHost+"/a/b.git/'",
			gitrepo.ErrNotFetched)
	}

	return deps
}

func TestGetReviewLooksNoFurtherAtABranchRefusedAsItStands(t *testing.T) {
	t.Parallel()

	// Arrange
	// Committed to since it was pushed: refused as it stands, so nothing is
	// fetched to count what was made after the merge.
	var asked atomic.Int32

	deps := merged(webtest.FilledDeps())
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: webtest.TestBranchName, Base: webtest.TestBase, Head: committedHead}, nil
	}
	deps.Git.MadeAfter = func(context.Context, forge.PullRequest) (gitrepo.AfterMerge, error) {
		asked.Add(1)

		return gitrepo.AfterMerge{}, nil
	}

	// Act
	webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/review")

	// Assert
	if n := asked.Load(); n != 0 {
		t.Errorf("read what was made after the merge %d times, want none for a branch refused already", n)
	}
}

// errLogUnread is git failing to read what was made after the merge.
var errLogUnread = errors.New("fatal: bad object")

func TestFinishTellsAReadOfWhatWasMadeAfterTheMergeThatFailedFromARefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	// Not a branch in the way of a finish, so not a conflict: a git read
	// that failed, which fault classifies and hands to Unexpected.
	writes := &reviewWrites{}
	deps := merged(reviewDeps(writes))
	deps.Git.MadeAfter = func(context.Context, forge.PullRequest) (gitrepo.AfterMerge, error) {
		return gitrepo.AfterMerge{}, errLogUnread
	}

	var noted atomic.Int32

	deps.Unexpected = func(_ config.Config, err error) {
		if errors.Is(err, errLogUnread) {
			noted.Add(1)
		}
	}

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, finishAt, "")

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, "could not be completed")

	if writes.sent() != 0 || noted.Load() != 1 {
		t.Errorf("writes = %+v, git's failure heard %d times; want no write, and it heard once", writes, noted.Load())
	}
}
