// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// A token a command prints can expire while workflow runs, as the documented
// az account get-access-token's does within the hour. Jira refusing it runs
// the command once more and asks again, rather than refusing every call until
// a restart.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// countingTokenCommand is a stand-in token command that prints token-N on its
// Nth run, so each run gives a token no earlier one did.
func countingTokenCommand(t *testing.T) tokenCommand {
	t.Helper()

	return writeTokenCommand(t, `printf 'token-%s\n' "$(grep -c x "$(dirname "$0")/runs")"`+"\n")
}

// expiringJira is a stand-in Jira that refuses, as 401, any token but the one
// it accepts, and keeps the Authorization header of every request in order.
type expiringJira struct {
	url  string
	lock *sync.Mutex
	sent *[]string
}

// newExpiringJira serves a Jira that accepts only accepted, after hold has
// had its say on each request.
func newExpiringJira(t *testing.T, accepted string, hold func()) expiringJira {
	t.Helper()

	stand := expiringJira{url: "", lock: &sync.Mutex{}, sent: &[]string{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		signed := request.Header.Get("Authorization")

		stand.lock.Lock()
		*stand.sent = append(*stand.sent, signed)
		stand.lock.Unlock()

		hold()

		writer.Header().Set("Content-Type", "application/json")

		if signed != "Bearer "+accepted {
			writer.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = writer.Write([]byte(`{"total":0,"issues":[]}`))
	}))
	t.Cleanup(server.Close)

	stand.url = server.URL

	return stand
}

// requests is the Authorization header of every request Jira was sent.
func (j expiringJira) requests() []string {
	j.lock.Lock()
	defer j.lock.Unlock()

	return append([]string(nil), *j.sent...)
}

// jiraOver is the tracker wired to Jira at address with settings' token
// source.
func jiraOver(t *testing.T, address string, settings config.Jira) func() error {
	t.Helper()

	cfg := config.Default()
	cfg.Jira = settings
	cfg.Jira.BaseURL = address
	tracker := wired(t, cfg, onOrigin(t.TempDir(), ""), nil).Jira

	return func() error {
		_, err := tracker.Search(t.Context(), "assignee = currentUser()", 0)

		return err
	}
}

func TestAJiraTokenCommandsTokenRefusedIsFetchedAgainAndTheRequestRetried(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := countingTokenCommand(t)
	stand := newExpiringJira(t, "token-2", func() {})
	search := jiraOver(t, stand.url, config.Jira{TokenCommand: tokens.command})

	// Act
	err := search()
	// Assert
	if err != nil {
		t.Errorf("Search = %v, want the retry with the command's new token answered", err)
	}

	if runs, sent := tokens.runs(), stand.requests(); runs != 2 ||
		len(sent) != 2 || sent[1] != "Bearer token-2" {
		t.Errorf("the token command ran %d times and Jira was sent %q; want twice, the retry with token-2", runs, sent)
	}
}

func TestAJiraTokenCommandsNewTokenRefusedTooIsNotFetchedAThirdTime(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := countingTokenCommand(t)
	stand := newExpiringJira(t, "never-printed", func() {})
	search := jiraOver(t, stand.url, config.Jira{TokenCommand: tokens.command})

	// Act
	err := search()

	// Assert
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Search = %v, want jira.ErrUnauthorized", err)
	}

	if runs, sent := tokens.runs(), len(stand.requests()); runs != 2 || sent != 2 {
		t.Errorf("the token command ran %d times and Jira was asked %d times; want twice each", runs, sent)
	}
}

func TestAJiraTokenCommandThatFailsWhenRunAgainIsReportedAsNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := writeTokenCommand(t, `[ "$(grep -c x "$(dirname "$0")/runs")" = 1 ] || exit 1`+"\nprintf 'token-1\\n'\n")
	stand := newExpiringJira(t, "never-printed", func() {})
	search := jiraOver(t, stand.url, config.Jira{TokenCommand: tokens.command})

	// Act
	err := search()

	// Assert
	if !errors.Is(err, jira.ErrNoCredential) || tokens.runs() != 2 {
		t.Errorf("Search = %v after %d runs of the token command, want jira.ErrNoCredential after two",
			err, tokens.runs())
	}
}

func TestAJiraTokenFromTheEnvironmentRefusedIsNotRetried(t *testing.T) {
	t.Parallel()

	// Arrange
	stand := newExpiringJira(t, "never-sent", func() {})
	search := jiraOver(t, stand.url, config.Jira{TokenEnv: "PATH"})

	// Act
	err := search()

	// Assert
	if !errors.Is(err, jira.ErrUnauthorized) || len(stand.requests()) != 1 {
		t.Errorf("Search = %v after %d requests, want jira.ErrUnauthorized after one", err, len(stand.requests()))
	}
}

func TestJiraRefusingTwoCallsAtOnceRunsTheTokenCommandOnceMore(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := countingTokenCommand(t)
	bothSent := sync.WaitGroup{}
	bothSent.Add(2)

	var held atomic.Int32

	stand := newExpiringJira(t, "token-2", func() {
		// The first two requests, both with token-1, are refused only once
		// both have arrived, so each asks for a new token after the other has
		// read the old one.
		if held.Add(1) <= 2 {
			bothSent.Done()
			bothSent.Wait()
		}
	})
	search := jiraOver(t, stand.url, config.Jira{TokenCommand: tokens.command})

	// Act
	errs := make(chan error, 2)

	for range 2 {
		go func() { errs <- search() }()
	}

	first, second := <-errs, <-errs

	// Assert
	if first != nil || second != nil {
		t.Errorf("Search = %v and %v, want both retried and answered", first, second)
	}

	if runs := tokens.runs(); runs != 2 {
		t.Errorf("two calls refused at once ran the token command %d times, want twice: once, then once more", runs)
	}
}
