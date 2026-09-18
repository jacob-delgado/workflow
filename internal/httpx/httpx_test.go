// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
