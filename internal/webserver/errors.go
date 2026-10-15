// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// writeResponseError is the safety net for a handler that returns an error
// rather than a typed response, or an answer that could not be written: an
// opaque 500, so an unexpected failure never leaks its detail, that still says
// what to do. The error goes to Unexpected, since the answer carries none of it.
func (s *server) writeResponseError(w http.ResponseWriter, _ *http.Request, err error) {
	s.unexpected(err)
	problem.Write(w, api.ProblemCodeInternal, "the server could not answer; "+report.TryAgain)
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
// configuration in effect, when it is wired. A failure that is only its
// context ending, its page gone or the server stopping, is kept from it
// wherever it is met: nothing went wrong to hear of.
func (s *server) unexpected(err error) {
	if s.deps.Unexpected == nil || errors.Is(err, context.Canceled) {
		return
	}

	s.deps.Unexpected(s.config(), err)
}

// faultProblem classifies a seam's error into the problem shown for it: the
// first of faultClasses it belongs to, reported true, or an opaque internal
// error, reported false.
func faultProblem(err error) (api.Problem, bool) {
	told, classified := report.Classify(err, faultClasses())

	prob := problem.New(told.Code, told.Detail)
	prob.RetryAfter = told.RetryAfter

	return prob, classified
}
