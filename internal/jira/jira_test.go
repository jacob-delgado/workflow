// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
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

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(myselfBody))
	})

	user, err := client.Myself(t.Context())
	if err != nil {
		t.Fatalf("Myself returned %v, want nil", err)
	}

	if user.DisplayName != "Fred F. User" {
		t.Errorf("DisplayName = %q, want %q", user.DisplayName, "Fred F. User")
	}

	if user.Name != "fred" {
		t.Errorf("Name = %q, want %q", user.Name, "fred")
	}

	if !user.Active {
		t.Error("Active = false, want true")
	}
}

func TestMyselfSendsTheCredentialOnlyInTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jira   func(string) config.Jira
		header string
	}{
		"bearer token": {
			jira:   bearerConfig,
			header: "Bearer " + token,
		},
		"basic auth": {
			jira: func(baseURL string) config.Jira {
				return config.Jira{BaseURL: baseURL, Token: token, User: "alice"}
			},
			// base64("alice:jira-token-for-tests")
			header: "Basic YWxpY2U6amlyYS10b2tlbi1mb3ItdGVzdHM=",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

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

			client := jira.New(server.Client().Do, tt.jira(server.URL))

			_, err := client.Myself(t.Context())
			if err != nil {
				t.Fatalf("Myself returned %v, want nil", err)
			}

			if got := gotHeader.Load(); got != tt.header {
				t.Errorf("Authorization = %q, want %q", got, tt.header)
			}

			// Without this, Jira answers errors in XML rather than JSON.
			if got := gotAccept.Load(); got != "application/json" {
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

			var gotPath atomic.Value

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotPath.Store(request.URL.Path)

				_, _ = writer.Write([]byte(myselfBody))
			}))
			t.Cleanup(server.Close)

			client := jira.New(server.Client().Do, bearerConfig(server.URL+tt.suffix))

			_, err := client.Myself(t.Context())
			if err != nil {
				t.Fatalf("Myself returned %v, want nil", err)
			}

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

			client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tt.status)
				// Jira answers a failed Basic login with an HTML login page, so
				// the body is never a reliable explanation. Nothing reads it.
				_, _ = writer.Write([]byte("<html>login</html>"))
			})

			_, err := client.Myself(t.Context())
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

	_, err := client.Myself(t.Context())
	if err == nil {
		t.Fatal("Myself followed a redirect, want an error")
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}

	if strings.Contains(err.Error(), token) {
		t.Errorf("the error carried the token: %v", err)
	}
}

func TestMyselfRejectsABaseURLCarryingCredentials(t *testing.T) {
	t.Parallel()

	// net/http turns userinfo into an Authorization header of its own, which
	// would compete with the configured token. Refuse it outright.
	client := jira.New(http.DefaultClient.Do, config.Jira{
		BaseURL: "https://alice:sekret@jira.example.com",
		Token:   token,
		User:    "",
	})

	_, err := client.Myself(t.Context())
	if !errors.Is(err, jira.ErrCredentialInBaseURL) {
		t.Fatalf("Myself returned %v, want ErrCredentialInBaseURL", err)
	}

	if strings.Contains(err.Error(), "sekret") {
		t.Errorf("the error quoted the password: %v", err)
	}
}

func TestMyselfRejectsABaseURLThatIsNotAbsolute(t *testing.T) {
	t.Parallel()

	client := jira.New(http.DefaultClient.Do, bearerConfig("jira.example.com/jira"))

	_, err := client.Myself(t.Context())
	if !errors.Is(err, jira.ErrInvalidBaseURL) {
		t.Errorf("Myself returned %v, want ErrInvalidBaseURL", err)
	}
}

func TestMyselfWithoutACredential(t *testing.T) {
	t.Parallel()

	client := jira.New(http.DefaultClient.Do, config.Jira{
		BaseURL: "https://jira.example.com",
		Token:   "",
		User:    "",
	})

	_, err := client.Myself(t.Context())
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Myself returned %v, want ErrNoCredential", err)
	}
}

func TestMyselfReportsAnUnreachableServer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close()

	client := jira.New(jira.HTTPClient(2*time.Second).Do, bearerConfig(baseURL))

	_, err := client.Myself(t.Context())
	if !errors.Is(err, jira.ErrUnreachable) {
		t.Errorf("Myself returned %v, want ErrUnreachable", err)
	}
}

func TestMyselfReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{not json"))
	})

	_, err := client.Myself(t.Context())
	if err == nil {
		t.Fatal("Myself accepted a malformed body, want an error")
	}
}

func TestMyselfRejectsAMalformedBaseURL(t *testing.T) {
	t.Parallel()

	// A control character is what url.Parse refuses outright, which is the only
	// way to reach the request-building failure.
	client := jira.New(http.DefaultClient.Do, config.Jira{
		BaseURL: "https://jira.example.com/\x7f",
		Token:   token,
		User:    "",
	})

	_, err := client.Myself(t.Context())
	if !errors.Is(err, jira.ErrInvalidBaseURL) {
		t.Fatalf("Myself returned %v, want ErrInvalidBaseURL", err)
	}

	// The parse error quotes the whole URL, so it is deliberately not wrapped:
	// a base_url carrying userinfo would otherwise put the password here.
	if strings.Contains(err.Error(), "jira.example.com") {
		t.Errorf("the error quoted the base URL: %v", err)
	}
}
