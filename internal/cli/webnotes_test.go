// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
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

	deps := webserver.Deps{
		Jira: seams.Jira{
			Search: func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, cause },
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	notes := &sharedNotes{}
	done := make(chan error, 1)

	go func() { done <- cli.WebServerAt("127.0.0.1:0")(ctx, cfg, deps, webserver.Info{}, notes) }()

	served := servingAt(t, notes, done)
	awaitServing(t, served.base)
	askOnce(t, served, "/api/issues")
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

// askOnce sends one GET for path to the server served, presenting its session,
// and drops the answer.
func askOnce(t *testing.T, served servingAddress, path string) {
	t.Helper()

	response, err := getURLPresenting(t, served.base+path, served.authorization)
	if err != nil {
		t.Fatalf("asking %s: %v", path, err)
	}

	_ = response.Body.Close()
}

// getURL sends a GET to target over the test's context, presenting no
// session: an answer of any kind says the server is up.
func getURL(t *testing.T, target string) (*http.Response, error) {
	t.Helper()

	return getURLPresenting(t, target, "")
}

// getURLPresenting sends a GET to target over the test's context, with
// authorization as its Authorization header when it is not empty.
func getURLPresenting(t *testing.T, target, authorization string) (*http.Response, error) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("building a request for %s: %v", target, err)
	}

	if authorization != "" {
		request.Header.Set("Authorization", authorization)
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

func TestTheWebServerSaysTheAddressItBound(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	notes := &sharedNotes{}
	done := make(chan error, 1)

	// Act
	go func() {
		done <- cli.WebServerAt("127.0.0.1:0")(ctx, config.Default(), webserver.Deps{}, webserver.Info{}, notes)
	}()

	served := servingAt(t, notes, done)
	awaitServing(t, served.base)
	cancel()

	err := <-done
	// Assert
	if err != nil {
		t.Errorf("the web server stopped with %v, want a clean stop", err)
	}

	if strings.HasSuffix(served.base, ":0") {
		t.Errorf("the web server said it serves %s, want the port it bound", served.base)
	}
}

func TestTheWebServerAnswersOnlyTheSessionItsAddressCarries(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	notes := &sharedNotes{}
	done := make(chan error, 1)

	go func() {
		done <- cli.WebServerAt("127.0.0.1:0")(ctx, config.Default(), webserver.Deps{}, webserver.Info{}, notes)
	}()

	served := servingAt(t, notes, done)
	awaitServing(t, served.base)

	// Act
	presented := healthStatus(t, served.base, served.authorization)
	unpresented := healthStatus(t, served.base, "")

	// Assert
	if presented != http.StatusOK || unpresented != http.StatusUnauthorized {
		t.Errorf("the health read answered %d with the printed session and %d without, want 200 and 401",
			presented, unpresented)
	}
}

// healthStatus is the status GET /api/health at base answers, presenting
// authorization when it is not empty.
func healthStatus(t *testing.T, base, authorization string) int {
	t.Helper()

	response, err := getURLPresenting(t, base+"/api/health", authorization)
	if err != nil {
		t.Fatalf("reading the health: %v", err)
	}

	_ = response.Body.Close()

	return response.StatusCode
}

func TestTheWebServerSaysNothingOfServingOnAPortItCannotBind(t *testing.T) {
	t.Parallel()

	// Arrange
	taken, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("holding a port: %v", err)
	}

	t.Cleanup(func() { _ = taken.Close() })

	notes := &sharedNotes{}

	// Act
	err = cli.WebServerAt(taken.Addr().String())(t.Context(), config.Default(), webserver.Deps{}, webserver.Info{}, notes)

	// Assert
	if err == nil {
		t.Fatal("the web server served on a port another listener holds, want the failure")
	}

	if strings.Contains(notes.String(), "serving") {
		t.Errorf("the web server said %q on a port it could not bind, want no claim to serve", notes.String())
	}
}

// startupBound is how long a test waits for the web server it started to say
// where it serves. Building the handler parses the embedded API spec, which
// under the race detector on a loaded machine takes seconds, so the bound is
// generous: a server that stops is reported at once, through its done channel,
// rather than waited out.
const startupBound = time.Minute

// servingAddress is where the web server said it serves, and the Authorization a
// request there presents: the session its address carries, read from the
// fragment as the page reads it.
type servingAddress struct{ base, authorization string }

// servedAt reads the address the web server prints.
func servedAt(t *testing.T, address string) servingAddress {
	t.Helper()

	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("reading the address %q: %v", address, err)
	}

	token, found := strings.CutPrefix(parsed.Fragment, "session=")
	if !found || token == "" {
		t.Fatalf("the address %q carries no session", address)
	}

	return servingAddress{base: parsed.Scheme + "://" + parsed.Host, authorization: "Bearer " + token}
}

// servingAt is where the web server said it serves, once it says so. A
// server that stops before saying it fails the test with its error.
func servingAt(t *testing.T, notes *sharedNotes, done <-chan error) servingAddress {
	t.Helper()

	deadline := time.Now().Add(startupBound)

	for {
		_, rest, found := strings.Cut(notes.String(), "serving ")
		if address, _, said := strings.Cut(rest, " "); found && said {
			return servedAt(t, address)
		}

		if time.Now().After(deadline) {
			t.Fatalf("the web server never said where it serves: %q", notes.String())
		}

		select {
		case err := <-done:
			t.Fatalf("the web server stopped with %v before it said where it serves", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
