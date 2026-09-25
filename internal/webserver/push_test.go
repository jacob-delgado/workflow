// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// doPush posts a push against a server over deps. Push takes no body.
func doPush(t *testing.T, deps webserver.Deps) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/push", "")
}

func TestPushPublishesTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' branch is ahead with commits and has no upstream, so it can push.
	var pushed string

	deps := filledDeps()
	deps.Push = func(branch string) (proc.Output, error) {
		pushed = branch

		return fakeOutput(nil, nil), nil
	}

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if pushed != testBranchName {
		t.Errorf("pushed %q, want the current branch %q", pushed, testBranchName)
	}
}

func TestPushRefusesWhenThereIsNothingToPush(t *testing.T) {
	t.Parallel()

	commit := []gitrepo.Commit{{Hash: testCommitHash, Subject: "feat: done"}}
	cases := map[string]gitrepo.Branch{
		"already up to date": {
			Name: testBranchName, Upstream: "origin/" + testBranchName, PushRemote: gitrepo.DefaultRemote,
			Ahead: 0, Commits: commit,
		},
		// A detached HEAD is not on a branch, so there is nothing to push — and it
		// must not reach `git push origin ""`.
		"detached HEAD": {Name: "", Detached: true, Commits: commit},
	}

	for name, branch := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			called := false
			deps := filledDeps()
			deps.Branch = func() (gitrepo.Branch, error) { return branch, nil }
			deps.Push = func(string) (proc.Output, error) {
				called = true

				return fakeOutput(nil, nil), nil
			}

			// Act
			recorder := doPush(t, deps)

			// Assert
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409 when there is nothing to push", recorder.Code)
			}

			if called {
				t.Error("pushed when there was nothing to push")
			}
		})
	}
}

func TestPushReportsAFailingPush(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Push = func(string) (proc.Output, error) {
		return fakeOutput([]string{"! [rejected] fix/PROJ-412"}, errSeam), nil
	}

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 when the push fails", recorder.Code)
	}

	message := decode[api.Problem](t, recorder).Detail
	if !strings.Contains(message, "the push failed") || !strings.Contains(message, "rejected") {
		t.Errorf("message = %q, want the failure and its output", message)
	}
}

func TestPushReportsAFailedPushAsAFailureNotAStart(t *testing.T) {
	t.Parallel()

	// Arrange
	// The push ran and was rejected, so the detail is the push's own output, not
	// a push that never started.
	deps := filledDeps()
	deps.Push = func(string) (proc.Output, error) {
		return fakeOutput([]string{"! [rejected] fix/PROJ-412"}, errSeam), nil
	}

	// Act
	recorder := doPush(t, deps)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; detail != "the push failed:\n! [rejected] fix/PROJ-412" {
		t.Errorf("detail = %q, want the failure and its output, not a failed start", detail)
	}
}

func TestPushReportsAFailedStart(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Push = func(string) (proc.Output, error) { return proc.Output{}, errSeam }

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the push cannot be started", recorder.Code)
	}
}

func TestPushReturnsThePublishedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// The push sets the upstream, so the branch read after it differs from the one
	// read before; the response must carry the published (after) branch.
	before := gitrepo.Branch{Name: testBranchName, Ahead: 1, Commits: []gitrepo.Commit{{Hash: "a", Subject: "x"}}}
	after := gitrepo.Branch{Name: testBranchName, Upstream: "origin/" + testBranchName, PushRemote: gitrepo.DefaultRemote}
	reads := 0

	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		reads++
		if reads == 1 {
			return before, nil
		}

		return after, nil
	}
	deps.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if branch := decode[api.Branch](t, recorder); branch.Upstream != "origin/"+testBranchName {
		t.Errorf("branch = %+v, want the published branch with its new upstream", branch)
	}
}

func TestPushSucceedsEvenIfTheRereadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The push reaches the remote, but the read that would confirm it fails; the
	// completed, outward push must not be reported as failed.
	before := gitrepo.Branch{Name: testBranchName, Ahead: 1, Commits: []gitrepo.Commit{{Hash: "a", Subject: "x"}}}
	reads := 0

	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		reads++
		if reads == 1 {
			return before, nil
		}

		return gitrepo.Branch{}, errSeam
	}
	deps.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the push succeeded despite the re-read failing", recorder.Code)
	}

	if branch := decode[api.Branch](t, recorder); branch.Name != testBranchName {
		t.Errorf("branch = %+v, want the pre-push branch as a best-effort result", branch)
	}
}

func TestPushReportsABranchReadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	deps.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the branch cannot be read", recorder.Code)
	}
}

func TestPushIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Pushing needs the push and branch-read seams; missing either means it is not
	// available. filledDeps leaves Push nil, so the branch case sets it.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no push seam": func(deps webserver.Deps) webserver.Deps { return deps },
		noBranchSeam: func(deps webserver.Deps) webserver.Deps {
			deps.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
			deps.Branch = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := doPush(t, mutate(filledDeps()))

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when pushing is not available", recorder.Code)
			}
		})
	}
}
