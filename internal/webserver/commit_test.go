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

// fakeOutput is a finished command's output: the lines it wrote, already closed,
// and a Wait that reports how it exited. It lets a commit test stand in for the
// streaming process the seam returns.
func fakeOutput(lines []string, waitErr error) proc.Output {
	channel := make(chan string, len(lines))
	for _, line := range lines {
		channel <- line
	}

	close(channel)

	return proc.Output{
		Lines: channel,
		Wait:  func() error { return waitErr },
		Stop:  func() {},
	}
}

// doCommit posts a commit request body against a server over deps.
func doCommit(t *testing.T, deps webserver.Deps, body string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/commit", body)
}

func TestCommitCommitsStagedChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' one change is staged, so there is something to commit.
	var message string

	deps := filledDeps()
	deps.Commit = func(msg string) (proc.Output, error) {
		message = msg

		return fakeOutput(nil, nil), nil
	}

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	// The message is the Conventional subject plus a Refs trailer for the branch's
	// issue (fix/PROJ-412 → PROJ-412).
	if !strings.Contains(message, "fix: redact tokens") || !strings.Contains(message, "Refs: PROJ-412") {
		t.Errorf("committed message = %q, want the subject and a Refs trailer", message)
	}
}

func TestCommitBuildsAFullMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	var message string

	deps := filledDeps()
	deps.Commit = func(msg string) (proc.Output, error) {
		message = msg

		return fakeOutput(nil, nil), nil
	}

	// Act
	// The scope, body, and breaking marker all feed into the assembled message.
	recorder := doCommit(t, deps,
		`{"type":"feat","scope":"redact","subject":"mask tokens","body":"Longer reasoning.","breaking":true}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(message, "feat(redact)!: mask tokens") || !strings.Contains(message, "Longer reasoning.") {
		t.Errorf("message = %q, want the scope, breaking marker, and body", message)
	}
}

func TestCommitRefusesWhenNothingIsStaged(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, nil }
	deps.Commit = func(string) (proc.Output, error) {
		called = true

		return fakeOutput(nil, nil), nil
	}

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 when nothing is staged", recorder.Code)
	}

	if called {
		t.Error("committed with nothing staged")
	}

	if failure := decode[api.Error](t, recorder); failure.Code != api.Conflict {
		t.Errorf("code = %q, want conflict", failure.Code)
	}
}

func TestCommitRejectsAnInvalidMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := filledDeps()
	deps.Commit = func(string) (proc.Output, error) {
		called = true

		return fakeOutput(nil, nil), nil
	}

	// Act
	// "nope" is not a Conventional Commit type.
	recorder := doCommit(t, deps, `{"type":"nope","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for an invalid message", recorder.Code)
	}

	if called {
		t.Error("committed despite an invalid message")
	}
}

func TestCommitReportsAFailingCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	// A hook rejects the commit: the exit is a failure and its output explains why.
	deps := filledDeps()
	deps.Commit = func(string) (proc.Output, error) {
		return fakeOutput([]string{"lint: trailing whitespace"}, errSeam), nil
	}

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 when the commit fails", recorder.Code)
	}

	message := decode[api.Error](t, recorder).Message

	if !strings.Contains(message, "the commit failed") || !strings.Contains(message, "trailing whitespace") {
		t.Errorf("message = %q, want the failure and its output", message)
	}
}

func TestCommitReportsAChangesReadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }
	deps.Commit = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the staged state cannot be read", recorder.Code)
	}
}

func TestCommitReportsAFailedStart(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit cannot even be started.
	deps := filledDeps()
	deps.Commit = func(string) (proc.Output, error) { return proc.Output{}, errSeam }

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the commit cannot be started", recorder.Code)
	}
}

func TestCommitReportsWhenTheBranchCannotBeReadAfter(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit runs, but the branch read that would confirm it fails.
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	deps.Commit = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	// Act
	recorder := doCommit(t, deps, `{"type":"fix","subject":"redact tokens"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the branch cannot be read", recorder.Code)
	}
}

func TestCommitIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Committing needs the commit, changes, and branch-read seams; missing any one
	// means it is not available. filledDeps leaves Commit nil, so the other cases
	// set it to isolate the seam under test.
	withCommit := func(deps webserver.Deps) webserver.Deps {
		deps.Commit = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

		return deps
	}

	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no commit seam": func(deps webserver.Deps) webserver.Deps { return deps },
		"no changes seam": func(deps webserver.Deps) webserver.Deps {
			deps = withCommit(deps)
			deps.Changes = nil

			return deps
		},
		noBranchSeam: func(deps webserver.Deps) webserver.Deps {
			deps = withCommit(deps)
			deps.Branch = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := doCommit(t, mutate(filledDeps()), `{"type":"fix","subject":"redact tokens"}`)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when committing is not available", recorder.Code)
			}
		})
	}
}
