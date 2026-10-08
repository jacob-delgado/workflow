// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
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

// pushRoutes are the two requests that push the branch: the push itself, and
// the open, which pushes a branch its remote does not have yet first.
func pushRoutes() map[string]route {
	return map[string]route{
		"the push": {method: http.MethodPost, path: "/api/push"},
		"the open": {method: http.MethodPost, path: "/api/pull-request", body: openRequestBody},
	}
}

func TestPushPublishesTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' branch is ahead with commits and has no upstream, so it can push.
	var pushed string

	deps := filledDeps()
	deps.Git.Push = func(branch string) (proc.Output, error) {
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

func TestPushSendsABranchWhoseUpstreamIsOffItsPushRemote(t *testing.T) {
	t.Parallel()

	// Arrange
	// remote.pushDefault sends the push to fork, which does not hold the branch
	// its upstream on origin is level with.
	const forkRemote = "fork"

	var pushed string

	deps := filledDeps()
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{
			Name: testBranchName, Upstream: "origin/" + testBranchName, PushRemote: forkRemote,
			Ahead: 0, Commits: []gitrepo.Commit{{Hash: testCommitHash, Subject: "feat: done"}},
		}, nil
	}
	deps.Git.Push = func(branch string) (proc.Output, error) {
		pushed = branch

		return fakeOutput(nil, nil), nil
	}

	// Act
	recorder := doPush(t, deps)

	// Assert
	if recorder.Code != http.StatusOK || pushed != testBranchName {
		t.Errorf("status = %d, pushed %q; want 200 with %q pushed", recorder.Code, pushed, testBranchName)
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
		// The base is what work merges into, not a branch to publish, however far
		// ahead of its upstream it is — the terminal withholds that push too.
		"the base branch": {
			Name: "main", Base: testBase, Upstream: testBase, PushRemote: gitrepo.DefaultRemote,
			Ahead: 1, Commits: commit,
		},
	}

	for name, branch := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			called := false
			deps := filledDeps()
			deps.Git.Branch = func() (gitrepo.Branch, error) { return branch, nil }
			deps.Git.Push = func(string) (proc.Output, error) {
				called = true

				return fakeOutput(nil, nil), nil
			}

			// Act
			recorder := doPush(t, deps)

			// Assert
			assertProblem(t, recorder, http.StatusConflict, "nothing to push")

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
	deps.Git.Push = func(string) (proc.Output, error) {
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
	deps.Git.Push = func(string) (proc.Output, error) {
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

	// The push cannot even be started, and the seam's words name where the
	// repository is; the answer says how to see why instead, whichever request
	// pushed.
	const want = "the push could not be started; push from a terminal to see why"

	for name, request := range pushRoutes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := openableDeps()
			deps.Git.Push = func(string) (proc.Output, error) {
				return proc.Output{}, fmt.Errorf("running git in %s: %w", repoPath, errSeam)
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), request.method, request.path, request.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != want {
				t.Errorf("status = %d, detail %q; want 422 saying %q", recorder.Code, failure.Detail, want)
			}
		})
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
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		reads++
		if reads == 1 {
			return before, nil
		}

		return after, nil
	}
	deps.Git.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

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
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		reads++
		if reads == 1 {
			return before, nil
		}

		return gitrepo.Branch{}, errSeam
	}
	deps.Git.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

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

func TestPushIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Pushing needs the push and branch-read seams; missing either means it is not
	// available. filledDeps leaves Push nil, so the branch case sets it.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no push seam": func(deps webserver.Deps) webserver.Deps { return deps },
		noBranchSeam: func(deps webserver.Deps) webserver.Deps {
			deps.Git.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
			deps.Git.Branch = nil

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

func TestAFailedPushKeepsGitsReasonButNotTheRemotesAddress(t *testing.T) {
	t.Parallel()

	// The push's output names the remote the way git prints it, around the
	// reason; the detail keeps every line with the address taken out,
	// whichever request pushed.
	remotes := map[string]string{
		"an https remote":         "https://" + forgeHost + "/acme/repo.git",
		"an scp-style ssh remote": "git@" + forgeHost + ":acme/repo.git",
		"an ssh remote":           "ssh://git@" + forgeHost + ":2222/acme/repo.git",
	}

	for routeName, request := range pushRoutes() {
		for remoteName, remote := range remotes {
			t.Run(routeName+", "+remoteName, func(t *testing.T) {
				t.Parallel()

				// Arrange
				output := []string{
					"To " + remote,
					" ! [rejected]        " + testBranchName + " -> " + testBranchName + " (fetch first)",
					"error: failed to push some refs to '" + remote + "'",
					"hint: Updates were rejected because the remote contains work that you do not",
				}
				deps := openableDeps()
				deps.Git.Push = func(string) (proc.Output, error) { return fakeOutput(output, errSeam), nil }

				// Act
				recorder := send(t, serve(t, deps, config.Default()), request.method, request.path, request.body)

				// Assert
				want := "the push failed:\n" +
					"To <address>\n" +
					" ! [rejected]        " + testBranchName + " -> " + testBranchName + " (fetch first)\n" +
					"error: failed to push some refs to '<address>'\n" +
					"hint: Updates were rejected because the remote contains work that you do not"

				failure := decode[api.Problem](t, recorder)
				if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != want {
					t.Errorf("status = %d, detail %q; want 422 saying %q", recorder.Code, failure.Detail, want)
				}
			})
		}
	}
}

func TestAFailedPushKeepsARefNamedWithAnAtSign(t *testing.T) {
	t.Parallel()

	// Arrange
	// A ref name may hold an at sign but never a colon, so it is not an ssh
	// remote's user@host:path and stays as git printed it.
	const refLine = " ! [rejected]        spike@home -> spike@home (fetch first)"

	deps := filledDeps()
	deps.Git.Push = func(string) (proc.Output, error) { return fakeOutput([]string{refLine}, errSeam), nil }

	// Act
	recorder := doPush(t, deps)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; detail != "the push failed:\n"+refLine {
		t.Errorf("detail = %q, want the ref line kept as git printed it", detail)
	}
}
