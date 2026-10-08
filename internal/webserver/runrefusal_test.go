// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A run with nothing to work on, with no seam to run it, or under a dry run is
// refused before any program starts.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestARunWithNothingToWorkOnIsRefusedBeforeItRuns(t *testing.T) {
	t.Parallel()

	onBase := func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: prBase, Base: testBase, Head: filledHead, PushRemote: gitrepo.DefaultRemote}, nil
	}
	pushed := func() (gitrepo.Branch, error) {
		return gitrepo.Branch{
			Name: testBranchName, Base: testBase, Upstream: gitrepo.DefaultRemote + "/" + testBranchName,
			PushRemote: gitrepo.DefaultRemote, Commits: []gitrepo.Commit{{Hash: filledHead, Subject: testCommitSubject}},
		}, nil
	}
	unstaged := func() ([]gitrepo.Change, error) {
		return []gitrepo.Change{{Path: "a.go", Staged: ' ', Unstaged: 'M'}}, nil
	}

	cases := map[string]struct {
		body    string
		branch  func() (gitrepo.Branch, error)
		changes func() ([]gitrepo.Change, error)
		status  int
		saying  string
	}{
		"a rebase on the base itself": {
			body: rebaseRun, branch: onBase, status: http.StatusConflict, saying: "nothing to rebase",
		},
		"an amend with nothing staged": {
			body: amendRun, changes: unstaged, status: http.StatusConflict, saying: "nothing to amend",
		},
		"an amend of a pushed commit": {
			body: amendRun, branch: pushed, status: http.StatusConflict, saying: "nothing to amend",
		},
		"a fixup of a commit not on the branch": {
			body: `{"kind":"fixup","commit":"fff9999"}`, status: http.StatusUnprocessableEntity,
			saying: "only a commit not yet pushed",
		},
		"a fixup naming no commit": {
			body: `{"kind":"fixup"}`, status: http.StatusUnprocessableEntity, saying: "only a commit not yet pushed",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			deps := runDeps(calls, passing)

			if tt.branch != nil {
				deps.Branch = tt.branch
			}

			if tt.changes != nil {
				deps.Changes = tt.changes
			}

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, tt.status, tt.saying)

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

func TestARunWhoseReadsFailHasNothingToWorkOn(t *testing.T) {
	t.Parallel()

	failingBranch := func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	failingChanges := func() ([]gitrepo.Change, error) { return nil, errSeam }

	cases := map[string]struct {
		body    string
		branch  func() (gitrepo.Branch, error)
		changes func() ([]gitrepo.Change, error)
	}{
		"a rebase whose branch cannot be read": {body: rebaseRun, branch: failingBranch},
		"an amend whose branch cannot be read": {body: amendRun, branch: failingBranch},
		"a fixup whose changes cannot be read": {body: `{"kind":"fixup","commit":"abc123"}`, changes: failingChanges},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			deps := runDeps(calls, passing)

			if tt.branch != nil {
				deps.Branch = tt.branch
			}

			if tt.changes != nil {
				deps.Changes = tt.changes
			}

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusConflict, "nothing to")

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

func TestARunThatIsNotWiredIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := runDeps(&runCalls{}, passing)
	deps.RunHook = nil

	// Act
	recorder := startRun(t, serve(t, deps, config.Default()), preCommitRun)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestEveryRunIsUnavailableWithoutItsSeam(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body  string
		unset func(*webserver.Deps)
	}{
		"a rebase":          {body: rebaseRun, unset: func(deps *webserver.Deps) { deps.Rebase = nil }},
		"an amend":          {body: amendRun, unset: func(deps *webserver.Deps) { deps.Amend = nil }},
		"a fixup":           {body: `{"kind":"fixup"}`, unset: func(deps *webserver.Deps) { deps.Fixup = nil }},
		"an amend, no tree": {body: amendRun, unset: func(deps *webserver.Deps) { deps.Changes = nil }},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := runDeps(&runCalls{}, passing)
			tt.unset(&deps)

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
		})
	}
}

func TestEveryRunWithoutABranchSeamIsNotAvailable(t *testing.T) {
	t.Parallel()

	for _, body := range []string{rebaseRun, amendRun} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := runDeps(&runCalls{}, passing)
			deps.Branch = nil

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), body)

			// Assert
			assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
		})
	}
}

func TestARunThatCannotStartKeepsGitsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := runDeps(&runCalls{}, passing)
	deps.Rebase = func(string) (proc.Output, error) {
		return proc.Output{}, fmt.Errorf("git -C %s rebase: %w", repoPath, errSeam)
	}

	// Act
	recorder := startRun(t, serve(t, deps, config.Default()), rebaseRun)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "could not be started")

	if strings.Contains(recorder.Body.String(), repoPath) {
		t.Errorf("the refusal %s names where the repository is", recorder.Body)
	}
}

func TestARunIsHeldBackUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := serveWith(t, runDeps(calls, passing), config.Default(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	recorder := startRun(t, handler, rebaseRun)

	// Assert
	if recorder.Code != http.StatusForbidden || len(calls.asked()) != 0 {
		t.Errorf("status = %d after %v, want 403 and nothing run", recorder.Code, calls.asked())
	}
}
