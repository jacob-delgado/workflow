// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestServeStopsWhenTheContextIsCanceled(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	handler := webtest.Serve(t, webserver.Deps{}, config.Default())
	done := make(chan error, 1)

	listener := listen(t)

	go func() { done <- webserver.Serve(ctx, listener, handler) }()

	// Act
	cancel()

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil on a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop when the context was canceled")
	}
}

// inFlightFor is how long a request in flight waits to learn whether Serve has
// returned while it was still being answered. Serve cannot return before it
// ends unless it skips the drain, so the passing path never waits on a race.
const inFlightFor = 250 * time.Millisecond

func TestServeReturnsOnlyOnceARequestInFlightIsAnswered(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	entered, returned := make(chan struct{}), make(chan struct{})
	servedWhileReturned := make(chan bool, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)

		select {
		case <-returned:
			servedWhileReturned <- true
		case <-time.After(inFlightFor):
			servedWhileReturned <- false
		}

		w.WriteHeader(http.StatusNoContent)
	})
	listener := listen(t)
	done := make(chan error, 1)

	go func() {
		done <- webserver.Serve(ctx, listener, handler)

		close(returned)
	}()

	go askOnce(t, "http://"+listener.Addr().String()+"/")

	<-entered

	// Act
	cancel()

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil on a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop when the context was canceled")
	}

	if <-servedWhileReturned {
		t.Error("Serve returned while a request was still being answered, want it to drain first")
	}
}

// An open event stream never goes idle, so a request in flight that went on
// past the server's stop would hold the drain for the whole grace.
func TestServeEndsTheContextOfARequestInFlightWhenItStops(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	entered, ended := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(entered)
		<-request.Context().Done()
		close(ended)
	})
	listener := listen(t)

	go func() { _ = webserver.Serve(ctx, listener, handler) }()

	go askOnce(t, "http://"+listener.Addr().String()+"/")

	<-entered

	// Act
	cancel()

	// Assert
	select {
	case <-ended:
	case <-time.After(webtest.EndsWithin):
		t.Fatal("the request in flight's context did not end when the server stopped")
	}
}

// askOnce sends one GET to url and reads the answer whole, whatever it is: the
// server's drain, not the answer, is what the caller looks at.
func askOnce(t *testing.T, url string) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Errorf("building the request: %v", err)

		return
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}

	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}

func TestServeReportsAListenerThatCannotAccept(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, webserver.Deps{}, config.Default())
	listener := listen(t)
	_ = listener.Close()

	// Act
	err := webserver.Serve(context.Background(), listener, handler)

	// Assert
	if err == nil {
		t.Error("Serve returned nil, want the closed listener's failure")
	}
}

// listen is a loopback listener on a port the system chooses, closed with the
// test.
func listen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on loopback: %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	return listener
}
