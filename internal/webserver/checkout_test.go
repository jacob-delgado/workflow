// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

const targetBranch = "feat/PROJ-500-metrics"

// doCheckout posts a checkout of branch against a server built over deps.
func doCheckout(t *testing.T, deps webserver.Deps, branch string) *httptest.ResponseRecorder {
	t.Helper()

	body := `{"branch":"` + branch + `"}`

	return send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/checkout", body)
}

// cleanDeps is filledDeps with a clean working tree and a checkout that records
// the branch it was asked to switch to, so a test can assert the switch happened.
func cleanDeps(t *testing.T, switched *string) webserver.Deps {
	t.Helper()

	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, nil }
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: targetBranch}, nil }
	deps.Checkout = func(name string) error {
		*switched = name

		return nil
	}

	return deps
}

func TestCheckoutSwitchesToTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	var switched string

	deps := cleanDeps(t, &switched)

	// Act
	recorder := doCheckout(t, deps, targetBranch)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if switched != targetBranch {
		t.Errorf("checked out %q, want %q", switched, targetBranch)
	}

	if branch := decode[api.Branch](t, recorder); branch.Name != targetBranch {
		t.Errorf("branch = %+v, want the switched-to branch", branch)
	}
}

func TestCheckoutRefusesADirtyTree(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' Changes reports a modified file, so the tree is dirty.
	called := false
	deps := filledDeps()
	deps.Checkout = func(string) error {
		called = true

		return nil
	}

	// Act
	recorder := doCheckout(t, deps, targetBranch)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a dirty tree", recorder.Code)
	}

	if called {
		t.Error("checkout ran despite a dirty tree, want it refused before switching")
	}

	if failure := decode[api.Error](t, recorder); failure.Code != api.Conflict {
		t.Errorf("code = %q, want conflict", failure.Code)
	}
}

func TestCheckoutReportsAFailedSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, nil }
	deps.Checkout = func(string) error { return errSeam }

	// Act
	recorder := doCheckout(t, deps, "nope")

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for a checkout that fails", recorder.Code)
	}

	if failure := decode[api.Error](t, recorder); failure.Code != api.Unprocessable {
		t.Errorf("code = %q, want unprocessable", failure.Code)
	}
}

func TestCheckoutRejectsAnEmptyBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	var switched string

	deps := cleanDeps(t, &switched)

	// Act
	recorder := doCheckout(t, deps, "")

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for an empty branch", recorder.Code)
	}

	if switched != "" {
		t.Errorf("checked out %q, want no switch for an empty branch", switched)
	}
}

func TestCheckoutIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Checking out needs both the checkout and the branch-read seams; missing
	// either means it is not available.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no checkout seam": func(deps webserver.Deps) webserver.Deps {
			deps.Checkout = nil

			return deps
		},
		noBranchSeam: func(deps webserver.Deps) webserver.Deps {
			deps.Branch = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := mutate(filledDeps())
			deps.Changes = func() ([]gitrepo.Change, error) { return nil, nil }

			// Act
			recorder := doCheckout(t, deps, targetBranch)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when checking out is not available", recorder.Code)
			}
		})
	}
}

func TestCheckoutTreatsAMissingChangesSeamAsClean(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no changes seam the tree cannot be read as dirty, so the switch goes
	// ahead rather than being blocked.
	var switched string

	deps := cleanDeps(t, &switched)
	deps.Changes = nil

	// Act
	recorder := doCheckout(t, deps, targetBranch)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when the tree cannot be read as dirty", recorder.Code)
	}

	if switched != targetBranch {
		t.Errorf("checked out %q, want the switch to proceed", switched)
	}
}

func TestCheckoutReportsAChangesReadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// If the working tree cannot be read, the switch is refused rather than risked.
	called := false
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }
	deps.Checkout = func(string) error {
		called = true

		return nil
	}

	// Act
	recorder := doCheckout(t, deps, targetBranch)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 when the working tree cannot be read", recorder.Code)
	}

	if called {
		t.Error("checkout ran despite an unreadable working tree")
	}
}
