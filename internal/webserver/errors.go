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
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
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
		api.BadRequest:           {status: http.StatusBadRequest, title: "Bad request"},
		api.NotFound:             {status: http.StatusNotFound, title: "Not found"},
		api.MethodNotAllowed:     {status: http.StatusMethodNotAllowed, title: "Method not allowed"},
		api.Conflict:             {status: http.StatusConflict, title: "Conflict"},
		api.Unprocessable:        {status: http.StatusUnprocessableEntity, title: "Unprocessable content"},
		api.PreconditionRequired: {status: http.StatusPreconditionRequired, title: "Precondition required"},
		api.Unreachable:          {status: http.StatusBadGateway, title: "Upstream unreachable"},
		api.Internal:             {status: http.StatusInternalServerError, title: "Internal error"},
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
	writeProblem(w, api.BadRequest, "the request could not be understood")
}

// writeResponseError is the safety net for a handler that returns an error
// rather than a typed response, or an answer that could not be written: an
// opaque 500, so an unexpected failure never leaks its detail, that still says
// what to do. The error goes to Unexpected, since the answer carries none of it.
func (s *server) writeResponseError(w http.ResponseWriter, _ *http.Request, err error) {
	s.unexpected(err)
	writeProblem(w, api.Internal, "the server could not answer; "+tryAgain)
}

// tryAgain is what to do about a failure nothing more is known of.
const tryAgain = "try again, and run workflow doctor if it keeps failing"

// fault maps a seam's error onto an RFC 9457 problem and its status. The class
// comes from the error — one of faultClasses (no repository to work in, a
// missing resource, an upstream that could not be reached, asked to wait or
// answered oddly, a setting it cannot use, a service's refusal) or else an
// unexpected failure — but the detail is curated and safe: the raw cause
// carries a host, a path, a webhook or a credential and never reaches the wire.
// An unexpected failure's cause goes to Unexpected instead, since no class
// says what it was.
func (s *server) fault(err error) (api.Problem, int) {
	prob, classified := faultProblem(err)
	if !classified {
		s.unexpected(err)
	}

	return prob, prob.Status
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
	for _, class := range faultClasses() {
		if slices.ContainsFunc(class.causes, func(cause error) bool { return errors.Is(err, cause) }) {
			return problem(class.code, class.detail), true
		}
	}

	return problem(api.Internal, "the request could not be completed; "+tryAgain), false
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
		taskwarriorFaults())
}

// gitFaults are the repository's failures fault can name. Any other read git
// could not answer is left to the internal problem: its words can name where
// the repository is on disk, and the caller cannot act on them.
func gitFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{gitrepo.ErrNotARepository},
			code:   api.Conflict,
			detail: "the server is not running in a git repository; start workflow --web from a repository's work tree",
		},
	}
}

// transportFaults are the failures any upstream can answer with.
func transportFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{jira.ErrNotFound, forge.ErrNoRepository},
			code:   api.NotFound, detail: "the requested resource was not found",
		},
		{
			causes: []error{jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable},
			code:   api.Unreachable, detail: "the service could not be reached; check the network, then try again",
		},
		// Every client answers a 429 and a redirect with the transport's own
		// sentinels, so these details name no service.
		{
			causes: []error{httpx.ErrRateLimited},
			code:   api.Unreachable, detail: "the service is limiting requests; wait and try again",
		},
		{
			causes: []error{httpx.ErrRedirected},
			code:   api.Unreachable,
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
			code:   api.Unprocessable,
			detail: "Jira has no token; set jira.token, or check that jira.token_command or jira.token_env gives one " +
				"— workflow doctor --online tests it",
		},
		{
			causes: []error{config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL},
			code:   api.Unprocessable,
			detail: "jira.base_url is not a usable address; workflow doctor checks it",
		},
		// Before the credential's class: a 403 Jira explained answers to both, and
		// doctor, which asks only who the token is, would pass a token that lacks
		// one permission.
		{
			causes: []error{jira.ErrRejected},
			code:   api.Unprocessable, detail: "Jira refused the request",
		},
		{
			causes: []error{jira.ErrUnauthorized, jira.ErrForbidden},
			code:   api.Unprocessable,
			detail: "Jira did not accept the configured credential; workflow doctor checks it",
		},
		{
			causes: []error{jira.ErrNoAPI},
			code:   api.Unprocessable, detail: "no Jira API answers at jira.base_url; workflow doctor checks it",
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
			code:   api.Unprocessable,
			// Not the resolver's own words: for a host other than github.com or
			// gitlab.com they name the host, which a detail never does.
			detail: "no forge token was found; for GitHub set $GITHUB_TOKEN or sign in with gh, for GitLab set " +
				"$GITLAB_TOKEN, or set forge.token; workflow doctor names where it looks for this repository",
		},
		{
			causes: []error{forge.ErrKindNeedsHost},
			code:   api.Unprocessable, detail: "forge.kind is set without forge.host; set the host it describes",
		},
		{
			causes: []error{forge.ErrUnauthorized},
			code:   api.Unprocessable,
			detail: "the forge did not accept the token, which may have expired; workflow doctor --online checks it",
		},
		{
			causes: []error{forge.ErrNoAPI, forge.ErrNotJSON},
			code:   api.Unprocessable,
			detail: "no forge API answered; check forge.host, which workflow doctor --online tests",
		},
		{
			causes: []error{forge.ErrRefused},
			code:   api.Unprocessable,
			detail: "the forge refused the request; the token may lack a permission this needs, so check its scopes",
		},
		{
			// An explained status is the same undocumented status with the
			// forge's reason, which stays off the wire.
			causes: []error{forge.ErrUnexpectedStatus, forge.ErrRejected},
			code:   api.Unreachable, detail: "the forge answered with a status it does not document; try again",
		},
	}
}

