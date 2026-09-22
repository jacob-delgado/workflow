// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
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

// codeMeaning is the HTTP status and human title fixed for each problem code.
func codeMeaning(code api.ProblemCode) (int, string) {
	switch code {
	case api.BadRequest:
		return http.StatusBadRequest, "Bad request"
	case api.NotFound:
		return http.StatusNotFound, "Not found"
	case api.Conflict:
		return http.StatusConflict, "Conflict"
	case api.Unprocessable:
		return http.StatusUnprocessableEntity, "Unprocessable content"
	case api.Unreachable:
		return http.StatusBadGateway, "Upstream unreachable"
	case api.Internal:
		return http.StatusInternalServerError, "Internal error"
	default:
		return http.StatusInternalServerError, "Internal error"
	}
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
// rather than a typed response: an opaque 500, so an unexpected failure never
// leaks its detail.
func writeResponseError(w http.ResponseWriter, _ *http.Request, _ error) {
	writeProblem(w, api.Internal, "something went wrong")
}

// fault maps a seam's error onto an RFC 9457 problem and its status. The class
// comes from the error — a missing resource, an unreachable upstream, or an
// unexpected failure — but the detail is curated and safe: the raw cause carries
// a host or a credential and never reaches the wire.
func fault(err error) (api.Problem, int) {
	prob := faultProblem(err)

	return prob, prob.Status
}

// faultProblem classifies a seam's error into the problem shown for it.
func faultProblem(err error) api.Problem {
	switch {
	case errors.Is(err, jira.ErrNotFound), errors.Is(err, forge.ErrNoRepository):
		return problem(api.NotFound, "the requested resource was not found")
	case errors.Is(err, jira.ErrUnreachable), errors.Is(err, forge.ErrUnreachable):
		return problem(api.Unreachable, "the service could not be reached")
	default:
		return problem(api.Internal, "the request could not be completed")
	}
}
