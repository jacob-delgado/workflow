// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// token is the credential every test sends. It is distinctive so an assertion
// that it did NOT appear somewhere means something.
const token = "jira-token-for-tests"

// myselfBody is the shape Jira Data Center documents for GET /rest/api/2/myself.
const myselfBody = `{"self":"https://jira.example.com/rest/api/2/user?username=fred",` +
	`"key":"JIRAUSER10100","name":"fred","displayName":"Fred F. User","active":true}`

// inProgress is the status most fixtures put an issue in, and indeterminate the
// category Jira files it under.
const (
	inProgress    = "In Progress"
	indeterminate = "indeterminate"
)

// servedUser is who the fixtures say Jira served, and basicUser who a Basic
// credential names.
const (
	servedUser = "fred"
	basicUser  = "alice"
)

// jsonMediaType is what Jira's answers are, and what its requests say they want.
const jsonMediaType = "application/json"

// exampleBaseURL is an instance no test ever reaches: tests that use it fail
// the transport or refuse before sending.
const exampleBaseURL = "https://jira.example.com"

// bearerConfig authenticates with a Data Center personal access token.
func bearerConfig(baseURL string) config.Jira {
	return config.Jira{BaseURL: baseURL, Token: token, User: ""}
}

// serve starts a Jira that answers handler, and returns a client pointed at it.
func serve(t *testing.T, handler http.HandlerFunc) jira.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return jira.New(server.Client().Do, bearerConfig(server.URL))
}

func TestMyselfReportsTheAuthenticatedUser(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", jsonMediaType)
		_, _ = writer.Write([]byte(myselfBody))
	})

	// Act
	user, err := client.Myself(t.Context())
	if err != nil {
		t.Fatalf("Myself returned %v, want nil", err)
	}

	// Assert
	if user.DisplayName != "Fred F. User" {
		t.Errorf("DisplayName = %q, want %q", user.DisplayName, "Fred F. User")
	}

	if user.Name != servedUser {
		t.Errorf("Name = %q, want %q", user.Name, servedUser)
	}

	if !user.Active {
		t.Error("Active = false, want true")
	}
}

func TestMyselfSendsTheCredentialOnlyInTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		user   string
		header string
	}{
		"bearer token": {user: "", header: "Bearer " + token},
		// base64("alice:jira-token-for-tests")
		"basic auth": {user: basicUser, header: "Basic YWxpY2U6amlyYS10b2tlbi1mb3ItdGVzdHM="},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var (
				gotHeader atomic.Value
				gotURI    atomic.Value
				gotAccept atomic.Value
			)

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotHeader.Store(request.Header.Get("Authorization"))
				gotURI.Store(request.RequestURI)
				gotAccept.Store(request.Header.Get("Accept"))

				_, _ = writer.Write([]byte(myselfBody))
			}))
			t.Cleanup(server.Close)

			client := jira.New(server.Client().Do, config.Jira{BaseURL: server.URL, Token: token, User: tt.user})

			// Act
			_, err := client.Myself(t.Context())
			if err != nil {
				t.Fatalf("Myself returned %v, want nil", err)
			}

			// Assert
			if got := gotHeader.Load(); got != tt.header {
				t.Errorf("Authorization = %q, want %q", got, tt.header)
			}

			// Without this, Jira answers errors in XML rather than JSON.
			if got := gotAccept.Load(); got != jsonMediaType {
				t.Errorf("Accept = %q, want application/json", got)
			}

			// The credential belongs in the header and nowhere else. A token in
			// the path or query reaches proxy logs and browser history.
			if uri, _ := gotURI.Load().(string); strings.Contains(uri, token) {
				t.Errorf("the request URI carried the token: %q", uri)
			}
		})
	}
}

