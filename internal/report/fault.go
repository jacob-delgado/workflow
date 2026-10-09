// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package report builds what every surface reports in the API's own shapes,
// internal/api's: the Summary of a period, the directories you work in, the
// review queue, and the words for a failure. The web server answers with
// them and the command line prints them for --json, so the two cannot come
// to say the same thing differently. It reads the seams' answers and does no
// I/O of its own.
package report

import (
	"errors"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// TryAgain is what to do about a failure nothing more is known of.
const TryAgain = "try again, and run workflow doctor if it keeps failing"

// Class is one kind of failure: the sentinels that belong to it, and the
// problem code and the sentence it is told with — a sentence that says what
// to do, never the cause's own text, which can carry the tracker's or the
// forge's address.
type Class struct {
	Causes []error
	Code   api.ProblemCode
	Detail string
}

// Matches reports whether err is one of the class's causes.
func (c Class) Matches(err error) bool {
	return slices.ContainsFunc(c.Causes, func(cause error) bool { return errors.Is(err, cause) })
}

// Fault is a failure as it is told: the problem code the web answers with,
// the sentence every surface shows, and, for a rate limit that said, how many
// seconds it asked to wait.
type Fault struct {
	Code       api.ProblemCode
	Detail     string
	RetryAfter *int
}

// Classify tells err as the first of classes it belongs to, reported true, or
// as an opaque internal failure, reported false. A token the forge turned
// down and a message Slack would not deliver are told in their own words
// before any class: neither carries an address.
func Classify(err error, classes []Class) (Fault, bool) {
	// A token the forge turned down is told in forge.Advice's words, the ones
	// the terminal shows: the forge, the scope it asks for, and its own reason
	// for the refusal, which is the forge's and never carries its address.
	if advice, ok := forge.Advice(err); ok {
		return Fault{Code: api.ProblemCodeUnprocessable, Detail: advice, RetryAfter: nil}, true
	}

	// A message Slack would not deliver carries the fix for its channel, which
	// names a channel and Slack's code but never an address.
	if refused, ok := errors.AsType[messaging.PostRefusedError](err); ok {
		return Fault{Code: api.ProblemCodeUnprocessable, Detail: refused.Error(), RetryAfter: nil}, true
	}

	for _, class := range classes {
		if class.Matches(err) {
			return Fault{Code: setUpCode(class.Code, err), Detail: class.Detail, RetryAfter: askedWait(err)}, true
		}
	}

	return Fault{
		Code: api.ProblemCodeInternal, Detail: "the request could not be completed; " + TryAgain, RetryAfter: nil,
	}, false
}

// FaultDetail is a failure's detail in the words the shared classes
// (SharedFaults) give it: the same sentence for the same class of failure,
// never naming a host or a path. A surface's own classes may come before
// them, as the web's do, and word a failure of that class differently.
func FaultDetail(err error) string {
	told, _ := Classify(err, SharedFaults().All())

	return told.Detail
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
