// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// Why lefthook was not set up, or its hooks not listed.
var (
	errHookSetupUnavailable = errors.New("setting up lefthook is not available here")
	errNothingToSetUp       = errors.New("there is nothing to set up: lefthook is configured, " +
		"or the hooks directory holds no hook it does not manage")
	errHooksUnavailable = errors.New("lefthook's hooks cannot be listed here: lefthook is not installed")
	errHooksUnreadable  = errors.New("lefthook's configuration could not be read; " +
		"run lefthook dump in a terminal to see why")
)

// ListHooks is the hooks lefthook configures, read each time it is asked, as
// the terminal's h reads them: those a run by hand can stand in for, and those
// only git runs. lefthook's own words on a configuration it cannot read name
// the repository, so they go to the unexpected New is handed rather than onto
// the wire.
func (r *Runner) ListHooks(ctx context.Context, _ api.ListHooksRequestObject) (api.ListHooksResponseObject, error) {
	if r.seams.Hooks.Configured == nil {
		return api.ListHooks422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, errHooksUnavailable.Error())), nil
	}

	configured, err := r.seams.Hooks.Configured(ctx)
	if err != nil {
		r.unexpected(err)

		return api.ListHooks422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, errHooksUnreadable.Error())), nil
	}

	return api.ListHooks200JSONResponse(configuredDTO(configured)), nil
}

// configuredDTO maps the hooks lefthook configures onto the wire, an empty
// list as [] rather than null. A name goes out as lefthook gave it, unlike
// unmanagedDTO's: the page shows it as text React escapes, and sends it back
// to run, so it must match lefthook's exactly.
func configuredDTO(configured hooks.Configured) api.ConfiguredHooks {
	gitOnly := make([]api.GitOnlyHook, 0, len(configured.GitOnly))
	for _, hook := range configured.GitOnly {
		gitOnly = append(gitOnly, api.GitOnlyHook{Name: hook.Name, Reason: hook.Reason})
	}

	return api.ConfiguredHooks{Runnable: append([]string{}, configured.Runnable...), GitOnly: gitOnly}
}

// GetHookSetup is the lefthook configuration offered for the hooks lefthook
// does not manage, as the terminal's g offers it, or nothing offered once
// lefthook is configured or there is no such hook.
func (r *Runner) GetHookSetup(
	ctx context.Context, _ api.GetHookSetupRequestObject,
) (api.GetHookSetupResponseObject, error) {
	if r.seams.Hooks.Existing == nil {
		return api.GetHookSetup422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, errHookSetupUnavailable.Error())), nil
	}

	found := r.unmanagedHooks(ctx)
	setup := api.HookSetup{Offered: len(found) > 0, Hooks: unmanagedDTO(found), Config: "", Scripts: 0}

	if setup.Offered {
		generated := hooks.Structured(found)
		setup.Config, setup.Scripts = sanitize.Text(generated.Config), len(generated.Scripts)
	}

	return api.GetHookSetup200JSONResponse(setup), nil
}

// SetUpHooks writes the configuration offered — or one keeping every hook
// whole as a script — and installs lefthook, as the terminal's offer does.
// With nothing offered it is a 409; a write or an install that fails is a 422
// whose words stay off the wire, since they name where the repository is;
// the unexpected New is handed hears them.
func (r *Runner) SetUpHooks(
	ctx context.Context, request api.SetUpHooksRequestObject,
) (api.SetUpHooksResponseObject, error) {
	if r.seams.Hooks.Existing == nil || r.seams.Hooks.Write == nil {
		return api.SetUpHooks422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, errHookSetupUnavailable.Error())), nil
	}

	found := r.unmanagedHooks(ctx)
	if len(found) == 0 {
		return api.SetUpHooks409ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeConflict, errNothingToSetUp.Error())), nil
	}

	generated := hooks.Structured(found)
	if orZero(request.Body.Verbatim) {
		generated = hooks.Verbatim(found)
	}

	err := r.seams.Hooks.Write(ctx, generated)
	if err != nil {
		// The answer leaves the cause out; the server's own log keeps it.
		r.unexpected(err)

		return api.SetUpHooks422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			"lefthook.yml was not written, or lefthook not installed; "+
				"set it up with g in the terminal's Commits pane to see why")), nil
	}

	return api.SetUpHooks200JSONResponse(api.HookSetupWritten{Scripts: len(generated.Scripts)}), nil
}

// unmanagedHooks is the hooks lefthook does not manage, or none once the
// repository configures lefthook, as the terminal forgets them then.
func (r *Runner) unmanagedHooks(ctx context.Context) []hooks.GitHook {
	found, configured := r.seams.Hooks.Existing(ctx)
	if configured {
		return nil
	}

	return found
}

// HooksUnmanaged counts the hooks a frame offers to set up lefthook for, or
// none without a repository.
func (r *Runner) HooksUnmanaged(ctx context.Context) int {
	if r.seams.Hooks.Existing == nil {
		return 0
	}

	return len(r.unmanagedHooks(ctx))
}

// unmanagedDTO maps the hooks found onto the wire, each by its name and how
// many lines its script holds.
func unmanagedDTO(found []hooks.GitHook) []api.UnmanagedHook {
	mapped := make([]api.UnmanagedHook, 0, len(found))
	for _, hook := range found {
		lines := strings.Count(strings.TrimRight(hook.Script, "\n"), "\n") + 1
		mapped = append(mapped, api.UnmanagedHook{Name: sanitize.Line(hook.Name), Lines: lines})
	}

	return mapped
}
