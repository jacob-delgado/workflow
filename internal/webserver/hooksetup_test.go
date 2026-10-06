// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// hookSetupAt is where the lefthook offer is read and taken.
const hookSetupAt = "/api/hooks/setup"

// oldHooks is a repository's own pre-commit hook, which lefthook does not
// manage: two plain commands.
func oldHooks() []hooks.GitHook {
	return []hooks.GitHook{{Name: "pre-commit", Script: "#!/bin/sh\ngo vet ./...\ngo test ./...\n"}}
}

// hookDeps is filledDeps with the hooks directory holding found, configured
// or not, and the configuration written kept in written.
func hookDeps(found []hooks.GitHook, configured bool, written *[]hooks.Generated) webserver.Deps {
	deps := filledDeps()
	deps.HookExisting = func() ([]hooks.GitHook, bool) { return found, configured }
	deps.HookWrite = func(generated hooks.Generated) error {
		*written = append(*written, generated)

		return nil
	}

	return deps
}

func TestHookSetupOffersTheConfigurationThatRunsTheOldHooks(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, hookDeps(oldHooks(), false, &[]hooks.Generated{}), config.Default())

	// Act
	recorder := get(t, handler, hookSetupAt)

	// Assert
	setup := decode[api.HookSetup](t, recorder)
	if !setup.Offered || len(setup.Hooks) != 1 || setup.Hooks[0].Name != "pre-commit" || setup.Hooks[0].Lines != 3 {
		t.Fatalf("setup = %+v, want the pre-commit hook of three lines offered", setup)
	}

	if setup.Config != hooks.Structured(oldHooks()).Config {
		t.Errorf("config = %q, want the terminal's structured configuration", setup.Config)
	}
}

func TestHookSetupOffersNothingOnceLefthookIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, hookDeps(oldHooks(), true, &[]hooks.Generated{}), config.Default())

	// Act
	setup := decode[api.HookSetup](t, get(t, handler, hookSetupAt))

	// Assert
	if setup.Offered || len(setup.Hooks) != 0 || setup.Config != "" {
		t.Errorf("setup = %+v, want nothing offered", setup)
	}
}

func TestSettingUpHooksWritesTheChosenConfiguration(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want hooks.Generated
	}{
		"as lefthook jobs":       {body: `{}`, want: hooks.Structured(oldHooks())},
		"every hook as a script": {body: `{"verbatim":true}`, want: hooks.Verbatim(oldHooks())},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var written []hooks.Generated

			handler := serve(t, hookDeps(oldHooks(), false, &written), config.Default())

			// Act
			recorder := send(t, handler, http.MethodPost, hookSetupAt, tt.body)

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
			}

			if len(written) != 1 || written[0].Config != tt.want.Config {
				t.Errorf("wrote %+v, want %+v", written, tt.want)
			}
		})
	}
}

func TestSettingUpHooksWithNothingOfferedWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var written []hooks.Generated

	handler := serve(t, hookDeps(nil, false, &written), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPost, hookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "nothing to set up")

	if len(written) != 0 {
		t.Errorf("wrote %v, want nothing", written)
	}
}

func TestSettingUpHooksThatFailsKeepsItsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := hookDeps(oldHooks(), false, &[]hooks.Generated{})
	deps.HookWrite = func(hooks.Generated) error { return fmt.Errorf("writing %s/lefthook.yml: %w", repoPath, errSeam) }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, hookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "lefthook.yml was not written")

	if strings.Contains(recorder.Body.String(), repoPath) {
		t.Errorf("the refusal %s names where the repository is", recorder.Body)
	}
}

func TestHookSetupWithNoRepositoryIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()

	// Act
	read := get(t, serve(t, deps, config.Default()), hookSetupAt)
	write := send(t, serve(t, deps, config.Default()), http.MethodPost, hookSetupAt, `{}`)

	// Assert
	assertProblem(t, read, http.StatusUnprocessableEntity, "not available")
	assertProblem(t, write, http.StatusUnprocessableEntity, "not available")
}

func TestTheStreamCountsTheHooksLefthookDoesNotManage(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured bool
		want       int
	}{
		"lefthook not configured": {configured: false, want: 1},
		"lefthook configured":     {configured: true, want: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := serve(t, hookDeps(oldHooks(), tt.configured, &[]hooks.Generated{}), config.Default())

			// Act
			snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

			// Assert
			if snap.HooksUnmanaged != tt.want {
				t.Errorf("hooks_unmanaged = %d, want %d", snap.HooksUnmanaged, tt.want)
			}
		})
	}
}
