// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errUnreadableHere is lefthook unable to read the configuration, in words
// that name the repository.
var errUnreadableHere = errors.New("reading lefthook's configuration: lefthook: exit status 1: " +
	"couldn't load config: open " + repoPath + "/lefthook.yml: permission denied")

// listedDeps is filledDeps with lefthook configuring what list answers.
func listedDeps(list func(context.Context) (hooks.Configured, error)) webserver.Deps {
	deps := filledDeps()
	deps.Hooks.Configured = list

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
	if !setup.Offered || len(setup.Hooks) != 1 || setup.Hooks[0].Name != preCommitHook || setup.Hooks[0].Lines != 3 {
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
	deps.Hooks.Write = func(context.Context, hooks.Generated) error {
		return fmt.Errorf("writing %s/lefthook.yml: %w", repoPath, errSeam)
	}

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

func TestSettingUpHooksWithNoWayToWriteIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := hookDeps(oldHooks(), false, &[]hooks.Generated{})
	deps.Hooks.Write = nil

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, hookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestListHooksSplitsThemByWhetherARunByHandStandsInForGits(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, listedDeps(func(context.Context) (hooks.Configured, error) {
		return configuredHooks(), nil
	}), config.Default())

	// Act
	listed := decode[api.ConfiguredHooks](t, get(t, handler, hooksAt))

	// Assert
	if !slices.Equal(listed.Runnable, []string{preCommitHook, prePushHook}) {
		t.Errorf("runnable = %q, want pre-commit and pre-push", listed.Runnable)
	}

	want := []api.GitOnlyHook{{Name: commitMsgHook, Reason: commitMsgReason}}
	if !slices.Equal(listed.GitOnly, want) {
		t.Errorf("git_only = %+v, want %+v", listed.GitOnly, want)
	}
}

func TestListHooksAnswersEmptyListsWhereNoLefthookIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, listedDeps(func(context.Context) (hooks.Configured, error) {
		return hooks.Configured{}, nil
	}), config.Default())

	// Act
	recorder := get(t, handler, hooksAt)

	// Assert
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"runnable":[]`) ||
		!strings.Contains(body, `"git_only":[]`) {
		t.Errorf("%d %s, want both lists empty, not null", recorder.Code, body)
	}
}

func TestListHooksWithoutLefthookIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, listedDeps(nil), config.Default())

	// Act
	recorder := get(t, handler, hooksAt)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "lefthook is not installed")
}

func TestListHooksThatCannotReadTheConfigurationKeepsLefthooksWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, heard := hearing(listedDeps(func(context.Context) (hooks.Configured, error) {
		return hooks.Configured{}, errUnreadableHere
	}))
	handler := serve(t, deps, config.Default())

	// Act
	recorder := get(t, handler, hooksAt)

	// Assert
	failure := assertProblem(t, recorder, http.StatusUnprocessableEntity, "could not be read")
	if strings.Contains(failure.Detail, repoPath) {
		t.Errorf("detail = %q, want lefthook's words, which name the repository, kept off the wire", failure.Detail)
	}

	if len(*heard) != 1 || !errors.Is((*heard)[0], errUnreadableHere) {
		t.Errorf("Unexpected heard %v, want lefthook's refusal", *heard)
	}
}
