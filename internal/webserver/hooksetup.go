// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Why lefthook was not set up.
var (
	errHookSetupUnavailable = errors.New("setting up lefthook is not available here")
	errNothingToSetUp       = errors.New("there is nothing to set up: lefthook is configured, " +
		"or the hooks directory holds no hook it does not manage")
)

// GetHookSetup is the lefthook configuration offered for the hooks lefthook
// does not manage, as the terminal's g offers it, or nothing offered once
// lefthook is configured or there is no such hook.
func (s *server) GetHookSetup(context.Context, api.GetHookSetupRequestObject) (api.GetHookSetupResponseObject, error) {
	if s.deps.Hooks.Existing == nil {
		return api.GetHookSetup422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, errHookSetupUnavailable.Error())), nil
	}

	found := s.unmanagedHooks()
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
// Unexpected hears them.
func (s *server) SetUpHooks(
	_ context.Context, request api.SetUpHooksRequestObject,
) (api.SetUpHooksResponseObject, error) {
	if s.deps.Hooks.Existing == nil || s.deps.Hooks.Write == nil {
		return api.SetUpHooks422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, errHookSetupUnavailable.Error())), nil
	}

	found := s.unmanagedHooks()
	if len(found) == 0 {
		return api.SetUpHooks409ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeConflict, errNothingToSetUp.Error())), nil
	}

	generated := hooks.Structured(found)
	if orZero(request.Body.Verbatim) {
		generated = hooks.Verbatim(found)
	}

	err := s.deps.Hooks.Write(generated)
	if err != nil {
		// The answer leaves the cause out; the server's own log keeps it.
		s.unexpected(err)

		return api.SetUpHooks422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"lefthook.yml was not written, or lefthook not installed; "+
				"set it up with g in the terminal's Commits pane to see why")), nil
	}

	return api.SetUpHooks200JSONResponse(api.HookSetupWritten{Scripts: len(generated.Scripts)}), nil
}

// unmanagedHooks is the hooks lefthook does not manage, or none once the
// repository configures lefthook, as the terminal forgets them then.
func (s *server) unmanagedHooks() []hooks.GitHook {
	found, configured := s.deps.Hooks.Existing()
	if configured {
		return nil
	}

	return found
}

// hooksUnmanaged counts the hooks a frame offers to set up lefthook for, or
// none without a repository.
func (s *server) hooksUnmanaged() int {
	if s.deps.Hooks.Existing == nil {
		return 0
	}

	return len(s.unmanagedHooks())
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
