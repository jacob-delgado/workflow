// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Compile-time proof that an http.Client satisfies the seam Client takes.
var _ forge.Doer = http.DefaultClient.Do

// githubBody and gitlabBody are what each forge answers for GET /user. The two
// name the same thing differently, which is the only difference that matters
// here.
const (
	githubBody = `{"login":"octocat","id":1,"type":"User"}`
	gitlabBody = `{"username":"octocat","id":1,"state":"active"}`
)

// serveForge starts a forge API and returns a client pointed at it.
func serveForge(t *testing.T, handler http.HandlerFunc) forge.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret)
}

// answerJSON writes a JSON body with the right media type.
func answerJSON(body string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = writer.Write([]byte(body))
	}
}

func TestWhoamiReadsEitherForgesName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"github calls it login":    githubBody,
		"gitlab calls it username": gitlabBody,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			identity, err := serveForge(t, answerJSON(body)).Whoami(t.Context())
			if err != nil {
				t.Fatalf("Whoami returned %v, want nil", err)
			}

			if identity.Name() != "octocat" {
				t.Errorf("Name() = %q, want %q", identity.Name(), "octocat")
			}
		})
	}
}

func TestWhoamiSendsACredentialAndAUserAgent(t *testing.T) {
	t.Parallel()

	var (
		gotAuth      atomic.Value
		gotUserAgent atomic.Value
		gotURI       atomic.Value
		gotPath      atomic.Value
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth.Store(request.Header.Get("Authorization"))
		gotUserAgent.Store(request.Header.Get("User-Agent"))
		gotURI.Store(request.RequestURI)
		gotPath.Store(request.URL.Path)

		answerJSON(githubBody)(writer, request)
	}))
	t.Cleanup(server.Close)

	_, err := forge.New(server.Client().Do, server.URL, secret).Whoami(t.Context())
	if err != nil {
		t.Fatalf("Whoami returned %v, want nil", err)
	}

	if got := gotAuth.Load(); got != "Bearer "+secret {
		t.Errorf("Authorization = %q, want a bearer token", got)
	}

	// GitHub answers 403 to a request with no User-Agent, which a status map
	// would otherwise report as a refused credential.
	agent, _ := gotUserAgent.Load().(string)
	if agent == "" {
		t.Error("no User-Agent was sent; GitHub answers 403 without one")
	}

	if got := gotPath.Load(); got != "/user" {
		t.Errorf("path = %q, want /user", got)
	}

	if uri, _ := gotURI.Load().(string); strings.Contains(uri, secret) {
		t.Errorf("the request URI carried the token: %q", uri)
	}
}

func TestWhoamiTranslatesEachStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		want   error
	}{
		// A bad token is 401 on both forges.
		"unauthorized": {status: http.StatusUnauthorized, want: forge.ErrUnauthorized},
		// 403 is NOT a bad credential on GitHub — it covers rate limiting, a
		// temporary lockout that rejects valid credentials, and a missing
		// User-Agent. Saying "not accepted" here would send someone to rotate a
		// token that is fine.
		"refused":      {status: http.StatusForbidden, want: forge.ErrRefused},
		"rate limited": {status: http.StatusTooManyRequests, want: forge.ErrRefused},
		// GET /user needs no scope, so a 404 here is a wrong address rather than
		// an under-scoped token.
		"no api there":   {status: http.StatusNotFound, want: forge.ErrNoAPI},
		"server trouble": {status: http.StatusInternalServerError, want: forge.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tt.status)
			})

			_, err := client.Whoami(t.Context())
			if !errors.Is(err, tt.want) {
				t.Errorf("Whoami returned %v, want %v", err, tt.want)
			}

			if err != nil && strings.Contains(err.Error(), secret) {
				t.Errorf("the error carried the token: %v", err)
			}
		})
	}
}

func TestWhoamiRejectsAnHTMLAnswer(t *testing.T) {
	t.Parallel()

	// A base URL pointing at the WEB host rather than the API answers 200 with
	// a login page. Decoding that gives "invalid character '<'", which tells
	// nobody what went wrong.
	client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte("<!doctype html><html><body>sign in</body></html>"))
	})

	_, err := client.Whoami(t.Context())
	if !errors.Is(err, forge.ErrNotJSON) {
		t.Fatalf("Whoami returned %v, want ErrNotJSON", err)
	}

	if !strings.Contains(err.Error(), "text/html") {
		t.Errorf("the error does not name what came back instead: %v", err)
	}
}

func TestWhoamiWithoutAToken(t *testing.T) {
	t.Parallel()

	client := forge.New(http.DefaultClient.Do, "https://api.example.com", "")

	_, err := client.Whoami(t.Context())
	if !errors.Is(err, forge.ErrNoToken) {
		t.Errorf("Whoami returned %v, want ErrNoToken", err)
	}
}

func TestWhoamiReportsAnUnreachableAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := server.URL
	server.Close()

	client := forge.New(forge.HTTPClient(2*time.Second).Do, base, secret)

	_, err := client.Whoami(t.Context())
	if !errors.Is(err, forge.ErrUnreachable) {
		t.Errorf("Whoami returned %v, want ErrUnreachable", err)
	}
}

func TestWhoamiReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	_, err := serveForge(t, answerJSON("{not json")).Whoami(t.Context())
	if err == nil {
		t.Fatal("Whoami accepted a malformed body, want an error")
	}
}

func TestWhoamiReportsAMalformedBase(t *testing.T) {
	t.Parallel()

	client := forge.New(http.DefaultClient.Do, "https://api.example.com/\x7f", secret)

	_, err := client.Whoami(t.Context())
	if !errors.Is(err, forge.ErrUnreachable) {
		t.Fatalf("Whoami returned %v, want ErrUnreachable", err)
	}

	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error carried the token: %v", err)
	}
}

func TestForgeHTTPClientRefusesARedirect(t *testing.T) {
	t.Parallel()

	var secondHopSawHeader atomic.Bool

	second := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secondHopSawHeader.Store(request.Header.Get("Authorization") != "")

		answerJSON(githubBody)(writer, request)
	}))
	t.Cleanup(second.Close)

	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, second.URL+"/user", http.StatusFound)
	}))
	t.Cleanup(first.Close)

	client := forge.New(forge.HTTPClient(5*time.Second).Do, first.URL, secret)

	_, err := client.Whoami(t.Context())
	if err == nil {
		t.Fatal("Whoami followed a redirect, want an error")
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}
}
