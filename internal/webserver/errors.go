// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// problemBase is where a problem's type URI points: one anchor per code on the
// errors reference page, so the URI dereferences to what the code means.
const problemBase = "https://jacob-delgado.github.io/workflow/docs/errors/"

// problem builds an RFC 9457 problem details object for a code, deriving its
// type, title and status. The detail is safe to show and never carries a secret.
func problem(code api.ProblemCode, detail string) api.Problem {
	status, title := codeMeaning(code)

	return api.Problem{
		// The fragment is the code with hyphens, matching the heading anchors the
		// errors reference page generates, so the type URI dereferences to it.
		Type:   problemBase + "#" + strings.ReplaceAll(string(code), "_", "-"),
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
	}
}

// codeMeaning is the HTTP status and human title fixed for each problem code. A
// map, not a switch, as ciState is, so there is no last-case arm gobco can never
// see; exhaustive keeps it complete, and every code is one of the spec's
// constants.
func codeMeaning(code api.ProblemCode) (int, string) {
	meaning := map[api.ProblemCode]struct {
		status int
		title  string
	}{
		api.ProblemCodeBadRequest:           {status: http.StatusBadRequest, title: "Bad request"},
		api.ProblemCodeNotFound:             {status: http.StatusNotFound, title: "Not found"},
		api.ProblemCodeMethodNotAllowed:     {status: http.StatusMethodNotAllowed, title: "Method not allowed"},
		api.ProblemCodeConflict:             {status: http.StatusConflict, title: "Conflict"},
		api.ProblemCodeUnprocessable:        {status: http.StatusUnprocessableEntity, title: "Unprocessable content"},
		api.ProblemCodeNotSetUp:             {status: http.StatusUnprocessableEntity, title: "Not set up"},
		api.ProblemCodeTooLong:              {status: http.StatusUnprocessableEntity, title: "Too long"},
		api.ProblemCodePreconditionRequired: {status: http.StatusPreconditionRequired, title: "Precondition required"},
		api.ProblemCodeUnreachable:          {status: http.StatusBadGateway, title: "Upstream unreachable"},
		api.ProblemCodeFetchFailed:          {status: http.StatusBadGateway, title: "Fetch failed"},
		api.ProblemCodeCheckFailed:          {status: http.StatusUnprocessableEntity, title: "Check failed"},
		api.ProblemCodeInternal:             {status: http.StatusInternalServerError, title: "Internal error"},
	}[code]

	return meaning.status, meaning.title
}

// writeProblem writes a problem details object as application/problem+json, for
// the hand-written middleware that answers before a strict handler runs.
func writeProblem(w http.ResponseWriter, code api.ProblemCode, detail string) {
	prob := problem(code, detail)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(prob.Status)
	_ = json.NewEncoder(w).Encode(prob) //nolint:errchkjson // the api types marshal cleanly
}

// writeRequestError answers a request the generated binder or the strict decoder
// could not read — a bad query parameter or a malformed body. The detail stays
// off the wire; that a field was wrong, not which internal parser said so, is
// what the caller can act on.
func writeRequestError(w http.ResponseWriter, _ *http.Request, _ error) {
	writeProblem(w, api.ProblemCodeBadRequest, "the request could not be understood")
}

// writeResponseError is the safety net for a handler that returns an error
// rather than a typed response, or an answer that could not be written: an
// opaque 500, so an unexpected failure never leaks its detail, that still says
// what to do. The error goes to Unexpected, since the answer carries none of it.
func (s *server) writeResponseError(w http.ResponseWriter, _ *http.Request, err error) {
	s.unexpected(err)
	writeProblem(w, api.ProblemCodeInternal, "the server could not answer; "+tryAgain)
}

// tryAgain is what to do about a failure nothing more is known of.
const tryAgain = "try again, and run workflow doctor if it keeps failing"

// fault maps a seam's error onto an RFC 9457 problem, its status the code's. The class
// comes from the error — one of faultClasses (no repository to work in, a
// missing resource, an upstream that could not be reached, asked to wait or
// answered oddly, a setting it cannot use, a service's refusal) or else an
// unexpected failure — but the detail is curated and safe: the raw cause
// carries a host, a path, a webhook or a credential and never reaches the wire.
// An unexpected failure's cause goes to Unexpected instead, since no class
// says what it was.
func (s *server) fault(err error) api.Problem {
	prob, classified := faultProblem(err)
	if !classified {
		s.unexpected(err)
	}

	return prob
}

