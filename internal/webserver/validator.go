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
	// router routes by path alone: Host validation is off, since the one
	// server is the loopback URL, and which host a client uses to reach the
	// loopback is not the spec's concern.
	router routers.Router

	// options admit only a request presenting the session, as the spec's
	// security schemes say.
	options openapi3filter.Options

	// methods are every method an operation in the spec takes, the ones worth
	// asking the router about when a path is asked with another.
	methods []string
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

	doc.Servers = nil

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return contract{}, fmt.Errorf("routing the embedded OpenAPI spec: %w", err)
	}

	return contract{
		router:  router,
		options: openapi3filter.Options{AuthenticationFunc: session.admits},
		methods: contractMethods(doc.Paths),
	}, nil
}

// operationServer serves a request the contract admitted, told whether the
// operation it asks for is one the spec marks as a switch: one that changes
// what the server works with — the directory, or a first configuration file
// set up.
type operationServer func(writer http.ResponseWriter, request *http.Request, switches bool)

// admit checks each request against the contract, routing it once, before
// serve takes it; one the contract does not admit is answered here, with the
// house error envelope.
func (c contract) admit(serve operationServer) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		route, parameters, err := c.router.FindRoute(request)
		if err != nil {
			c.refuseUnrouted(writer, request, err)

			return
		}

		err = openapi3filter.ValidateRequest(request.Context(), &openapi3filter.RequestValidationInput{
			Request: request, PathParams: parameters, Route: route, Options: &c.options,
		})
		if err != nil {
			refuseInvalid(writer, err)

			return
		}

		switched, _ := route.Operation.Extensions[switchExtension].(bool)

		serve(writer, request, switched)
	})
}

// refuseUnrouted answers a request the contract routes nowhere. A known path
// asked with a method it declares no operation for is not allowed, and Allow
// lists the methods it does; an unknown path is a not-found.
func (c contract) refuseUnrouted(writer http.ResponseWriter, request *http.Request, err error) {
	if !errors.Is(err, routers.ErrMethodNotAllowed) {
		writeProblem(writer, api.ProblemCodeNotFound, "no such endpoint")

		return
	}

	allowed := strings.Join(allowedMethods(request.Context(), c.router, c.methods, request), ", ")
	writer.Header().Set("Allow", allowed)
	writeProblem(writer, api.ProblemCodeMethodNotAllowed, "this endpoint does not answer that method; it answers "+allowed)
}

// refuseInvalid answers a request routed to an operation it does not fit. One
// that presents no session of this run is unauthorized, before anything else
// about it is checked; every other mismatch — a bad parameter, a body that
// does not fit the schema — is a bad request. The specific validation detail
// stays off the wire; that the request did not match the contract is what the
// caller acts on.
func refuseInvalid(writer http.ResponseWriter, err error) {
	if _, unauthorized := errors.AsType[*openapi3filter.SecurityRequirementsError](err); unauthorized {
		writer.Header().Set("WWW-Authenticate", `Bearer realm="workflow"`)
		writeProblem(writer, api.ProblemCodeUnauthorized, noSession)

		return
	}

	writeProblem(writer, api.ProblemCodeBadRequest, "the request did not match the API contract")
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
