// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
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

// faultClasses are the failures fault tells apart: those every surface tells
// alike, report.SharedFaults, with the server's own placed among them where
// they belong, the more specific first, and a request whose context ended
// ahead of all, whatever the seam's error wraps beside it.
func faultClasses() []report.Class {
	shared := report.SharedFaults()

	return slices.Concat(endedFaults(), repositoryFaults(), shared.Git, branchFaults(), shared.Transport,
		shared.Jira, shared.Forge, shared.Messaging, taskWriteFaults(), shared.Taskwarrior, shared.Directory,
		switchFaults(), localDataFaults(), peopleFaults())
}

// endedFaults are a request whose context ended before it was answered: its
// page has gone, or the server is stopping. No one reads the answer and
// nothing failed, so it is classed, first, and never handed to Unexpected.
func endedFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{context.Canceled}, Code: api.ProblemCodeInternal,
			Detail: "the request ended before it was answered; " + report.TryAgain,
		},
	}
}

// repositoryFaults are git's failures the server words itself: a server
// started outside any work tree is told how to start one inside, ahead of the
// words every surface shares.
func repositoryFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{gitrepo.ErrNotARepository}, Code: api.ProblemCodeConflict,
			Detail: "the server is not running in a git repository; start workflow --web from a repository's work tree",
		},
	}
}

// branchFaults are the server's own refusals of a new branch.
func branchFaults() []report.Class {
	return []report.Class{
		{Causes: []error{errBranchExists}, Code: api.ProblemCodeConflict, Detail: errBranchExists.Error()},
	}
}

// taskWriteFaults are Taskwarrior's writes the server words itself: a start
// it declined, ahead of Taskwarrior's own classes, since it has words of its
// own.
func taskWriteFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{errStartDeclined},
			Code:   api.ProblemCodeConflict, Detail: "the task is already started, or no such task exists",
		},
	}
}

// switchFaults are the server's refusals of a favorite or a switch to another
// directory, in words that never repeat its path, which the error itself
// carries.
func switchFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{errFavoritesNotKept, errNoSwitching}, Code: api.ProblemCodeUnprocessable,
			Detail: "that is not available here: the store is turned off, or the server cannot switch",
		},
		{
			Causes: []error{ErrConfigurationUnreadable}, Code: api.ProblemCodeUnprocessable,
			Detail: "the configuration there did not load; workflow doctor there says why",
		},
		{
			Causes: []error{ErrConfigurationRefused}, Code: api.ProblemCodeUnprocessable,
			Detail: "the configuration there binds keys workflow refuses; workflow doctor there names them",
		},
	}
}
