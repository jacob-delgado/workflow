// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

// A request can fail before anything the forge says is read: the address
// cannot carry it, nothing answers, or the answer does not say it is JSON.
// Each is reported as that, rather than as something the forge decided.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

func TestAWriteThatNeverReachesTheForgeSaysSo(t *testing.T) {
	t.Parallel()

	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	goneBase := gone.URL
	gone.Close()

	cases := map[string]string{
		"an address url.Parse refuses": unusableBase,
		"a forge that is not there":    goneBase,
	}

	for name, base := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forge.New(httpx.Client(2*time.Second).Do, base, secret)

			// Act
			err := client.Merge(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, forge.MergeCommit)

			// Assert
			if !errors.Is(err, forge.ErrUnreachable) || strings.Contains(err.Error(), secret) {
				t.Errorf("Merge returned %v, want ErrUnreachable without the token", err)
			}
		})
	}
}

func TestAnAnswerThatNamesNoMediaTypeIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no Content-Type at all":         nil,
		"a Content-Type that is garbled": {"application/json; charset"},
	}

	for name, contentType := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A nil header value stops the server from sniffing one in.
			client := serveForge(t, func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header()["Content-Type"] = contentType
				_, _ = writer.Write([]byte(githubBody))
			})

			// Act
			_, err := client.Whoami(t.Context())

			// Assert
			if !errors.Is(err, forge.ErrNotJSON) {
				t.Errorf("Whoami returned %v, want ErrNotJSON for an answer that does not say it is JSON", err)
			}
		})
	}
}
