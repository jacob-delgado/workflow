// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package edge is the request boundary of `workflow --web`: what every request
// passes before the server's own handlers take it. It refuses a request whose
// Host is not the loopback interface and a cross-origin write, makes the
// surface read-only under --dry-run, gives every answer the content policy,
// checks each API request against the embedded OpenAPI contract and the
// session the run made, and serves the embedded single-page app under every
// other path. It changes for security's reasons, and the server's handlers do
// not.
package edge

import (
	"io/fs"
	"net/http"
)

// Handler is the request boundary around serve: the API under /api, checked
// against the contract by the request validator, which admits only a request
// presenting session before serve takes it, and the embedded single-page app,
// assets, under every other path. The whole surface is behind the loopback
// guard, so a browser aimed at the server from a foreign origin is refused,
// read-only when dryRun is set, and every answer carries the content policy,
// so no other page can frame the app. It fails when the embedded spec cannot
// be loaded or routed, which is a build defect.
func Handler(dryRun bool, session Session, assets fs.FS, serve OperationServer) (http.Handler, error) {
	// Trade-off TRADE-14: the embedded spec loads in every build a test runs.
	spec, err := routedContract()
	if err != nil {
		return nil, err
	}

	// The API is validated against the contract before serve takes a request,
	// so a request presenting no session learns nothing of what serve would
	// do with it; the app is not, since its paths are not in the spec, so only
	// the /api subtree passes through the validator.
	root := http.NewServeMux()
	root.Handle("/api/", spec.admitting(session).admit(serve))
	root.Handle("/", spaHandler(assets))

	return withPolicyHeaders(guardLoopback(refuseWritesInDryRun(dryRun, root))), nil
}