// messagingFaults are the messaging service's failures, told the same whichever
// service it is. None forwards the error's own text: a client's error can name
// the webhook, whose address is its credential.
func messagingFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{messaging.ErrNoCredential},
			code:   api.Unprocessable,
			detail: "messaging has no credential; add a token or a webhook URL in Settings, " +
				"or check that messaging.token_command or messaging.token_env gives one",
		},
		{
			causes: []error{messaging.ErrInsecureWebhook},
			code:   api.Unprocessable,
			detail: "messaging.webhook_url is not an https address; copy the webhook's https address into it in Settings",
		},
		// Every 4xx a post meets is this one, and a webhook — the only transport
		// Teams, Discord and a plain webhook have — is one workflow doctor cannot
		// check, so the detail points at the settings rather than at doctor.
		{
			causes: []error{messaging.ErrRejected},
			code:   api.Unprocessable,
			detail: "the messaging service refused the announcement; " +
				"check the token, or that the webhook URL is current, in Settings",
		},
		{
			causes: []error{messaging.ErrPostRefused},
			code:   api.Unprocessable,
			detail: "the messaging service refused the message; announce from a terminal to see its reason",
		},
		{
			causes: []error{messaging.ErrUnexpectedStatus},
			code:   api.Unreachable,
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
			faultClass{causes: []error{reason.cause}, code: api.Unprocessable, detail: reason.text})
	}

	return slices.Concat([]faultClass{
		{
			causes: []error{errStartDeclined},
			code:   api.Conflict, detail: "the task is already started, or no such task exists",
		},
		{
			causes: []error{taskwarrior.ErrNothingChanged},
			code:   api.Conflict, detail: "the task is already in that state, or is no longer pending",
		},
		{
			causes: []error{taskwarrior.ErrNoSync},
			code:   api.Unprocessable,
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
			cause: taskwarrior.ErrNotInstalled, code: api.NotInstalled,
			text: "Taskwarrior is not installed, or no task program is on PATH. Install Taskwarrior " +
				taskwarrior.MinimumVersion + " or newer, or set taskwarrior.program.",
		},
		{
			cause: taskwarrior.ErrNotTaskwarrior, code: api.NotTaskwarrior,
			text: "The task on PATH is another program (go-task, most likely), not Taskwarrior.",
		},
		{
			cause: taskwarrior.ErrTooOld, code: api.TooOld,
			text: "Taskwarrior is too old: " + taskwarrior.MinimumVersion +
				" or newer is needed; workflow doctor shows the version found.",
		},
		{
			cause: taskwarrior.ErrNotConfigured, code: api.NeverRun,
			text: "Taskwarrior has never been run: run it once in a terminal so it creates its configuration; " +
				"workflow doctor names the program.",
		},
	}
}

// taskFault is fault for what a Taskwarrior seam answered. A refusal carries
// Taskwarrior's own words, which name what in the line it could not take, and a
// bound that ran out is Taskwarrior's: no class can say so, since git's reads
// time out with the same sentinel.
func (s *server) taskFault(err error) (api.Problem, int) {
	var prob api.Problem

	switch {
	case errors.Is(err, taskwarrior.ErrRefused):
		prob = problem(api.Unprocessable, s.refusalDetail(err))
	case errors.Is(err, proc.ErrTimedOut):
		prob = problem(api.Unreachable, "Taskwarrior did not answer in time")
	default:
		return s.fault(err)
	}

	return prob, prob.Status
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
