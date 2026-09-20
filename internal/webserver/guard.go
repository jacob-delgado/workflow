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

// allowedOrigin guards a state-changing request against a cross-origin caller: a
// write carrying an Origin header must name the loopback interface. A read (a
// safe method), or a write with no Origin — a same-origin request, curl, the
// event stream — is allowed. This closes the cross-site request path a browser
// would otherwise take to the local server, on top of the loopback Host check.
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

	return isLoopbackHost(parsed.Host)
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
