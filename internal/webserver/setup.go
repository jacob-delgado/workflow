// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// noFileHere is why there is no configuration to read or write: none applies
// where the server works, and Settings sets one up.
const noFileHere = "no " + config.FileName + " applies where the server works; set one up in Settings, " +
	"or run workflow config init"

// setupNeeded reports a server with no configuration file to read or write,
// where a first one can be set up.
func (s *server) setupNeeded() bool {
	return s.files == (config.Files{}) && s.deps.Setup.Write != nil
}

// GetSetup says whether a first configuration file is needed here, where it
// may go, and whether the keychain can keep the token.
func (s *server) GetSetup(context.Context, api.GetSetupRequestObject) (api.GetSetupResponseObject, error) {
	offer := api.SetupOffer{Needed: s.setupNeeded(), Places: []api.SetupPlace{}, Keychain: false}
	if s.deps.Setup.Offer == nil {
		return api.GetSetup200JSONResponse(offer), nil
	}

	offered := s.deps.Setup.Offer()
	for _, place := range offered.Places {
		offer.Places = append(offer.Places, api.SetupPlace{
			Place: api.SetupPlaceName(place.Place), Path: place.Path, Shown: s.shownFile(place.Path),
		})
	}

	offer.Keychain = offered.Keychain

	return api.GetSetup200JSONResponse(offer), nil
}

// shownFile is a file written from your home.
func (s *server) shownFile(path string) string {
	return workdirs.Shown(path, s.deps.Repositories.Home)
}

// SetUp checks the Jira token, writes the first configuration file, and
// takes it up where the server works. No answer carries a credential.
func (s *server) SetUp(_ context.Context, request api.SetUpRequestObject) (api.SetUpResponseObject, error) {
	if !s.setupNeeded() {
		return api.SetUp409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"a configuration file already applies here; edit it in Settings")), nil
	}

	body := *request.Body
	answers := setup.Answers{
		Jira: config.Jira{
			BaseURL: strings.TrimSpace(body.JiraBaseURL), Token: config.Secret(strings.TrimSpace(body.JiraToken)),
		},
		Webhook: config.Secret(strings.TrimSpace(body.WebhookURL)),
	}

	who, err := s.checkTyped(answers.Jira, body.KeepUnchecked)
	if err != nil {
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.CheckFailed, checkRefusal(err))), nil
	}

	written, err := s.deps.Setup.Write(setup.Request{
		Place: setup.Place(body.Place), Answers: answers, Keychain: body.Keychain,
	})
	if err != nil {
		return s.setupRefusal(err), nil
	}

	return api.SetUp200JSONResponse(api.SetupResult{
		Path: written.Path, Shown: s.shownFile(written.Path), JiraUser: who, Keychain: written.Keychain,
		NotIgnored: written.NotIgnored, Reopened: s.takeUp(),
	}), nil
}

// checkTyped asks Jira who the token typed is: no one to ask with Jira left
// out, and a refusal kept unchecked when asked.
func (s *server) checkTyped(settings config.Jira, keepUnchecked bool) (string, error) {
	if settings.BaseURL == "" {
		return "", nil
	}

	who, err := s.deps.Setup.Check(settings)
	if err != nil && keepUnchecked {
		return "", nil
	}

	return who, err
}

// setupRefusal is why a setup wrote nothing.
func (s *server) setupRefusal(err error) api.SetUpResponseObject {
	switch {
	case errors.Is(err, setup.ErrExists):
		return api.SetUp409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"a configuration file is already there; it is never written over, so edit it in Settings"))
	case errors.Is(err, setup.ErrNoKeychain):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable,
			"there is no keychain here to keep the token in; keep it in the file instead"))
	case errors.Is(err, setup.ErrNoHome):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable,
			"there is no home directory here to keep the file in; keep it in the repository instead"))
	}

	body, code := s.fault(err)

	return api.SetUpdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// takeUp serves the file just written from now on, where the server works,
// as a switch does, and reports whether it could.
func (s *server) takeUp() bool {
	if s.worlds == nil {
		return false
	}

	_, err := s.worlds.switchTo(s.deps.Repositories.Here.Dir)
	if err != nil && !errors.Is(err, errNoSwitching) {
		s.unexpected(err)
	}

	return err == nil
}

// checkRefusal says why Jira's check did not pass, in words that name no
// address and no token, and what to do.
func checkRefusal(err error) string {
	for _, refusal := range checkRefusals() {
		if slices.ContainsFunc(refusal.causes, func(cause error) bool { return errors.Is(err, cause) }) {
			return refusal.detail + "; check it, or keep it anyway"
		}
	}

	return "Jira could not check the token; check the address and the token, or keep them anyway"
}

// checkRefusals are the reasons a check tells apart.
func checkRefusals() []faultClass {
	return []faultClass{
		{
			causes: []error{config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL},
			code:   api.CheckFailed,
			detail: "Jira's address is not an http or https address without a username or password",
		},
		{
			causes: []error{jira.ErrUnauthorized, jira.ErrForbidden, jira.ErrNoCredential},
			code:   api.CheckFailed, detail: "Jira did not accept the token",
		},
		{
			causes: []error{jira.ErrUnreachable, httpx.ErrRedirected, httpx.ErrRateLimited},
			code:   api.CheckFailed, detail: "Jira could not be reached at that address",
		},
		{
			causes: []error{jira.ErrNoAPI, jira.ErrNotFound},
			code:   api.CheckFailed, detail: "no Jira answers at that address",
		},
	}
}
