// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// noFileHere is why there is no configuration to read or write: none applies
// where the server works, and Settings sets one up.
const noFileHere = "no " + config.FileName + " applies where the server works; set one up in Settings, " +
	"or run workflow config init"

// notTakenUp is why there is no configuration to read or write where a file
// was made since none applied: the server has not taken it up.
const notTakenUp = "a " + config.FileName + " was made here that the server has not taken up; " +
	"read the configuration again, or restart workflow --web"

// setupNeeded reports a server with no configuration file to read or write,
// where a first one can be set up.
func (s *server) setupNeeded() bool {
	return s.files == (config.Files{}) && s.deps.Settings.Setup.Write != nil
}

// configMadeSince is GetConfig by a server where no file applied: a file made
// since — by hand, by config init, or by a setup that could not take it up —
// is taken up under ctx, as a switch does, and read by the server taking it
// up; with none, a first one is set up. A take-up that fails, or that finds
// no file to serve, as a dangling link at the path leaves, answers conflict.
func (s *server) configMadeSince(
	ctx context.Context, request api.GetConfigRequestObject,
) (api.GetConfigResponseObject, error) {
	if !s.fileMade() {
		return api.GetConfig404ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeNotFound, noFileHere)), nil
	}

	next, served := s.takeUpMade(ctx)
	if !served {
		return api.GetConfig409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict, notTakenUp)), nil
	}

	return next.GetConfig(ctx, request)
}

// takeUpMade serves a file made since none applied from now on, where the
// server works, as a switch does under ctx, and answers the server switched
// to, and whether it serves a file. One take-up runs at a time, as one setup
// does: a read that arrives during another's waits for it, then answers by the
// server it put in place rather than wiring the directory again.
func (s *server) takeUpMade(ctx context.Context) (*server, bool) {
	s.worlds.setups.Lock()
	defer s.worlds.setups.Unlock()

	if current, _ := s.worlds.served(); current != s {
		return current, !current.setupNeeded()
	}

	next, err := s.worlds.switchTo(ctx, s.deps.Repositories.Here.Dir)
	s.noteTakeUp(err)

	return next, err == nil && !next.setupNeeded()
}

// noConfigToUpdate is UpdateConfig by a server where no file applied: none to
// write over, or one made since that a read takes up first, which a write
// would only write over.
func (s *server) noConfigToUpdate() api.UpdateConfigResponseObject {
	if s.fileMade() {
		return api.UpdateConfig409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict, notTakenUp))
	}

	return api.UpdateConfig404ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeNotFound, noFileHere))
}

// GetSetup says whether a first configuration file is needed here, where it
// may go, and whether the keychain can keep the token for each.
func (s *server) GetSetup(context.Context, api.GetSetupRequestObject) (api.GetSetupResponseObject, error) {
	offer := api.SetupOffer{Needed: s.setupNeeded(), Places: []api.SetupPlace{}}
	if s.deps.Settings.Setup.Offer == nil {
		return api.GetSetup200JSONResponse(offer), nil
	}

	for _, place := range s.deps.Settings.Setup.Offer().Places {
		offer.Places = append(offer.Places, api.SetupPlace{
			Place: api.SetupPlaceName(place.Place), Path: place.Path, Shown: s.shownFile(place.Path),
			Keychain: place.Keychain,
		})
	}

	return api.GetSetup200JSONResponse(offer), nil
}

// shownFile is a file written from your home.
func (s *server) shownFile(path string) string {
	return workdirs.Shown(path, s.deps.Repositories.Home)
}

// writeHoldsSetupBack is why a setup sent while a write is in flight wrote
// nothing: the file could not be taken up until that write ends.
const writeHoldsSetupBack = "a write is in flight; set up once it has finished, nothing was written"

// SetUp checks the Jira token, writes the first configuration file, and
// takes it up where the server works. No answer carries a credential. One
// setup runs at a time, by the server in effect once it may: a setup before it
// may have put another in place. It holds the gate alone from the start, as a
// switch does, so no write begun after it can keep its file from being taken
// up, and a write in flight refuses it before anything is written.
func (s *server) SetUp(ctx context.Context, request api.SetUpRequestObject) (api.SetUpResponseObject, error) {
	s.worlds.setups.Lock()
	defer s.worlds.setups.Unlock()

	if !s.worlds.gate.TryLock() {
		return api.SetUp409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict,
			writeHoldsSetupBack)), nil
	}
	defer s.worlds.gate.Unlock()

	current, _ := s.worlds.served()

	return current.setUp(ctx, *request.Body), nil
}

