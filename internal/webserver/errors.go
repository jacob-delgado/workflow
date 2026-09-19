// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"net/http"

	"github.com/jacob-delgado/workflow/internal/api"
)

// writeJSON encodes body as the response at status. A failed write to the
// connection is not actionable — the channel to report it on is the one that
// just failed — so it is dropped, the way the CLI drops a failed write to stdout.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body) //nolint:errchkjson // see doc comment; the api types marshal cleanly
}

// writeError writes the house error envelope. The message is safe to show and
// never carries a credential.
func writeError(w http.ResponseWriter, status int, code api.ErrorCode, message string) {
	writeJSON(w, status, api.Error{Code: code, Message: message})
}

// writeRequestError answers a request the generated binder or the strict decoder
// could not read — a bad query parameter or a malformed body. The detail stays
// off the wire; that a field was wrong, not which internal parser said so, is
// what the caller can act on.
func writeRequestError(w http.ResponseWriter, _ *http.Request, _ error) {
	writeError(w, http.StatusBadRequest, api.BadRequest, "the request could not be understood")
}

// writeResponseError is the safety net for a handler that returns an error
// rather than a typed response: an opaque 500, so an unexpected failure never
// leaks its detail.
func writeResponseError(w http.ResponseWriter, _ *http.Request, _ error) {
	writeError(w, http.StatusInternalServerError, api.Internal, "something went wrong")
}

// fault maps a seam's error onto the wire. Domain errors carry no credential,
// but the message is kept generic and stable; the specific cause is not the
// caller's to act on and stays out of the response.
func fault(_ error) (api.Error, int) {
	return api.Error{Code: api.Internal, Message: "the request could not be completed"}, http.StatusInternalServerError
}
