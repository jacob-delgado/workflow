// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"os"
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
// takes it up where the server works. No answer carries a credential. One
// setup runs at a time, by the server in effect once it may: a setup before it
// may have put another in place.
func (s *server) SetUp(_ context.Context, request api.SetUpRequestObject) (api.SetUpResponseObject, error) {
	s.worlds.setups.Lock()
	defer s.worlds.setups.Unlock()

	current, _ := s.worlds.served()

	return current.setUp(*request.Body), nil
}

// setUp is SetUp by this server: refused where a file applies, and a file
// made since the server started — by hand, by config init, or by a setup
// whose take-up failed — taken up rather than written over.
func (s *server) setUp(body api.SetupRequest) api.SetUpResponseObject {
	if !s.setupNeeded() {
		return api.SetUp409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"a configuration file already applies here; edit it in Settings"))
	}

	if s.fileMade() {
		return s.alreadyThere()
	}

	answers := setup.Answers{
		Jira: config.Jira{
			BaseURL: strings.TrimSpace(body.JiraBaseURL), Token: config.Secret(strings.TrimSpace(body.JiraToken)),
		},
		Webhook: config.Secret(strings.TrimSpace(body.WebhookURL)),
	}

	who, err := s.checkTyped(answers.Jira, body.KeepUnchecked)
	if err != nil {
		return checkFailure(err)
	}

	written, err := s.deps.Setup.Write(setup.Request{
		Place: setup.Place(body.Place), Answers: answers, Keychain: body.Keychain,
	})
	if err != nil {
		return s.setupRefusal(err)
	}

	return api.SetUp200JSONResponse(api.SetupResult{
		Path: written.Path, Shown: s.shownFile(written.Path), JiraUser: who, Keychain: written.Keychain,
		NotIgnored: written.NotIgnored, Reopened: s.takeUp(),
	})
}

// fileMade reports a file, or anything else, now at a place setup offers.
func (s *server) fileMade() bool {
	if s.deps.Setup.Offer == nil {
		return false
	}

	for _, place := range s.deps.Setup.Offer().Places {
		_, err := os.Lstat(place.Path)
		if err == nil {
			return true
		}
	}

	return false
}

// alreadyThere takes up a file found where setup would write, which is never
// written over, and says so.
func (s *server) alreadyThere() api.SetUpResponseObject {
	s.takeUp()

	return api.SetUp409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
		"a configuration file is already there; it is never written over, so edit it in Settings"))
}

// checkTyped asks Jira who the token typed is: no one to ask with Jira left
// out, and a refusal kept unchecked when asked and setup lets it be kept.
func (s *server) checkTyped(settings config.Jira, keepUnchecked bool) (string, error) {
	if settings.BaseURL == "" {
		return "", nil
	}

	who, err := s.deps.Setup.Check(settings)
	if err != nil && keepUnchecked && setup.Keepable(err) {
		return "", nil
	}

	return who, err
}

// setupRefusal is why a setup wrote nothing.
func (s *server) setupRefusal(err error) api.SetUpResponseObject {
	switch {
	case errors.Is(err, setup.ErrExists):
		return s.alreadyThere()
	case errors.Is(err, setup.ErrNoKeychain):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"there is no keychain here to keep the token in; keep it in the file instead"))
	case errors.Is(err, setup.ErrNoHome):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"there is no home directory here to keep the file in; keep it in the repository instead"))
	}

	body := s.fault(err)

	return api.SetUpdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: body.Status}
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

// checkFailure is why a check did not pass: an address that is no address,
// which is never kept, as unprocessable, and otherwise check_failed, which
// the page offers to keep anyway.
func checkFailure(err error) api.SetUpResponseObject {
	if !setup.Keepable(err) {
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"Jira's address is not an http or https address without a username or password; type it again"))
	}

	return api.SetUp422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeCheckFailed, checkRefusal(err)))
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
			causes: []error{jira.ErrUnauthorized, jira.ErrForbidden, jira.ErrNoCredential},
			code:   api.ProblemCodeCheckFailed, detail: "Jira did not accept the token",
		},
		{
			causes: []error{jira.ErrUnreachable, httpx.ErrRedirected, httpx.ErrRateLimited},
			code:   api.ProblemCodeCheckFailed, detail: "Jira could not be reached at that address",
		},
		{
			causes: []error{jira.ErrNoAPI, jira.ErrNotFound},
			code:   api.ProblemCodeCheckFailed, detail: "no Jira answers at that address",
		},
	}
}
