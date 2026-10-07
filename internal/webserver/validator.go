// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	apispec "github.com/jacob-delgado/workflow/api"
	"github.com/jacob-delgado/workflow/internal/api"
)

// loadSpec parses the embedded OpenAPI document and validates it. A spec that
// does not parse or validate is a build defect rather than a runtime condition,
// so the error is returned to Handler to surface at startup, not swallowed.
//
// Trade-off TRADE-14: no test runs on a broken spec, so neither error arm runs.
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

// validate returns middleware that checks each request against the embedded
// contract before it reaches a handler, answering one that does not match with
// the house error envelope. Host validation is off: the one server is the
// loopback URL, and which host a client uses to reach the loopback is not the
// spec's concern. Like a spec that does not load, one whose paths cannot be
// routed is a build defect, returned for Handler to surface at startup.
//
// Trade-off TRADE-14: no test runs on a broken spec, so neither error arm runs.
func validate() (func(http.Handler) http.Handler, error) {
	doc, err := loadSpec()
	if err != nil {
		return nil, err
	}

	// Routed by path alone, as the validator routes with Host validation off, so
	// the methods this router finds at a path are the ones the validator admits.
	router, err := gorillamux.NewRouter(&openapi3.T{Paths: doc.Paths})
	if err != nil {
		return nil, fmt.Errorf("routing the embedded OpenAPI spec: %w", err)
	}

	return nethttpmiddleware.OapiRequestValidatorWithOptions(doc, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: answerValidationError(router, contractMethods(doc.Paths)),
	}), nil
}

// answerValidationError answers a request the contract rejects. A known path
// asked with a method it declares no operation for is not allowed, and Allow
// lists the methods it does; an unknown path is a not-found; every other
// mismatch — a bad parameter, a body that does not fit the schema — is a bad
// request. The specific validation detail stays off the wire; that the request
// did not match the contract is what the caller acts on.
func answerValidationError(router routers.Router, methods []string) nethttpmiddleware.ErrorHandlerWithOpts {
	return func(
		ctx context.Context, err error, w http.ResponseWriter, request *http.Request, opts nethttpmiddleware.ErrorHandlerOpts,
	) {
		switch {
		case errors.Is(err, routers.ErrMethodNotAllowed):
			allowed := strings.Join(allowedMethods(ctx, router, methods, request), ", ")
			w.Header().Set("Allow", allowed)
			writeProblem(w, api.ProblemCodeMethodNotAllowed, "this endpoint does not answer that method; it answers "+allowed)
		case opts.StatusCode == http.StatusNotFound:
			writeProblem(w, api.ProblemCodeNotFound, "no such endpoint")
		default:
			writeProblem(w, api.ProblemCodeBadRequest, "the request did not match the API contract")
		}
	}
}

// contractMethods is every method an operation in paths takes, sorted, without
// repeats: the methods worth asking the router about at any one path.
func contractMethods(paths *openapi3.Paths) []string {
	var methods []string

	for _, item := range paths.Map() {
		methods = slices.AppendSeq(methods, maps.Keys(item.Operations()))
	}

	slices.Sort(methods)

	return slices.Compact(methods)
}

// allowedMethods is those of methods the router finds an operation for at the
// path request names.
func allowedMethods(ctx context.Context, router routers.Router, methods []string, request *http.Request) []string {
	return slices.DeleteFunc(slices.Clone(methods), func(method string) bool {
		probe := request.Clone(ctx)
		probe.Method = method

		_, _, err := router.FindRoute(probe)

		return err != nil
	})
}
