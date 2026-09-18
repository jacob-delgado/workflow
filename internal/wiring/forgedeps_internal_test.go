// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

// forgeDeps builds a forge.Client from a Doer — a concrete type, not a seam a
// black-box test can fake through the wiring — so these drive the interface's
// forge seams with a connect that hands back a client pointed at a fake GitHub.
// Without this, every seam's success arm (the branch taken once connect works)
// went unexercised: only connection FAILURES were ever tested.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// exampleRepo and exampleHost describe the fake repository the forge tests point
// a client at.
const (
	exampleRepo = "ex/repo"
	exampleHost = "github.com"
)

// fakeGitHub answers the endpoints the forge seams call, enough for each to
// return a real value rather than an error.
func fakeGitHub(t *testing.T) func() (forgeConnection, error) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch {
		case request.URL.Path == "/user":
			_, _ = writer.Write([]byte(`{"login":"me"}`))
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/pulls"):
			_, _ = writer.Write([]byte(`{"number":43,"html_url":"https://github.com/ex/repo/pull/43","title":"fix: token"}`))
		case strings.HasSuffix(request.URL.Path, "/pulls"):
			_, _ = writer.Write([]byte(`[]`))
		case strings.HasSuffix(request.URL.Path, "/status"):
			_, _ = writer.Write([]byte(`{"total_count":0,"statuses":[]}`))
		case strings.HasSuffix(request.URL.Path, "/check-runs"):
			_, _ = writer.Write([]byte(`{"total_count":0,"check_runs":[]}`))
		case request.URL.Path == "/search/issues":
			_, _ = writer.Write([]byte(`{"items":[]}`))
		default:
			_, _ = writer.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)

	connection := forgeConnection{
		client: forge.New(server.Client().Do, server.URL, "token"),
		repo:   forge.Repo{Kind: forge.KindGitHub, Host: exampleHost, Path: exampleRepo},
	}

	return func() (forgeConnection, error) { return connection, nil }
}

func TestForgeDepsReachTheForgeOnceConnected(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := forgeDepsFrom(context.Background(), fakeGitHub(t), func() []forge.Template { return nil }, forge.KindGitHub)

	// Act
	_, found, findErr := deps.FindPullRequest("feat/x")
	created, createErr := deps.CreatePullRequest(forge.NewPullRequest{Title: "fix: token"})
	_, statusErr := deps.CheckStatus(forge.PullRequest{Number: 43}, "abc123")
	reviews, reviewErr := deps.ReviewRequests()
	author, authorErr := deps.Author()

	// Assert
	// Every seam gets past its connect guard and returns the forge's answer,
	// which the connection-failure tests never reach.
	if findErr != nil || found {
		t.Errorf("FindPullRequest = %v, %v; want no pull found and no error", found, findErr)
	}

	if createErr != nil || created.Number != 43 {
		t.Errorf("CreatePullRequest = %+v, %v; want the created pull #43", created, createErr)
	}

	if statusErr != nil {
		t.Errorf("CheckStatus returned %v, want the forge's answer", statusErr)
	}

	if reviewErr != nil || len(reviews) != 0 {
		t.Errorf("ReviewRequests = %+v, %v; want an empty list and no error", reviews, reviewErr)
	}

	if authorErr != nil || author != "me" {
		t.Errorf("Author = %q, %v; want the login the forge reported", author, authorErr)
	}
}
