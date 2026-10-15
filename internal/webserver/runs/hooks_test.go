// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

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
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// errUnreadableHere is lefthook unable to read the configuration, in words
// that name the repository.
var errUnreadableHere = errors.New("reading lefthook's configuration: lefthook: exit status 1: " +
	"couldn't load config: open " + webtest.RepoPath + "/lefthook.yml: permission denied")

// listedDeps is webtest.FilledDeps with lefthook configuring what list answers.
func listedDeps(list func(context.Context) (hooks.Configured, error)) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Hooks.Configured = list

	return deps
}

func TestHookSetupOffersTheConfigurationThatRunsTheOldHooks(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, webtest.HookDeps(webtest.OldHooks(), false, &[]hooks.Generated{}), config.Default())

	// Act
	recorder := webtest.Get(t, handler, webtest.HookSetupAt)

	// Assert
	setup := webtest.Decode[api.HookSetup](t, recorder)
	if !setup.Offered || len(setup.Hooks) != 1 || setup.Hooks[0].Name != webtest.PreCommitHook ||
		setup.Hooks[0].Lines != 3 {
		t.Fatalf("setup = %+v, want the pre-commit hook of three lines offered", setup)
	}

	if setup.Config != hooks.Structured(webtest.OldHooks()).Config {
		t.Errorf("config = %q, want the terminal's structured configuration", setup.Config)
	}
}

func TestHookSetupOffersNothingOnceLefthookIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, webtest.HookDeps(webtest.OldHooks(), true, &[]hooks.Generated{}), config.Default())

	// Act
	setup := webtest.Decode[api.HookSetup](t, webtest.Get(t, handler, webtest.HookSetupAt))

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
		"as lefthook jobs":       {body: `{}`, want: hooks.Structured(webtest.OldHooks())},
		"every hook as a script": {body: `{"verbatim":true}`, want: hooks.Verbatim(webtest.OldHooks())},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var written []hooks.Generated

			handler := webtest.Serve(t, webtest.HookDeps(webtest.OldHooks(), false, &written), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, webtest.HookSetupAt, tt.body)

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

	handler := webtest.Serve(t, webtest.HookDeps(nil, false, &written), config.Default())

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, webtest.HookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "nothing to set up")

	if len(written) != 0 {
		t.Errorf("wrote %v, want nothing", written)
	}
}

func TestSettingUpHooksThatFailsKeepsItsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.HookDeps(webtest.OldHooks(), false, &[]hooks.Generated{})
	deps.Hooks.Write = func(context.Context, hooks.Generated) error {
		return fmt.Errorf("writing %s/lefthook.yml: %w", webtest.RepoPath, webtest.ErrSeam)
	}

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, webtest.HookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "lefthook.yml was not written")

	if strings.Contains(recorder.Body.String(), webtest.RepoPath) {
		t.Errorf("the refusal %s names where the repository is", recorder.Body)
	}
}

func TestHookSetupWithNoRepositoryIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()

	// Act
	read := webtest.Get(t, webtest.Serve(t, deps, config.Default()), webtest.HookSetupAt)
	write := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, webtest.HookSetupAt, `{}`)

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
			handler := webtest.Serve(t, webtest.HookDeps(webtest.OldHooks(), tt.configured, &[]hooks.Generated{}),
				config.Default())

			// Act
			snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

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
	deps := webtest.HookDeps(webtest.OldHooks(), false, &[]hooks.Generated{})
	deps.Hooks.Write = nil

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, webtest.HookSetupAt, `{}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestListHooksSplitsThemByWhetherARunByHandStandsInForGits(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, listedDeps(func(context.Context) (hooks.Configured, error) {
		return configuredHooks(), nil
	}), config.Default())

	// Act
	listed := webtest.Decode[api.ConfiguredHooks](t, webtest.Get(t, handler, webtest.HooksAt))

	// Assert
	if !slices.Equal(listed.Runnable, []string{webtest.PreCommitHook, webtest.PrePushHook}) {
		t.Errorf("runnable = %q, want pre-commit and pre-push", listed.Runnable)
	}

	want := []api.GitOnlyHook{{Name: webtest.CommitMsgHook, Reason: webtest.CommitMsgReason}}
	if !slices.Equal(listed.GitOnly, want) {
		t.Errorf("git_only = %+v, want %+v", listed.GitOnly, want)
	}
}

func TestListHooksAnswersEmptyListsWhereNoLefthookIsConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, listedDeps(func(context.Context) (hooks.Configured, error) {
		return hooks.Configured{}, nil
	}), config.Default())

	// Act
	recorder := webtest.Get(t, handler, webtest.HooksAt)

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
	handler := webtest.Serve(t, listedDeps(nil), config.Default())

	// Act
	recorder := webtest.Get(t, handler, webtest.HooksAt)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "lefthook is not installed")
}

func TestListHooksThatCannotReadTheConfigurationKeepsLefthooksWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, heard := webtest.Hearing(listedDeps(func(context.Context) (hooks.Configured, error) {
		return hooks.Configured{}, errUnreadableHere
	}))
	handler := webtest.Serve(t, deps, config.Default())

	// Act
	recorder := webtest.Get(t, handler, webtest.HooksAt)

	// Assert
	failure := assertProblem(t, recorder, http.StatusUnprocessableEntity, "could not be read")
	if strings.Contains(failure.Detail, webtest.RepoPath) {
		t.Errorf("detail = %q, want lefthook's words, which name the repository, kept off the wire", failure.Detail)
	}

	if len(*heard) != 1 || !errors.Is((*heard)[0], errUnreadableHere) {
		t.Errorf("Unexpected heard %v, want lefthook's refusal", *heard)
	}
}
