// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

// The forge-issue adapter maps a forge.Client — a concrete type built from a
// Doer, not a seam a black-box test can fake through the wiring — into the
// tracker the Issues pane reads. These drive the mappers with a client pointed
// at a fake forge, which is what the Doer is for.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// errConnectFailed is a forge that cannot be reached, for the failure paths.
var errConnectFailed = errors.New("cannot reach the forge")

// forgeIssueConnect returns a connect that hands back a client pointed at a fake
// GitHub answering the issue endpoints the adapter calls.
func forgeIssueConnect(t *testing.T) func() (forgeConnection, error) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if request.URL.Path == "/search/issues" {
			_, _ = writer.Write([]byte(`{"items":[{"number":42,` +
				`"html_url":"https://github.com/ex/repo/issues/42","title":"the bug"}]}`))

			return
		}

		_, _ = writer.Write([]byte(`{"number":42,"html_url":"https://github.com/ex/repo/issues/42",` +
			`"title":"the bug","body":"it broke","user":{"login":"ana"}}`))
	}))
	t.Cleanup(server.Close)

	connection := forgeConnection{
		client: forge.New(server.Client().Do, server.URL, "token"),
		repo:   forge.Repo{Kind: forge.KindGitHub, Host: exampleHost, Path: exampleRepo},
	}

	return func() (forgeConnection, error) { return connection, nil }
}

func TestForgeIssuesListAsSearchRows(t *testing.T) {
	t.Parallel()

	// Arrange
	connect := forgeIssueConnect(t)

	// Act
	result, err := listForgeIssues(context.Background(), connect)

	// Assert
	if err != nil || result.Total != 1 || len(result.Issues) != 1 {
		t.Fatalf("listForgeIssues = %+v, %v, want one row", result, err)
	}

	if row := result.Issues[0]; row.Key != "42" || row.Summary != "the bug" ||
		row.StatusCategory != forgeOpenCategory {
		t.Errorf("issue row = %+v, want key 42, its title and the open category", row)
	}
}

func TestForgeIssueReadAsDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	connect := forgeIssueConnect(t)

	// Act
	detail, err := readForgeIssue(context.Background(), connect, "42")

	// Assert
	if err != nil || detail.Issue.Key != "42" || detail.Description != "it broke" || detail.Reporter != "ana" {
		t.Errorf("readForgeIssue = %+v, %v, want the body and author", detail, err)
	}
}

func TestForgeIssueClose(t *testing.T) {
	t.Parallel()

	// Arrange
	connect := forgeIssueConnect(t)

	// Act
	err := closeForgeIssue(context.Background(), connect, "42")
	// Assert
	if err != nil {
		t.Errorf("closeForgeIssue: %v", err)
	}
}

func TestForgeIssueRejectsANonNumericKey(t *testing.T) {
	t.Parallel()

	// Arrange
	connect := forgeIssueConnect(t)

	// Act
	err := closeForgeIssue(context.Background(), connect, "PROJ-1")

	// Assert
	if !errors.Is(err, errNotAnIssueNumber) {
		t.Errorf("closeForgeIssue returned %v, want errNotAnIssueNumber", err)
	}
}

func TestForgeIssuesReportAConnectFailure(t *testing.T) {
	t.Parallel()

	failing := func() (forgeConnection, error) { return forgeConnection{}, errConnectFailed }

	cases := map[string]func() error{
		"list": func() error {
			_, err := listForgeIssues(context.Background(), failing)

			return err
		},
		"read": func() error {
			_, err := readForgeIssue(context.Background(), failing, "42")

			return err
		},
		"close": func() error { return closeForgeIssue(context.Background(), failing, "42") },
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := act()

			// Assert
			if !errors.Is(err, errConnectFailed) {
				t.Errorf("%s returned %v, want the connect failure", name, err)
			}
		})
	}
}

func TestForgeIssuesReportAForgeError(t *testing.T) {
	t.Parallel()

	// Arrange
	broken := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(broken.Close)

	connect := func() (forgeConnection, error) {
		return forgeConnection{
			client: forge.New(broken.Client().Do, broken.URL, "token"),
			repo:   forge.Repo{Kind: forge.KindGitHub, Host: exampleHost, Path: exampleRepo},
		}, nil
	}

	// Act
	_, listErr := listForgeIssues(context.Background(), connect)
	_, readErr := readForgeIssue(context.Background(), connect, "42")

	// Assert
	if listErr == nil || readErr == nil {
		t.Errorf("a failing forge was not reported: list=%v, read=%v", listErr, readErr)
	}
}

func TestForgeIssueTransitionsOfferClose(t *testing.T) {
	t.Parallel()

	// Act
	transitions := forgeIssueTransitions()

	// Assert
	if len(transitions) != 1 || transitions[0].ToStatusCategory != forgeDoneCategory {
		t.Errorf("forgeIssueTransitions = %+v, want a single Close to the done category", transitions)
	}
}

func TestTrackerDepsPicksTheBackend(t *testing.T) {
	t.Parallel()

	cases := map[string]config.Config{
		"Jira when it is configured": {Jira: config.Jira{BaseURL: "https://jira.example.com"}},
		"the forge otherwise":        {},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			deps := trackerDeps(context.Background(), cfg, Workspace{}, RequestTimeout, nil)

			// Assert
			if deps.Search == nil {
				t.Errorf("trackerDeps for %s returned a tracker with no search", name)
			}
		})
	}
}
