// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	apispec "github.com/jacob-delgado/workflow/api"
	"github.com/jacob-delgado/workflow/internal/api"
)

// loadSpec parses the embedded OpenAPI document and validates it. A spec that
// does not parse or validate is a build defect rather than a runtime condition,
// so the error is returned to Handler to surface at startup, not swallowed.
func loadSpec() (*openapi3.T, error) {
	loader := openapi3.NewLoader()

	doc, err := loader.LoadFromData(apispec.Spec)
	if err != nil {
		return nil, fmt.Errorf("loading the embedded OpenAPI spec: %w", err)
	}

	err = doc.Validate(loader.Context)
	if err != nil {
		return nil, fmt.Errorf("validating the embedded OpenAPI spec: %w", err)
	}

	return doc, nil
}

// validate returns middleware that checks each request against the contract
// before it reaches a handler, answering one that does not match with the house
// error envelope. Host validation is off: the one server is the loopback URL,
// and which host a client uses to reach the loopback is not the spec's concern.
func validate(doc *openapi3.T) func(http.Handler) http.Handler {
	return nethttpmiddleware.OapiRequestValidatorWithOptions(doc, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: writeValidationError,
	})
}

// writeValidationError answers a request the contract rejects. An unknown route
// is a not-found; every other mismatch — a bad parameter, a body that does not
// fit the schema — is a bad request. The specific validation detail stays off
// the wire; that the request did not match the contract is what the caller acts
// on.
func writeValidationError(
	_ context.Context, _ error, w http.ResponseWriter, _ *http.Request, opts nethttpmiddleware.ErrorHandlerOpts,
) {
	if opts.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, api.NotFound, "no such endpoint")

		return
	}

	writeError(w, http.StatusBadRequest, api.BadRequest, "the request did not match the API contract")
}
