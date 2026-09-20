// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guardLoopback rejects a request whose Host names anything but the loopback
// interface. The server binds 127.0.0.1 only, yet a browser can still be aimed
// at it by a page whose own hostname has been rebound to 127.0.0.1 (DNS
// rebinding): the request reaches the loopback socket carrying that foreign
// hostname in Host. Requiring a loopback Host closes that path while leaving
// 127.0.0.1, localhost and ::1 reachable on any port, so an ephemeral-port test
// still passes. The rejection is plain text, not the API error envelope: a
// request that fails this gate is not a request from the local client the API
// serves.
func guardLoopback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !isLoopbackHost(request.Host) {
			http.Error(w, "the web interface serves the loopback interface only", http.StatusForbidden)

			return
		}

		if !allowedOrigin(request) {
			http.Error(w, "cross-origin writes are refused", http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, request)
	})
}

// refuseWritesInDryRun makes the whole surface read-only when the server runs
// with --dry-run: a state-changing request is answered 403 without reaching a
// handler, so no write — a checkout, a commit, a push — happens. When dry-run is
// off it adds nothing. This is the web's dry-run: the terminal interface instead
// simulates each write, but a read-only surface keeps the guarantee that matters
// — nothing is written — with one gate over every write.
func refuseWritesInDryRun(dryRun bool, next http.Handler) http.Handler {
	if !dryRun {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !isSafeMethod(request.Method) {
			http.Error(w, "the web interface is read-only while --dry-run is set", http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, request)
	})
}

// allowedOrigin guards a state-changing request against a cross-origin caller: a
// write carrying an Origin header must name the server's own origin — the same
// host and port the request was sent to. A read (a safe method), or a write with
// no Origin — curl, the event stream — is allowed. Requiring the exact origin,
// not merely "some loopback host", closes the path a co-resident page on another
// loopback port (a dev server, a served file) would otherwise take to the local
// server with a no-preflight POST; the browser always attaches Origin to a
// cross-origin write, and the app itself is always same-origin.
func allowedOrigin(request *http.Request) bool {
	if isSafeMethod(request.Method) {
		return true
	}

	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Host, request.Host)
}

// isSafeMethod reports whether method only reads, so it needs no write guard.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isLoopbackHost reports whether host (an HTTP Host header) names the loopback
// interface: the name "localhost" or any loopback IP literal, on any port.
// Anything else — including an empty or malformed Host, which no legitimate
// request to the loopback server carries — is refused.
func isLoopbackHost(host string) bool {
	name := host

	hostname, _, err := net.SplitHostPort(host)
	if err == nil {
		name = hostname
	}

	name = strings.ToLower(name)
	if name == "localhost" {
		return true
	}

	if ip := net.ParseIP(name); ip != nil {
		return ip.IsLoopback()
	}

	return false
}