// unexpected hands a failure the answer leaves out to Unexpected, when it is
// wired.
func (s *server) unexpected(err error) {
	if s.deps.Unexpected != nil {
		s.deps.Unexpected(err)
	}
}

// faultProblem classifies a seam's error into the problem shown for it: the
// first class it belongs to, reported true, or an opaque internal error,
// reported false.
func faultProblem(err error) (api.Problem, bool) {
	// A token the forge turned down is told in forge.Advice's words, the ones
	// the terminal shows: the forge, the scope it asks for, and its own reason
	// for the refusal, which is the forge's and never carries its address.
	if advice, ok := forge.Advice(err); ok {
		return problem(api.ProblemCodeUnprocessable, advice), true
	}

	// A message Slack would not deliver carries the fix for its channel, which
	// names a channel and Slack's code but never an address.
	if refused, ok := errors.AsType[messaging.PostRefusedError](err); ok {
		return problem(api.ProblemCodeUnprocessable, refused.Error()), true
	}

	for _, class := range faultClasses() {
		if slices.ContainsFunc(class.causes, func(cause error) bool { return errors.Is(err, cause) }) {
			return problem(setUpCode(class.code, err), class.detail), true
		}
	}

	return problem(api.ProblemCodeInternal, "the request could not be completed; "+tryAgain), false
}

// setUpDetail is how to set up what cause says is missing, in the words
// every surface shares (loop.SetUpAdvice), so the web, the command line and
// the terminal tell it alike.
func setUpDetail(cause error) string {
	advice, _ := loop.SetUpAdvice(cause)

	return advice
}

// setUpCode is code, or not_set_up for an error loop.NotSetUp says found
// nothing set up to ask, so every answer — an error, a panel's problem, a
// summary's source — tells setting up apart from a refusal the one way the
// command line does too. The class still words it, with how to set it up.
func setUpCode(code api.ProblemCode, err error) api.ProblemCode {
	if loop.NotSetUp(err) {
		return api.ProblemCodeNotSetUp
	}

	return code
}

// faultClass is one kind of seam failure: the sentinels that belong to it, and
// the problem shown for it — a detail that says what to do, never the cause's
// own text, which can carry the tracker's or the forge's address.
type faultClass struct {
	causes []error
	code   api.ProblemCode
	detail string
}

// faultClasses are the failures fault tells apart, most specific first: a Jira
// 404 that carries a reason is a missing resource before it is a refusal.
func faultClasses() []faultClass {
	return slices.Concat(gitFaults(), transportFaults(), jiraFaults(), forgeFaults(), messagingFaults(),
		taskwarriorFaults(), directoryFaults(), localDataFaults(), peopleFaults())
}

// gitFaults are the repository's failures fault can name. Any other read git
// could not answer is left to the internal problem: its words can name where
// the repository is on disk, and the caller cannot act on them.
func gitFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{gitrepo.ErrNotARepository},
			code:   api.ProblemCodeConflict,
			detail: "the server is not running in a git repository; start workflow --web from a repository's work tree",
		},
		{
			causes: []error{gitrepo.ErrNoIdentity},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(gitrepo.ErrNoIdentity),
		},
		{causes: []error{errBranchExists}, code: api.ProblemCodeConflict, detail: errBranchExists.Error()},
		{
			causes: []error{gitrepo.ErrIssueLinkNotSaved},
			code:   api.ProblemCodeUnprocessable,
			detail: "the link could not be kept: git could not change the repository's configuration; " +
				"check that .git/config is writable",
		},
	}
}

// transportFaults are the failures any upstream can answer with.
func transportFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{jira.ErrNotFound, forge.ErrNoRepository},
			code:   api.ProblemCodeNotFound, detail: "the requested resource was not found",
		},
		{
			causes: []error{jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable},
			code:   api.ProblemCodeUnreachable, detail: "the service could not be reached; check the network, then try again",
		},
		// Every client answers a 429 and a redirect with the transport's own
		// sentinels, so these details name no service.
		{
			causes: []error{httpx.ErrRateLimited},
			code:   api.ProblemCodeUnreachable, detail: "the service is limiting requests; wait and try again",
		},
		{
			causes: []error{httpx.ErrRedirected},
			code:   api.ProblemCodeUnreachable,
			detail: "the service answered with a redirect, refused so the credential goes nowhere else; " +
				"check its configured address",
		},
	}
}

