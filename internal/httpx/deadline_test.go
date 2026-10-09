// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package httpx_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
)

// clientTimeout is the timeout every client here is built with: short, so a
// test that waits past it stays quick.
const clientTimeout = 200 * time.Millisecond

// trickling answers parts parts, each a pause after the last; a pause is cut
// short once the client goes.
func trickling(t *testing.T, parts int, pause time.Duration) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		flusher, _ := writer.(http.Flusher)

		for range parts {
			_, _ = writer.Write([]byte("part\n"))

			flusher.Flush()

			select {
			case <-time.After(pause):
			case <-request.Context().Done():
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// readAll asks address under ctx through a client bounded by clientTimeout and
// reads the whole answer.
func readAll(ctx context.Context, address string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", fmt.Errorf("building the request: %w", err)
	}

	response, err := httpx.Client(clientTimeout).Do(request)
	if err != nil {
		return "", fmt.Errorf("asking: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return string(body), fmt.Errorf("reading: %w", err)
	}

	return string(body), nil
}

func TestAStreamedAnswerIsReadPastTheTimeoutWhileItKeepsComing(t *testing.T) {
	t.Parallel()

	// Arrange
	// Ten parts a fifth of the timeout apart: twice the timeout in all.
	address := trickling(t, 10, clientTimeout/5)

	// Act
	body, err := readAll(httpx.Streaming(t.Context()), address)

	// Assert
	if err != nil || strings.Count(body, "part\n") != 10 {
		t.Errorf("read %q, %v; want every part", body, err)
	}
}

func TestAStreamedAnswerThatStallsTimesOut(t *testing.T) {
	t.Parallel()

	// Arrange
	address := trickling(t, 2, 3*clientTimeout)

	// Act
	_, err := readAll(httpx.Streaming(t.Context()), address)

	// Assert
	if !errors.Is(err, httpx.ErrTimedOut) {
		t.Errorf("read = %v, want ErrTimedOut once a part was longer than the timeout in coming", err)
	}
}

func TestAnOrdinaryAnswerIsBoundedWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	address := trickling(t, 10, clientTimeout/5)

	// Act
	_, err := readAll(t.Context(), address)

	// Assert
	if !errors.Is(err, httpx.ErrTimedOut) {
		t.Errorf("read = %v, want ErrTimedOut for an answer longer than the timeout in all", err)
	}
}

func TestAnAnswerSlowToStartTimesOut(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(3 * clientTimeout):
		case <-request.Context().Done():
		}
	}))
	t.Cleanup(server.Close)

	// Act
	_, err := readAll(httpx.Streaming(t.Context()), server.URL)

	// Assert
	if !errors.Is(err, httpx.ErrTimedOut) {
		t.Errorf("read = %v, want ErrTimedOut for headers longer than the timeout in coming", err)
	}
}

func TestACallersOwnCancelIsNotATimeout(t *testing.T) {
	t.Parallel()

	// Arrange
	address := trickling(t, 10, clientTimeout/5)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Act
	_, err := readAll(ctx, address)

	// Assert
	if !errors.Is(err, context.Canceled) || errors.Is(err, httpx.ErrTimedOut) {
		t.Errorf("read = %v, want the caller's cancel, not a timeout", err)
	}
}

// errWouldNotClose is an answer's body that could not be closed.
var errWouldNotClose = errors.New("the connection would not close")

// unclosable is a body whose Close fails.
type unclosable struct {
	io.Reader
}

func (unclosable) Close() error { return errWouldNotClose }

// unclosableAnswers is a transport whose every answer's body fails to close,
// as a connection that cannot be shut down would.
type unclosableAnswers struct{}

var _ http.RoundTripper = unclosableAnswers{}

func (unclosableAnswers) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: unclosable{strings.NewReader("ok")}}, nil
}

func TestAnAnswerThatCannotBeClosedSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// The client sends through Go's default transport, which hands a scheme
	// registered on it to the transport registered for it: a scheme of this
	// run's own, since one registers only once in a process.
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport is a %T, not the *http.Transport a scheme registers on", http.DefaultTransport)
	}

	scheme := "unclosable" + strings.ToLower(rand.Text())
	transport.RegisterProtocol(scheme, unclosableAnswers{})

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, scheme+"://answer/", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := httpx.Client(clientTimeout).Do(request)
	if err != nil {
		t.Fatalf("asking: %v", err)
	}

	// Act
	err = response.Body.Close()

	// Assert
	if !errors.Is(err, errWouldNotClose) || !strings.Contains(err.Error(), "closing the answer") {
		t.Errorf("Close = %v, want why the answer could not be closed", err)
	}
}
