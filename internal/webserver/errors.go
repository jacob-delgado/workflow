// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
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
	case api.PreconditionRequired:
		return http.StatusPreconditionRequired, "Precondition required"
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
// rather than a typed response, or an answer that could not be written: an
// opaque 500, so an unexpected failure never leaks its detail, that still says
// what to do.
func writeResponseError(w http.ResponseWriter, _ *http.Request, _ error) {
	writeProblem(w, api.Internal, "the server could not answer; "+tryAgain)
}

// tryAgain is what to do about a failure nothing more is known of.
const tryAgain = "try again, and run workflow doctor if it keeps failing"

// fault maps a seam's error onto an RFC 9457 problem and its status. The class
// comes from the error — one of faultClasses (a missing resource, an upstream
// that could not be reached, asked to wait or answered oddly, a setting it
// cannot use, a service's refusal) or else an unexpected failure — but the
// detail is curated and safe: the raw cause carries a host, a webhook or a
// credential and never reaches the wire.
func fault(err error) (api.Problem, int) {
	prob := faultProblem(err)

	return prob, prob.Status
}

// faultProblem classifies a seam's error into the problem shown for it: the
// first class it belongs to, or an opaque internal error.
func faultProblem(err error) api.Problem {
	for _, class := range faultClasses() {
		if slices.ContainsFunc(class.causes, func(cause error) bool { return errors.Is(err, cause) }) {
			return problem(class.code, class.detail)
		}
	}

	return problem(api.Internal, "the request could not be completed; "+tryAgain)
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
	return slices.Concat(transportFaults(), jiraFaults(), forgeFaults(), messagingFaults())
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
			causes: []error{jira.ErrInvalidBaseURL, jira.ErrCredentialInBaseURL},
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
			detail: "no forge token was found; sign in with gh or glab, or set forge.token, which workflow doctor checks",
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