// jiraFaults are the tracker's failures.
func jiraFaults() []faultClass {
	return []faultClass{
		// The client refuses the next two classes before it asks Jira anything, so
		// their details name the setting, not what Jira did.
		{
			causes: []error{jira.ErrNoCredential},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(jira.ErrNoCredential),
		},
		{
			causes: []error{config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL},
			code:   api.ProblemCodeUnprocessable,
			detail: "jira.base_url is not a usable address; workflow doctor checks it",
		},
		// Before the credential's class: a 403 Jira explained answers to both, and
		// doctor, which asks only who the token is, would pass a token that lacks
		// one permission.
		{
			causes: []error{jira.ErrRejected},
			code:   api.ProblemCodeUnprocessable, detail: "Jira refused the request",
		},
		{
			causes: []error{jira.ErrUnauthorized, jira.ErrForbidden},
			code:   api.ProblemCodeUnprocessable,
			detail: "Jira did not accept the configured credential; workflow doctor checks it",
		},
		{
			causes: []error{jira.ErrNoAPI},
			code:   api.ProblemCodeUnprocessable, detail: "no Jira API answers at jira.base_url; workflow doctor checks it",
		},
	}
}

// forgeFaults are GitHub's and GitLab's failures. The first two are found
// before the forge is asked anything, so their details name what to set; a
// refusal is the forge saying what the token may not do, so it points at the
// token's scopes — a rate limit is told apart before it and answered with the
// transport's own class.
func forgeFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{forge.ErrNoToken},
			code:   api.ProblemCodeUnprocessable,
			// Not the resolver's own words: for a host other than github.com or
			// gitlab.com they name the host, which a detail never does.
			detail: setUpDetail(forge.ErrNoToken),
		},
		{
			causes: []error{forge.ErrKindNeedsHost},
			code:   api.ProblemCodeUnprocessable, detail: "forge.kind is set without forge.host; set the host it describes",
		},
		{
			causes: []error{forge.ErrUnknownForge},
			code:   api.ProblemCodeUnprocessable, detail: setUpDetail(forge.ErrUnknownForge),
		},
		{
			causes: []error{forge.ErrNotARemote},
			code:   api.ProblemCodeUnprocessable, detail: setUpDetail(forge.ErrNotARemote),
		},
		{
			causes: []error{forge.ErrNoAPI, forge.ErrNotJSON},
			code:   api.ProblemCodeUnprocessable,
			detail: "no forge API answered; check forge.host, which workflow doctor --online tests",
		},
		// A job log GitHub hands to storage elsewhere: neither class is the
		// token's doing, and the storage's address is signed, so it stays out.
		{
			causes: []error{forge.ErrInsecureLog, forge.ErrLogNotRedirected},
			code:   api.ProblemCodeUnreachable,
			detail: "the forge sent the log somewhere it could not be read from safely; open the check's page instead",
		},
		{
			causes: []error{forge.ErrLogStorage},
			code:   api.ProblemCodeUnreachable,
			detail: "the storage the forge keeps the log in did not hand it over; try again, or open the check's page",
		},
		{
			// An explained status is the same undocumented status with the
			// forge's reason, which stays off the wire.
			causes: []error{forge.ErrUnexpectedStatus, forge.ErrRejected},
			code:   api.ProblemCodeUnreachable, detail: "the forge answered with a status it does not document; try again",
		},
	}
}

// messagingFaults are the messaging service's failures, told the same whichever
// service it is. None forwards the error's own text: a client's error can name
// the webhook, whose address is its credential. A message refused for its
// channel is faultProblem's, which tells its reason.
func messagingFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{messaging.ErrNoCredential},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(messaging.ErrNoCredential),
		},
		{
			causes: []error{messaging.ErrInsecureWebhook},
			code:   api.ProblemCodeUnprocessable,
			detail: "messaging.webhook_url is not an https address; copy the webhook's https address into it in Settings",
		},
		// Every 4xx a post meets is this one, and a webhook — the only transport
		// Teams, Discord and a plain webhook have — is one workflow doctor cannot
		// check, so the detail points at the settings rather than at doctor.
		{
			causes: []error{messaging.ErrRejected},
			code:   api.ProblemCodeUnprocessable,
			detail: "the messaging service refused the post; " +
				"check the token, or that the webhook URL is current, in Settings",
		},
		{
			causes: []error{messaging.ErrUnexpectedStatus},
			code:   api.ProblemCodeUnreachable,
			detail: "the messaging service answered with a status it does not document; try again",
		},
	}
}

