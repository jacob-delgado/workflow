// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package problem is the RFC 9457 problem details the --web API answers a
// failure with: the status, title and type each code fixes, the default
// answer a strict handler returns with one, and one written by the
// hand-written middleware that answers before a strict handler runs.
package problem

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
)

// problemBase is where a problem's type URI points: one anchor per code on the
// errors reference page, so the URI dereferences to what the code means.
const problemBase = "https://jacob-delgado.github.io/workflow/docs/errors/"

// New builds an RFC 9457 problem details object for a code, deriving its
// type, title and status. The detail is safe to show and never carries a secret.
func New(code api.ProblemCode, detail string) api.Problem {
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
		api.ProblemCodeCiFailed:             {status: http.StatusConflict, title: "CI failed"},
		api.ProblemCodeInternal:             {status: http.StatusInternalServerError, title: "Internal error"},
	}[code]

	return meaning.status, meaning.title
}

// Answer is an operation's default answer with prob: the problem, at
// its own status, and a rate limit's wait as the Retry-After header. Every
// operation's default answer has this one shape, under its own type.
func Answer[Response ~struct {
	Body       api.Problem
	Headers    api.ProblemResponseHeaders
	StatusCode int
}](prob api.Problem) Response {
	return Response{Body: prob, Headers: api.ProblemResponseHeaders{RetryAfter: prob.RetryAfter}, StatusCode: prob.Status}
}

// Write writes a problem details object as application/problem+json, for
// the hand-written middleware that answers before a strict handler runs.
func Write(w http.ResponseWriter, code api.ProblemCode, detail string) {
	prob := New(code, detail)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(prob.Status)
	_ = json.NewEncoder(w).Encode(prob) //nolint:errchkjson // the api types marshal cleanly
}

// WriteRequestError answers a request the generated binder or the strict decoder
// could not read — a bad query parameter or a malformed body. The detail stays
// off the wire; that a field was wrong, not which internal parser said so, is
// what the caller can act on.
func WriteRequestError(w http.ResponseWriter, _ *http.Request, _ error) {
	Write(w, api.ProblemCodeBadRequest, "the request could not be understood")
}
