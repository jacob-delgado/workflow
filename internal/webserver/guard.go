// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// errNoSession is a request that presented no session, or another run's.
var errNoSession = errors.New("the request presented no session of this run")

// sessionField names the session where a request or an address carries it
// beside other things: in the event stream's query, as the contract's
// sessionQuery scheme names it, and in the fragment of the page's address.
const sessionField = "session"

// Session admits a request to the API: a secret one run of the server makes
// as it starts and hands to its page in the address it prints, so a program
// on this machine that was never shown that address cannot drive the API, as
// it could when reaching the loopback interface was all a request needed. The
// contract's security schemes say where a request presents it, and the
// request validator checks it there before any handler runs.
type Session struct {
	token string
}

// NewSession makes a session no other run shares.
func NewSession() Session {
	return Session{token: rand.Text()}
}

// Address is where a browser opens the page served at hostPort under this
// session. The session rides in the fragment, which a browser sends to no
// server, so it reaches only the page, which keeps it and takes it out of the
// address bar.
func (s Session) Address(hostPort string) string {
	page := url.URL{Scheme: "http", Host: hostPort, Path: "/", Fragment: sessionField + "=" + s.token}

	return page.String()
}

// admits checks a security scheme the contract names for the request's
// operation: the session as a bearer token in Authorization, or, where the
// contract takes it in the query, there. Neither the zero Session nor an empty
// one presented admits anything, and the comparison takes as long whatever
// was presented.
func (s Session) admits(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	presented := presentedSession(input.RequestValidationInput.Request, input.SecurityScheme)
	if s.token == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
		return errNoSession
	}

	return nil
}

// presentedSession is the session request presents where scheme says: the
// query parameter scheme names, or the bearer token in Authorization.
func presentedSession(request *http.Request, scheme *openapi3.SecurityScheme) string {
	if scheme.In == openapi3.ParameterInQuery {
		return request.URL.Query().Get(scheme.Name)
	}

	authScheme, token, _ := strings.Cut(request.Header.Get("Authorization"), " ")
	if !strings.EqualFold(authScheme, "Bearer") {
		return ""
	}

	return token
}

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

// contentPolicy is the content security policy every answer carries: the
// app's scripts, styles, fonts and requests come from its own origin only,
// never a script written into the page or built from a string; it embeds no
// plugin, posts no form elsewhere, and is framed by no page — a page on
// another origin cannot lay the app under its own to steer a click.
const contentPolicy = "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; " +
	"object-src 'none'"

// withPolicyHeaders gives every answer — the app, the API, and a guard's
// refusal — the content policy, and the headers that say the same to a
// browser that reads only those: never framed, never sniffed for a type other
// than the one sent, and never named as the referrer of a link followed out.
func withPolicyHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentPolicy)
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, request)
	})
}

// refuseWritesInDryRun makes the whole surface read-only when the server runs
// with --dry-run: a state-changing request is answered 403 without reaching a
// handler, so no write — a checkout, a commit, a push — happens. When dry-run is
// off it adds nothing. This is the web's dry-run: the terminal interface instead
// simulates each write, but a read-only surface keeps the guarantee that matters
// — nothing is written — with one gate over every write.
//
// Trade-off TRADE-3: a web write under --dry-run is refused, not simulated.
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