// taskwarriorFaults are Taskwarrior's failures that have fixed words: a write it
// declined (a declined start ahead of the rest, since it has words of its own),
// a sync with nowhere to go, and each reason no Taskwarrior can be asked, worded
// as the task list words it. A refusal and a
// timeout are taskFault's, and an answer that is not Taskwarrior's JSON is left
// to the internal problem: nothing the caller can act on.
func taskwarriorFaults() []faultClass {
	reasons := taskwarriorReasons()

	unavailable := make([]faultClass, 0, len(reasons))
	for _, reason := range reasons {
		unavailable = append(unavailable,
			faultClass{causes: []error{reason.cause}, code: api.ProblemCodeUnprocessable, detail: reason.text})
	}

	return slices.Concat([]faultClass{
		{
			causes: []error{errStartDeclined},
			code:   api.ProblemCodeConflict, detail: "the task is already started, or no such task exists",
		},
		{
			causes: []error{taskwarrior.ErrNothingChanged},
			code:   api.ProblemCodeConflict, detail: "the task is already in that state, or is no longer pending",
		},
		{
			causes: []error{taskwarrior.ErrNoSync},
			code:   api.ProblemCodeUnprocessable,
			detail: "No sync backend is set in your taskrc, so there is nowhere to sync. " +
				"Set one of the sync.* settings (task-sync(5)).",
		},
	}, unavailable)
}

// taskwarriorReason is why no Taskwarrior can be asked, as the code the task
// list gives it and in words safe to show: never the error's own, which name
// the task program's path.
type taskwarriorReason struct {
	cause error
	code  api.TaskListReasonCode
	text  string
}

// taskwarriorReasons are the reasons the task list and a refused write share.
func taskwarriorReasons() []taskwarriorReason {
	return []taskwarriorReason{
		{
			cause: taskwarrior.ErrNotInstalled, code: api.TaskListReasonCodeNotInstalled,
			text: setUpDetail(taskwarrior.ErrNotInstalled),
		},
		{
			cause: taskwarrior.ErrNotTaskwarrior, code: api.TaskListReasonCodeNotTaskwarrior,
			text: setUpDetail(taskwarrior.ErrNotTaskwarrior),
		},
		{
			cause: taskwarrior.ErrTooOld, code: api.TaskListReasonCodeTooOld,
			text: "Taskwarrior is too old: " + taskwarrior.MinimumVersion +
				" or newer is needed; workflow doctor shows the version found.",
		},
		{
			cause: taskwarrior.ErrNotConfigured, code: api.TaskListReasonCodeNeverRun,
			text: setUpDetail(taskwarrior.ErrNotConfigured),
		},
	}
}

// taskFault is fault for what a Taskwarrior seam answered. A refusal carries
// Taskwarrior's own words, which name what in the line it could not take, and a
// bound that ran out is Taskwarrior's: no class can say so, since git's reads
// time out with the same sentinel.
func (s *server) taskFault(err error) api.Problem {
	switch {
	case errors.Is(err, taskwarrior.ErrRefused):
		return problem(api.ProblemCodeUnprocessable, s.refusalDetail(err))
	case errors.Is(err, proc.ErrTimedOut):
		return problem(api.ProblemCodeUnreachable, "Taskwarrior did not answer in time")
	default:
		return s.fault(err)
	}
}

// malformedEntry opens Taskwarrior's words for a taskrc line it cannot read,
// which quote the line, secret and all. internal/taskwarrior words that refusal
// anew; a line still carrying it is dropped all the same.
const malformedEntry = "Malformed entry '"

