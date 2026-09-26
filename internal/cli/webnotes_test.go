// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// shownToken is a Jira token the configuration holds and a failure quotes.
const shownToken = "jira-secret-1111"

var (
	// errIndexCorrupt is a search failure no class of failure explains.
	errIndexCorrupt = errors.New("the search index is corrupt")
	// errQuotesTheToken is a search failure that quotes the configured token.
	errQuotesTheToken = errors.New("the tracker answered " + shownToken + " oddly")
	// errSpansLines is a failure whose cause runs over lines, as git's output does.
	errSpansLines = errors.New("error: bad signature 0x00000000\nfatal: index file corrupt")
)

func TestTheWebServerWritesAFailureNoClassExplainsToItsNotes(t *testing.T) {
	t.Parallel()

	// Act
	notes := notesAfterAFailedSearch(t, config.Default(), errIndexCorrupt)

	// Assert
	if strings.Count(notes, "workflow web: "+errIndexCorrupt.Error()+"\n") != 1 {
		t.Errorf("the web server said %q, want the failure's cause on a line of its own", notes)
	}
}

func TestTheWebServerWritesAMultiLineCauseOnOneLine(t *testing.T) {
	t.Parallel()

	// Act
	notes := notesAfterAFailedSearch(t, config.Default(), errSpansLines)

	// Assert
	if !strings.Contains(notes, "workflow web: error: bad signature 0x00000000; fatal: index file corrupt\n") {
		t.Errorf("the web server said %q, want the cause's lines joined on one line", notes)
	}

	for line := range strings.Lines(notes) {
		if !strings.HasPrefix(line, "workflow web: ") {
			t.Errorf("the web server wrote the line %q, which does not start with workflow web:", line)
		}
	}
}

func TestTheWebServerMasksACredentialInItsNotes(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Jira.Token = shownToken

	// Act
	notes := notesAfterAFailedSearch(t, cfg, errQuotesTheToken)

	// Assert
	if strings.Contains(notes, shownToken) {
		t.Errorf("the web server said %q, which shows the Jira token", notes)
	}

	if !strings.Contains(notes, "workflow web: the tracker answered ****1111 oddly\n") {
		t.Errorf("the web server said %q, want the failure with the token masked", notes)
	}
}

// notesAfterAFailedSearch serves the web API over a search that fails with
// cause, asks it for the issues once, stops it, and returns what it said.
func notesAfterAFailedSearch(t *testing.T, cfg config.Config, cause error) string {
	t.Helper()

	addr := freeLoopbackAddr(t)
	deps := webserver.Deps{
		Search: func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, cause },
	}

	ctx, cancel := context.WithCancel(t.Context())
	notes := &sharedNotes{}
	done := make(chan error, 1)

	go func() { done <- cli.WebServerAt(addr)(ctx, cfg, deps, webserver.Info{}, notes) }()

	awaitServing(t, "http://"+addr)
	askOnce(t, "http://"+addr+"/api/issues")
	cancel()

	err := <-done
	if err != nil {
		t.Fatalf("the web server stopped with %v, want a clean stop", err)
	}

	return notes.String()
}

// freeLoopbackAddr is a loopback address with a port no one was listening on
// a moment ago, so the test knows where the server it starts will be.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}

	addr := listener.Addr().String()

	err = listener.Close()
	if err != nil {
		t.Fatalf("freeing the port: %v", err)
	}

	return addr
}

// awaitServing waits until the server at base answers its health read.
func awaitServing(t *testing.T, base string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for {
		response, err := getURL(t, base+"/api/health")
		if err == nil {
			_ = response.Body.Close()

			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("the web server at %s never answered: %v", base, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// askOnce sends one GET to target and drops the answer.
func askOnce(t *testing.T, target string) {
	t.Helper()

	response, err := getURL(t, target)
	if err != nil {
		t.Fatalf("asking %s: %v", target, err)
	}

	_ = response.Body.Close()
}

// getURL sends a GET to target over the test's context.
func getURL(t *testing.T, target string) (*http.Response, error) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("building a request for %s: %v", target, err)
	}

	return http.DefaultClient.Do(request) //nolint:wrapcheck // the caller reports it whole
}

// sharedNotes is a notes writer the server's request goroutines and the test
// share.
type sharedNotes struct {
	mu   sync.Mutex
	text strings.Builder
}

func (n *sharedNotes) Write(written []byte) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.text.Write(written) //nolint:wrapcheck // a strings.Builder never fails a write
}

func (n *sharedNotes) String() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.text.String()
}