func TestMyselfKeepsTheContextPath(t *testing.T) {
	t.Parallel()

	// Jira Data Center is commonly deployed under a context path. url.URL's
	// ResolveReference silently discards it, which is why this is pinned.
	cases := map[string]struct {
		suffix string
		want   string
	}{
		"no context path":                 {suffix: "", want: "/rest/api/2/myself"},
		"no context path, trailing slash": {suffix: "/", want: "/rest/api/2/myself"},
		"context path":                    {suffix: "/jira", want: "/jira/rest/api/2/myself"},
		"context path, trailing slash":    {suffix: "/jira/", want: "/jira/rest/api/2/myself"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var gotPath atomic.Value

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotPath.Store(request.URL.Path)

				_, _ = writer.Write([]byte(myselfBody))
			}))
			t.Cleanup(server.Close)

			client := jira.New(server.Client().Do, bearerConfig(server.URL+tt.suffix))

			// Act
			_, err := client.Myself(t.Context())
			if err != nil {
				t.Fatalf("Myself returned %v, want nil", err)
			}

			// Assert
			if got := gotPath.Load(); got != tt.want {
				t.Errorf("request path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMyselfTranslatesEachStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		want   error
	}{
		"unauthorized":   {status: http.StatusUnauthorized, want: jira.ErrUnauthorized},
		"forbidden":      {status: http.StatusForbidden, want: jira.ErrForbidden},
		"no api there":   {status: http.StatusNotFound, want: jira.ErrNoAPI},
		"server trouble": {status: http.StatusInternalServerError, want: jira.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tt.status)
				// Jira answers a failed Basic login with an HTML login page, so
				// the body is never a reliable explanation. Nothing reads it.
				_, _ = writer.Write([]byte("<html>login</html>"))
			})

			// Act
			_, err := client.Myself(t.Context())

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Myself returned %v, want %v", err, tt.want)
			}

			if err != nil && strings.Contains(err.Error(), token) {
				t.Errorf("the error carried the token: %v", err)
			}
		})
	}
}

func TestMyselfRefusesARedirect(t *testing.T) {
	t.Parallel()

	// Arrange
	var secondHopSawHeader atomic.Bool

	// The second hop shares 127.0.0.1 with the first and differs only by port.
	// net/http's default client forwards Authorization on exactly that shape.
	second := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secondHopSawHeader.Store(request.Header.Get("Authorization") != "")

		_, _ = writer.Write([]byte(myselfBody))
	}))
	t.Cleanup(second.Close)

	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, second.URL+"/rest/api/2/myself", http.StatusFound)
	}))
	t.Cleanup(first.Close)

	client := jira.New(jira.HTTPClient(5*time.Second).Do, bearerConfig(first.URL))

	// Act
	_, err := client.Myself(t.Context())

	// Assert
	if !errors.Is(err, jira.ErrRedirected) {
		t.Fatalf("Myself returned %v, want ErrRedirected", err)
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}

	if strings.Contains(err.Error(), token) {
		t.Errorf("the error carried the token: %v", err)
	}
}

func TestMyselfRefusesABaseURLItCannotUse(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jira config.Jira
		want error
		// hidden is what the error must never quote.
		hidden string
	}{
		// net/http turns userinfo into an Authorization header of its own, which
		// would compete with the configured token. Refuse it outright.
		"one carrying credentials": {
			jira:   config.Jira{BaseURL: "https://alice:sekret@jira.example.com", Token: token, User: ""},
			want:   jira.ErrCredentialInBaseURL,
			hidden: "sekret",
		},
		"one that is not absolute": {
			jira:   bearerConfig("jira.example.com/jira"),
			want:   jira.ErrInvalidBaseURL,
			hidden: token,
		},
		// A control character is what url.Parse refuses outright, which is the
		// only way to reach the request-building failure. The parse error quotes
		// the whole URL, so it is deliberately not wrapped: a base_url carrying
		// userinfo would otherwise put the password in the error.
		"one url.Parse refuses": {
			jira:   bearerConfig("https://jira.example.com/\x7f"),
			want:   jira.ErrInvalidBaseURL,
			hidden: "jira.example.com",
		},
		"one with no credential to send": {
			jira:   config.Jira{BaseURL: exampleBaseURL, Token: "", User: basicUser},
			want:   jira.ErrNoCredential,
			hidden: basicUser,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := jira.New(http.DefaultClient.Do, tt.jira)

			// Act
			_, err := client.Myself(t.Context())

			// Assert
			if !errors.Is(err, tt.want) || strings.Contains(err.Error(), tt.hidden) {
				t.Errorf("Myself returned %v, want %v without quoting %q", err, tt.want, tt.hidden)
			}
		})
	}
}

func TestMyselfReportsAnUnreachableServer(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close()

	client := jira.New(jira.HTTPClient(2*time.Second).Do, bearerConfig(baseURL))

	// Act
	_, err := client.Myself(t.Context())

	// Assert
	if !errors.Is(err, jira.ErrUnreachable) {
		t.Errorf("Myself returned %v, want ErrUnreachable", err)
	}
}

func TestMyselfReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{not json"))
	})

	// Act
	_, err := client.Myself(t.Context())

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("Myself returned %v, want the malformed body's syntax error", err)
	}
}

// Compile-time proof that an http.Client satisfies the seam Client takes, so the
// production wiring cannot drift from what these tests exercise. CLAUDE.md asks
// for this on every type meant to satisfy an interface; internal/gitrepo has the
// equivalent for its Runner and this file was missing it.
var _ jira.Doer = http.DefaultClient.Do