// refusalDetail words a command Taskwarrior refused: its own words but what
// they can carry that no answer may — its data directory and your home, which
// read as fixed words, and each line naming a URL or a host and port, since a
// hook's feedback can name the sync server, or quoting a taskrc line, dropped
// whole — or, with no words left, the refusal alone.
func (s *server) refusalDetail(err error) string {
	places := s.knownPlaces()
	address, clock := networkAddress(), clockTime()

	var kept []string

	for line := range strings.Lines(refusalWords(err)) {
		if !address.MatchString(clock.ReplaceAllString(line, "$1")) && !strings.Contains(line, malformedEntry) {
			kept = append(kept, places.Replace(strings.TrimRight(line, "\r\n")))
		}
	}

	words := strings.TrimSpace(strings.Join(kept, "\n"))
	if words == "" {
		return "Taskwarrior refused the command"
	}

	return "Taskwarrior refused the command: " + words
}

// knownPlaces puts fixed words in place of the directories Taskwarrior's words
// can name: its data directory, then your home, which most often holds it, so
// a path under both reads as the data directory. One that names no place of
// its own is left out — see namesAPlace.
func (s *server) knownPlaces() *strings.Replacer {
	var pairs []string

	install, err := s.installed()
	if err == nil && namesAPlace(install.DataDir) {
		pairs = append(pairs, install.DataDir, "the data directory")
	}

	if home := s.homeDir(); namesAPlace(home) {
		pairs = append(pairs, home, "~")
	}

	return strings.NewReplacer(pairs...)
}

// namesAPlace reports whether dir is an absolute path below the root, the only
// kind worth replacing: an empty one would match between every letter, a
// relative one ("task") ordinary words, and the root every path.
func namesAPlace(dir string) bool {
	return filepath.IsAbs(dir) && filepath.Dir(dir) != dir
}

// homeDir is your home directory, or "" when none is known.
func (s *server) homeDir() string {
	if s.deps.HomeDir == nil {
		return ""
	}

	home, err := s.deps.HomeDir()
	if err != nil {
		return ""
	}

	return home
}

// networkAddress matches what names a machine on the network: a URL, or a host
// and port — a name, an IPv4 address or a bracketed IPv6 one before a colon
// and a port. A clock time is neither: its hour holds no letter and no dot.
func networkAddress() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://|` +
		`(?:[A-Za-z0-9.-]*[A-Za-z.][A-Za-z0-9.-]*|\[[0-9A-Fa-f:.]+\]):[0-9]{1,5}\b`)
}

// clockTime matches a date and a time of day, as 2026-02-30T08:00 or
// tomorrowT10:00 write them, and the character before it, which is not part of
// a host name: the T before its hour would read as a host's letter, so it is
// taken out before networkAddress looks.
func clockTime() *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9.-])(?:[0-9]{4}-[0-9]{2}-[0-9]{2}|[a-z]+)` +
		`T[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?\b`)
}

// refusalWords is what Taskwarrior said in refusing a command: the error's text
// after the refusal's own.
func refusalWords(err error) string {
	_, words, _ := strings.Cut(err.Error(), taskwarrior.ErrRefused.Error()+": ")

	return words
}

// directoryFaults are a directory's failures, in words that never repeat its
// path, which the error itself carries.
func directoryFaults() []faultClass {
	return []faultClass{
		{causes: []error{workdirs.ErrNotFound}, code: api.ProblemCodeNotFound, detail: "there is no such directory"},
		{
			causes: []error{workdirs.ErrNotADirectory, workdirs.ErrNotAbsolute, store.ErrNotADirectoryPath},
			code:   api.ProblemCodeUnprocessable, detail: "that is not an absolute path to a directory",
		},
		{
			causes: []error{workdirs.ErrUnreadable}, code: api.ProblemCodeUnprocessable,
			detail: "that directory cannot be read; check its permissions",
		},
		{
			causes: []error{errFavoritesNotKept, errNoSwitching}, code: api.ProblemCodeUnprocessable,
			detail: "that is not available here: the store is turned off, or the server cannot switch",
		},
		{
			causes: []error{store.ErrKeptSchemaDiffers}, code: api.ProblemCodeUnprocessable,
			detail: store.ErrKeptSchemaDiffers.Error(),
		},
		{
			causes: []error{ErrConfigurationUnreadable}, code: api.ProblemCodeUnprocessable,
			detail: "the configuration there did not load; workflow doctor there says why",
		},
		{
			causes: []error{ErrConfigurationRefused}, code: api.ProblemCodeUnprocessable,
			detail: "the configuration there binds keys workflow refuses; workflow doctor there names them",
		},
	}
}
