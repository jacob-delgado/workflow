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
	"github.com/getkin/kin-openapi/openapi3filter"
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

// switchExtension marks an operation in the spec that switches what the
// server works with, which takes the write gate alone.
const switchExtension = "x-switch"

// contract is the embedded spec, ready to check requests against and to say
// which operation a request asks for.
type contract struct {
	// validate is middleware that checks each request against the spec before
	// it reaches a handler, answering one that does not match with the house
	// error envelope. Host validation is off: the one server is the loopback
	// URL, and which host a client uses to reach the loopback is not the spec's
	// concern.
	validate func(http.Handler) http.Handler

	// router routes by path alone, as the validator routes with Host
	// validation off, so the methods it finds at a path are the ones the
	// validator admits.
	router routers.Router
}

// loadContract loads the embedded spec and routes its paths, admitting a
// request only where it presents session as the spec's security schemes say.
// Like a spec that does not load, one whose paths cannot be routed is a build
// defect, returned for Handler to surface at startup.
//
// Trade-off TRADE-14: no test runs on a broken spec, so neither error arm runs.
func loadContract(session Session) (contract, error) {
	doc, err := loadSpec()
	if err != nil {
		return contract{}, err
	}

	router, err := gorillamux.NewRouter(&openapi3.T{Paths: doc.Paths})
	if err != nil {
		return contract{}, fmt.Errorf("routing the embedded OpenAPI spec: %w", err)
	}

	validate := nethttpmiddleware.OapiRequestValidatorWithOptions(doc, &nethttpmiddleware.Options{
		Options:              openapi3filter.Options{AuthenticationFunc: session.admits},
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: answerValidationError(router, contractMethods(doc.Paths)),
	})

	return contract{validate: validate, router: router}, nil
}

// switches reports a request for an operation the spec marks as a switch: one
// that changes what the server works with — the directory, or a first
// configuration file set up. A request the spec routes nowhere switches
// nothing; the validator answers it.
func (c contract) switches(request *http.Request) bool {
	route, _, err := c.router.FindRoute(request)
	if err != nil {
		return false
	}

	switched, _ := route.Operation.Extensions[switchExtension].(bool)

	return switched
}

// answerValidationError answers a request the contract rejects. A known path
// asked with a method it declares no operation for is not allowed, and Allow
// lists the methods it does; an unknown path is a not-found; a request that
// presents no session of this run is unauthorized, before anything else about
// it is checked; every other mismatch — a bad parameter, a body that does not
// fit the schema — is a bad request. The specific validation detail stays off
// the wire; that the request did not match the contract is what the caller
// acts on.
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
		case opts.StatusCode == http.StatusUnauthorized:
			w.Header().Set("WWW-Authenticate", `Bearer realm="workflow"`)
			writeProblem(w, api.ProblemCodeUnauthorized, noSession)
		default:
			writeProblem(w, api.ProblemCodeBadRequest, "the request did not match the API contract")
		}
	}
}

// noSession is what a request presenting no session of this run is told.
const noSession = "this request presented no session of the running workflow --web; " +
	"open the address it printed as it started, which carries one"

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
