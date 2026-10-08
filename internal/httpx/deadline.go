// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrTimedOut reports a server that took longer to answer than the client
// waits.
var ErrTimedOut = errors.New("the server took too long to answer")

// streaming is the context key a request carries whose answer may take longer
// than the timeout in all.
type streaming struct{}

// Streaming is ctx for a request whose answer may take longer than the
// client's timeout to arrive in full, so long as no part of it is longer than
// that in coming — a long log, read to its end. An answer is otherwise
// bounded whole, from the request to the last byte of its body.
func Streaming(ctx context.Context) context.Context {
	return context.WithValue(ctx, streaming{}, true)
}

// streams reports a request that asked to be bounded part by part.
func streams(ctx context.Context) bool {
	streamed, _ := ctx.Value(streaming{}).(bool)

	return streamed
}

// deadlines is a transport that bounds each request by timeout: the wait for
// its answer, and then its body whole, or for a Streaming request each wait
// for the next part of it. http.Client's own Timeout cannot do the second: it
// bounds every request whole, body and all.
type deadlines struct {
	next    http.RoundTripper
	timeout time.Duration
}

var _ http.RoundTripper = deadlines{}

// RoundTrip sends request under the deadline, and hands back an answer whose
// body keeps to it.
func (d deadlines) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(request.Context())
	timedOut := fmt.Errorf("%w (waited %s)", ErrTimedOut, d.timeout)
	timer := time.AfterFunc(d.timeout, func() { cancel(timedOut) })

	response, err := d.next.RoundTrip(request.WithContext(ctx))
	if err != nil {
		timer.Stop()
		cancel(nil)

		return nil, why(ctx, err)
	}

	body := &boundedBody{
		body: response.Body, cause: func() error { return context.Cause(ctx) }, timer: timer, cancel: cancel, idle: 0,
	}
	if streams(request.Context()) {
		body.idle = d.timeout
	}

	response.Body = body

	return response, nil
}

// why is the timeout that cut a request short, or err when it was not one.
func why(ctx context.Context, err error) error {
	return timeoutOr(context.Cause(ctx), err)
}

// timeoutOr is cause when it is a timeout, and err otherwise.
func timeoutOr(cause, err error) error {
	if errors.Is(cause, ErrTimedOut) {
		return cause
	}

	return err
}

// boundedBody is an answer's body read under its request's deadline: the one
// running since the request was sent, or, given an idle wait, one set afresh
// after every read.
type boundedBody struct {
	body   io.ReadCloser
	cause  func() error
	timer  *time.Timer
	cancel context.CancelCauseFunc
	idle   time.Duration
}

var _ io.ReadCloser = (*boundedBody)(nil)

// Read reads the body, and names the timeout when that is why it failed. The
// end of the body is io.EOF itself, which a reader's caller compares rather
// than unwraps.
func (b *boundedBody) Read(buffer []byte) (int, error) {
	read, err := b.body.Read(buffer)
	if b.idle > 0 {
		b.timer.Reset(b.idle)
	}

	switch {
	case err == nil:
		return read, nil
	case errors.Is(err, io.EOF):
		return read, io.EOF
	default:
		return read, timeoutOr(b.cause(), err)
	}
}

// Close stops the deadline and closes the body.
func (b *boundedBody) Close() error {
	b.timer.Stop()
	b.cancel(nil)

	err := b.body.Close()
	if err != nil {
		return fmt.Errorf("closing the answer: %w", err)
	}

	return nil
}
