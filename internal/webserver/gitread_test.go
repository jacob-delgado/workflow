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
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// gitReadWrite is a write that reads the repository before it changes it: how
// that read is made to fail, and the request that meets it.
type gitReadWrite struct {
	failRead   func(deps *webserver.Deps, err error)
	path, body string
}

// gitReadWrites are the writes that each fail their own first read: the
// branch for a push and a link, the working tree for a commit and a checkout,
// and the branch list for a new branch.
func gitReadWrites() map[string]gitReadWrite {
	failBranch := func(deps *webserver.Deps, err error) {
		deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, err }
	}
	failChanges := func(deps *webserver.Deps, err error) {
		deps.Changes = func() ([]gitrepo.Change, error) { return nil, err }
	}
	failBranches := func(deps *webserver.Deps, err error) {
		deps.Branches = func() ([]string, error) { return nil, err }
	}

	return map[string]gitReadWrite{
		"a push":       {failRead: failBranch, path: "/api/push"},
		"a link":       {failRead: failBranch, path: linkPath},
		"a commit":     {failRead: failChanges, path: "/api/commit", body: `{"type":"fix","subject":"redact tokens"}`},
		"a checkout":   {failRead: failChanges, path: "/api/checkout", body: `{"branch":"` + targetBranch + `"}`},
		"a new branch": {failRead: failBranches, path: "/api/branches", body: `{"issue_key":"` + startIssue + `"}`},
	}
}

// writesWired is filledDeps with every git write wired to count the writes
// that ran, so a request answered before its write can be told from one that
// went ahead.
func writesWired(ran *int) webserver.Deps {
	deps := filledDeps()
	deps.Push = func(string) (proc.Output, error) {
		*ran++

		return fakeOutput(nil, nil), nil
	}
	deps.Commit = deps.Push
	deps.Checkout = func(string) error {
		*ran++

		return nil
	}

	checkout := deps.Checkout
	deps.CreateBranch = func(name, _ string) error { return checkout(name) }
	deps.LinkPullRequest = func(_ jira.Key, url, _ string) error { return checkout(url) }

	return deps
}

// notARepository is what the answer to a server outside a work tree tells the
// caller to do.
const notARepository = "start workflow --web from a repository's work tree"

// gitReadAnswer is the problem a request whose read failed is answered with:
// its status, its code, and a phrase its curated detail carries.
type gitReadAnswer struct {
	status int
	code   api.ProblemCode
	detail string
}

// assertGitReadAnswer checks a write whose read failed was answered with want,
// ran no write, and named neither the read's own words nor where the
// repository is.
func assertGitReadAnswer(t *testing.T, recorder *httptest.ResponseRecorder, ran int, want gitReadAnswer) {
	t.Helper()

	failure := decode[api.Problem](t, recorder)
	if recorder.Code != want.status || failure.Code != want.code {
		t.Errorf("status = %d, code %q; want %d and %q", recorder.Code, failure.Code, want.status, want.code)
	}

	if !strings.Contains(failure.Detail, want.detail) {
		t.Errorf("detail = %q, want it to say %q", failure.Detail, want.detail)
	}

	if strings.Contains(failure.Detail, errSeam.Error()) || strings.Contains(failure.Detail, repoPath) {
		t.Errorf("detail = %q, names the read's own words or the repository's path", failure.Detail)
	}

	if ran != 0 {
		t.Errorf("%d writes ran after the read failed, want none", ran)
	}
}

func TestAFailedGitReadAnswersEveryWriteAlike(t *testing.T) {
	t.Parallel()

	// A read that fails says where the repository is, as gitrepo words it.
	readErr := fmt.Errorf("reading the current branch of %s: %w", repoPath, errSeam)

	for name, write := range gitReadWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ran := 0
			deps := writesWired(&ran)
			write.failRead(&deps, readErr)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, write.path, write.body)

			// Assert
			assertGitReadAnswer(t, recorder, ran, gitReadAnswer{http.StatusInternalServerError, api.Internal, tryAgain})
		})
	}
}

func TestAServerOutsideARepositoryAnswersEveryWriteWithAConflict(t *testing.T) {
	t.Parallel()

	// Outside a work tree gitrepo says so, naming the directory it looked in.
	readErr := fmt.Errorf("%w: %s", gitrepo.ErrNotARepository, repoPath)

	for name, write := range gitReadWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ran := 0
			deps := writesWired(&ran)
			write.failRead(&deps, readErr)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, write.path, write.body)

			// Assert
			assertGitReadAnswer(t, recorder, ran, gitReadAnswer{http.StatusConflict, api.Conflict, notARepository})
		})
	}
}

func TestAServerOutsideARepositoryAnswersItsReadsWithAConflict(t *testing.T) {
	t.Parallel()

	readErr := fmt.Errorf("%w: %s", gitrepo.ErrNotARepository, repoPath)
	cases := map[string]string{
		"the branch":  "/api/branch",
		"the changes": "/api/changes",
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, readErr }
			deps.Changes = func() ([]gitrepo.Change, error) { return nil, readErr }

			// Act
			recorder := get(t, serve(t, deps, config.Default()), path)

			// Assert
			assertGitReadAnswer(t, recorder, 0, gitReadAnswer{http.StatusConflict, api.Conflict, notARepository})
		})
	}
}
