// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
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

			// Arrange
			client := serveForge(t, answerJSON(body))

			// Act
			identity, err := client.Whoami(t.Context())

			// Assert
			if err != nil || identity.Name() != "octocat" {
				t.Errorf("Whoami = %q, %v; want octocat", identity.Name(), err)
			}
		})
	}
}

func TestWhoamiSendsACredentialAndAUserAgent(t *testing.T) {
	t.Parallel()

	// Arrange
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

	client := forge.New(server.Client().Do, server.URL, secret)

	// Act
	_, err := client.Whoami(t.Context())
	if err != nil {
		t.Fatalf("Whoami returned %v, want nil", err)
	}

	// Assert
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
		// 403 is NOT a bad credential: the forge took the token and will not let
		// it do this. Saying "not accepted" here would send someone to rotate a
		// token that is fine.
		"refused":      {status: http.StatusForbidden, want: forge.ErrRefused},
		"rate limited": {status: http.StatusTooManyRequests, want: httpx.ErrRateLimited},
		// GET /user needs no scope, so a 404 here is a wrong address rather than
		// an under-scoped token.
		"no api there":   {status: http.StatusNotFound, want: forge.ErrNoAPI},
		"server trouble": {status: http.StatusInternalServerError, want: forge.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tt.status)
			})

			// Act
			_, err := client.Whoami(t.Context())

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Whoami returned %v, want %v", err, tt.want)
			}

			if err != nil && strings.Contains(err.Error(), secret) {
				t.Errorf("the error carried the token: %v", err)
			}
		})
	}
}

func TestAForbiddenAnswerThatAsksToWaitIsARateLimit(t *testing.T) {
	t.Parallel()

	// GitHub answers a spent rate limit with a 403 like any refusal, and tells
	// the two apart only in its headers: no requests left, or a wait named.
	read := func(ctx context.Context, client forge.Client) error {
		_, err := client.Whoami(ctx)

		return err
	}
	write := func(ctx context.Context, client forge.Client) error {
		return client.Merge(ctx, githubRepo(), forge.PullRequest{Number: 42}, forge.MergeCommit)
	}

	const (
		remaining = "X-Ratelimit-Remaining"
		wait      = "Retry-After"
	)

	cases := map[string]struct {
		header, value string
		ask           func(context.Context, forge.Client) error
		want          error
	}{
		"a read with no requests left":  {header: remaining, value: "0", ask: read, want: httpx.ErrRateLimited},
		"a write with no requests left": {header: remaining, value: "0", ask: write, want: httpx.ErrRateLimited},
		"a read told to wait":           {header: wait, value: "60", ask: read, want: httpx.ErrRateLimited},
		"a write told to wait":          {header: wait, value: "60", ask: write, want: httpx.ErrRateLimited},
		"requests to spare":             {header: remaining, value: "4999", ask: read, want: forge.ErrRefused},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set(tt.header, tt.value)
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusForbidden)
				_, _ = writer.Write([]byte(`{"message":"API rate limit exceeded for user ID 1."}`))
			})

			// Act
			err := tt.ask(t.Context(), client)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("the forge's 403 returned %v, want %v", err, tt.want)
			}

			if errors.Is(err, httpx.ErrRateLimited) == errors.Is(err, forge.ErrRefused) {
				t.Errorf("the forge's 403 returned %v, want a rate limit or a refusal, not both or neither", err)
			}
		})
	}
}

func TestWhoamiRejectsAnHTMLAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// A base URL pointing at the WEB host rather than the API answers 200 with
	// a login page. Decoding that gives "invalid character '<'", which tells
	// nobody what went wrong.
	client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte("<!doctype html><html><body>sign in</body></html>"))
	})

	// Act
	_, err := client.Whoami(t.Context())

	// Assert
	if !errors.Is(err, forge.ErrNotJSON) || !strings.Contains(err.Error(), "text/html") {
		t.Errorf("Whoami returned %v, want ErrNotJSON naming what came back instead", err)
	}
}

func TestWhoamiRefusesWhatItCannotSend(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		base  string
		token forge.Token
		want  error
	}{
		"no token": {base: "https://api.example.com", token: "", want: forge.ErrNoToken},
		"a base url.Parse refuses": {
			base: "https://api.example.com/\x7f", token: secret, want: forge.ErrUnreachable,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forge.New(http.DefaultClient.Do, tt.base, tt.token)

			// Act
			_, err := client.Whoami(t.Context())

			// Assert
			if !errors.Is(err, tt.want) || strings.Contains(err.Error(), secret) {
				t.Errorf("Whoami returned %v, want %v without the token", err, tt.want)
			}
		})
	}
}

func TestWhoamiReportsAnUnreachableAPI(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := server.URL
	server.Close()

	client := forge.New(forge.HTTPClient(2*time.Second).Do, base, secret)

	// Act
	_, err := client.Whoami(t.Context())

	// Assert
	if !errors.Is(err, forge.ErrUnreachable) {
		t.Errorf("Whoami returned %v, want ErrUnreachable", err)
	}
}

func TestWhoamiReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serveForge(t, answerJSON("{not json"))

	// Act
	_, err := client.Whoami(t.Context())

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("Whoami returned %v, want the malformed body's syntax error", err)
	}
}

func TestForgeHTTPClientRefusesARedirect(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	_, err := client.Whoami(t.Context())

	// Assert
	if !errors.Is(err, httpx.ErrRedirected) {
		t.Errorf("Whoami returned %v, want ErrRedirected", err)
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}
}
