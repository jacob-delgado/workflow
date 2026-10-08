// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
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
		api.ProblemCodeRateLimited:          {status: http.StatusServiceUnavailable, title: "Rate limited"},
		api.ProblemCodeFetchFailed:          {status: http.StatusBadGateway, title: "Fetch failed"},
		api.ProblemCodeCheckFailed:          {status: http.StatusUnprocessableEntity, title: "Check failed"},
		api.ProblemCodeInternal:             {status: http.StatusInternalServerError, title: "Internal error"},
	}[code]

	return meaning.status, meaning.title
}

// problemAnswer is an operation's default answer with prob: the problem, at
// its own status, and a rate limit's wait as the Retry-After header. Every
// operation's default answer has this one shape, under its own type.
func problemAnswer[Answer ~struct {
	Body       api.Problem
	Headers    api.ProblemResponseHeaders
	StatusCode int
}](prob api.Problem) Answer {
	return Answer{Body: prob, Headers: api.ProblemResponseHeaders{RetryAfter: prob.RetryAfter}, StatusCode: prob.Status}
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
			prob := problem(setUpCode(class.code, err), class.detail)
			prob.RetryAfter = askedWait(err)

			return prob, true
		}
	}

	return problem(api.ProblemCodeInternal, "the request could not be completed; "+tryAgain), false
}

// askedWait is how many seconds an upstream's rate limit asked to wait, when it
// said; nil for any other failure.
func askedWait(err error) *int {
	limited, ok := errors.AsType[*httpx.RateLimitError](err)
	if !ok {
		return nil
	}

	return new(int(limited.Wait / time.Second))
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