// setUp is SetUp by this server: refused where a file applies, and a file
// made since the server started — by hand, by config init, or by a setup
// whose take-up failed — taken up rather than written over.
func (s *server) setUp(ctx context.Context, body api.SetupRequest) api.SetUpResponseObject {
	if !s.setupNeeded() {
		return api.SetUp409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict,
			"a configuration file already applies here; edit it in Settings"))
	}

	if s.fileMade() {
		return s.alreadyThere(ctx)
	}

	answers := setup.Answers{
		Jira: config.Jira{
			BaseURL: strings.TrimSpace(body.JiraBaseURL), Token: config.Secret(strings.TrimSpace(body.JiraToken)),
		},
		Webhook: config.Secret(strings.TrimSpace(body.WebhookURL)),
	}

	who, err := s.checkTyped(ctx, answers.Jira, body.KeepUnchecked)
	if err != nil {
		return checkFailure(err)
	}

	written, err := s.deps.Settings.Setup.Write(ctx, setup.Request{
		Place: setup.Place(body.Place), Answers: answers, Keychain: body.Keychain,
	})
	if err != nil {
		return s.setupRefusal(ctx, err)
	}

	return api.SetUp200JSONResponse(api.SetupResult{
		Path: written.Path, Shown: s.shownFile(written.Path), JiraUser: who, Keychain: written.Keychain,
		NotIgnored: written.NotIgnored, Reopened: s.takeUp(ctx),
	})
}

// fileMade reports a file, or anything else, now at a place setup offers.
func (s *server) fileMade() bool {
	if s.deps.Settings.Setup.Offer == nil {
		return false
	}

	for _, place := range s.deps.Settings.Setup.Offer().Places {
		_, err := os.Lstat(place.Path)
		if err == nil {
			return true
		}
	}

	return false
}

// alreadyThere takes up a file found where setup would write, which is never
// written over, and says so.
func (s *server) alreadyThere(ctx context.Context) api.SetUpResponseObject {
	s.takeUp(ctx)

	return api.SetUp409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict,
		"a configuration file is already there; it is never written over, so edit it in Settings"))
}

// checkTyped asks Jira who the token typed is: no one to ask with Jira left
// out, and a refusal kept unchecked when asked and setup lets it be kept.
func (s *server) checkTyped(ctx context.Context, settings config.Jira, keepUnchecked bool) (string, error) {
	if settings.BaseURL == "" {
		return "", nil
	}

	who, err := s.deps.Settings.Setup.Check(ctx, settings)
	if err != nil && keepUnchecked && setup.Keepable(err) {
		return "", nil
	}

	return who, err
}

// setupRefusal is why a setup wrote nothing.
func (s *server) setupRefusal(ctx context.Context, err error) api.SetUpResponseObject {
	switch {
	case errors.Is(err, setup.ErrExists):
		return s.alreadyThere(ctx)
	case errors.Is(err, setup.ErrNoKeychain):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			"there is no keychain here to keep the token in; keep it in the file instead"))
	case errors.Is(err, setup.ErrNoHome):
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			"there is no home directory here to keep the file in; keep it in the repository instead"))
	}

	return problem.Answer[api.SetUpdefaultApplicationProblemPlusJSONResponse](s.fault(err))
}

// takeUp serves the file just written from now on, where the server works,
// as a switch does under ctx with the gate SetUp holds, and reports whether it
// could.
func (s *server) takeUp(ctx context.Context) bool {
	_, err := s.worlds.switchHeld(ctx, s.deps.Repositories.Here.Dir)
	s.noteTakeUp(err)

	return err == nil
}

// noteTakeUp notes why a take-up failed, but for a server that cannot switch
// and a write in flight, which are no failure of the server's.
func (s *server) noteTakeUp(err error) {
	if err != nil && !errors.Is(err, errNoSwitching) && !errors.Is(err, errWriteInFlight) {
		s.unexpected(err)
	}
}

// checkFailure is why a check did not pass: an address that is no address,
// which is never kept, as unprocessable, and otherwise check_failed, which
// the page offers to keep anyway.
func checkFailure(err error) api.SetUpResponseObject {
	if !setup.Keepable(err) {
		return api.SetUp422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeUnprocessable,
			"Jira's address is not an https address, or http to this machine, without a username or password; "+
				"type it again"))
	}

	return api.SetUp422ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeCheckFailed, checkRefusal(err)))
}

// checkRefusal says why Jira's check did not pass, in words that name no
// address and no token, and what to do.
func checkRefusal(err error) string {
	for _, refusal := range checkRefusals() {
		if refusal.Matches(err) {
			return refusal.Detail + "; check it, or keep it anyway"
		}
	}

	return "Jira could not check the token; check the address and the token, or keep them anyway"
}

// checkRefusals are the reasons a check tells apart.
func checkRefusals() []report.Class {
	return []report.Class{
		{
			Causes: []error{jira.ErrUnauthorized, jira.ErrForbidden, jira.ErrNoCredential},
			Code:   api.ProblemCodeCheckFailed, Detail: "Jira did not accept the token",
		},
		{
			Causes: []error{jira.ErrUnreachable, httpx.ErrRedirected, httpx.ErrRateLimited},
			Code:   api.ProblemCodeCheckFailed, Detail: "Jira could not be reached at that address",
		},
		{
			Causes: []error{jira.ErrNoAPI, jira.ErrNotFound},
			Code:   api.ProblemCodeCheckFailed, Detail: "no Jira answers at that address",
		},
	}
}
