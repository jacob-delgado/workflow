// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
)

func TestClientRefusesARedirect(t *testing.T) {
	t.Parallel()

	// Arrange
	// A server that redirects to a plaintext host: the failure this client exists
	// to prevent is Go's default forwarding the Authorization header to it.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "http://elsewhere.invalid/", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	// Act
	//nolint:noctx,bodyclose // the redirect is refused before a response body exists.
	_, err := httpx.Client(time.Second).Get(server.URL)

	// Assert
	if !errors.Is(err, httpx.ErrRedirected) {
		t.Errorf("Get followed the redirect (err = %v), want it refused with ErrRedirected", err)
	}
}

func TestClientReturnsANonRedirectResponse(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	// Act
	//nolint:noctx // a test against a local server needs no deadline of its own.
	response, err := httpx.Client(time.Second).Get(server.URL)

	// Assert
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Errorf("Get = %v, %v; want the 204 the server sent and no error", response, err)
	}

	if response != nil {
		_ = response.Body.Close()
	}
}

var errUnderlying = errors.New("no such host")

func TestCauseStripsTheURLFromATransportError(t *testing.T) {
	t.Parallel()

	// A *url.Error quotes the whole request URL, which for a search is a long
	// line of encoded query and, for a webhook, the credential itself.
	wrapped := &url.Error{
		Op:  "Get",
		URL: "https://jira.example.com/rest/api/2/search?jql=secret-query",
		Err: errUnderlying,
	}

	cases := map[string]struct {
		err  error
		want error
	}{
		"a url.Error yields its inner cause":     {err: wrapped, want: errUnderlying},
		"a plain error passes through unchanged": {err: errUnderlying, want: errUnderlying},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := httpx.Cause(testCase.err)

			// Assert
			if !errors.Is(got, testCase.want) {
				t.Errorf("Cause(%v) = %v, want %v", testCase.err, got, testCase.want)
			}

			if strings.Contains(got.Error(), "secret-query") {
				t.Errorf("Cause kept the request URL: %v", got)
			}
		})
	}
}
