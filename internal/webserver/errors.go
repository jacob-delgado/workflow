// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/report"
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
// map, not a switch, so there is no last-case arm gobco can never see;
// exhaustive keeps it complete, and every code is one of the spec's constants.
func codeMeaning(code api.ProblemCode) (int, string) {
	meaning := map[api.ProblemCode]struct {
		status int
		title  string
	}{
		api.ProblemCodeBadRequest:           {status: http.StatusBadRequest, title: "Bad request"},
		api.ProblemCodeUnauthorized:         {status: http.StatusUnauthorized, title: "Unauthorized"},
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
	writeProblem(w, api.ProblemCodeInternal, "the server could not answer; "+report.TryAgain)
}

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

// unexpected hands a failure the answer leaves out to Unexpected, with the
// configuration in effect, when it is wired.
func (s *server) unexpected(err error) {
	if s.deps.Unexpected != nil {
		s.deps.Unexpected(s.config(), err)
	}
}

// faultProblem classifies a seam's error into the problem shown for it: the
// first of faultClasses it belongs to, reported true, or an opaque internal
// error, reported false.
func faultProblem(err error) (api.Problem, bool) {
	told, classified := report.Classify(err, faultClasses())

	prob := problem(told.Code, told.Detail)
	prob.RetryAfter = told.RetryAfter

	return prob, classified
}
