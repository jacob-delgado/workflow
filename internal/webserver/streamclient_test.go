// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// snapshotMessage and seedMessage name the stream's two kinds of message.
const (
	snapshotMessage = "snapshot"
	seedMessage     = "seed"
)

// message is one event-stream message: its id, its event and its data.
type message struct {
	id    string
	event string
	data  string
}

// messages splits an event-stream body into its messages, in the order sent:
// each is separated from the next by a blank line.
func messages(body string) []message {
	var sent []message

	for block := range strings.SplitSeq(body, "\n\n") {
		var read message

		for line := range strings.SplitSeq(block, "\n") {
			if id, ok := strings.CutPrefix(line, "id: "); ok {
				read.id = id
			}

			if event, ok := strings.CutPrefix(line, "event: "); ok {
				read.event = event
			}

			if data, ok := strings.CutPrefix(line, "data: "); ok {
				read.data = data
			}
		}

		if read.event != "" {
			sent = append(sent, read)
		}
	}

	return sent
}

// snapshots decodes every snapshot event's data from an event-stream body, in
// the order the stream pushed them.
func snapshots(t *testing.T, body string) []api.Snapshot {
	t.Helper()

	var pushed []api.Snapshot

	for _, sent := range messages(body) {
		if sent.event != snapshotMessage {
			continue
		}

		var snap api.Snapshot

		err := json.Unmarshal([]byte(sent.data), &snap)
		if err != nil {
			t.Fatalf("decoding the snapshot from %q: %v", sent.data, err)
		}

		pushed = append(pushed, snap)
	}

	if len(pushed) == 0 {
		t.Fatalf("no snapshot event in the stream body: %q", body)
	}

	return pushed
}

// firstSnapshot is the first snapshot an event-stream body carries.
func firstSnapshot(t *testing.T, body string) api.Snapshot {
	t.Helper()

	return snapshots(t, body)[0]
}

// streamOnce runs the events handler with a context that is canceled at once, so
// it pushes a single snapshot and returns. It returns the recorder.
func streamOnce(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	request.Host = loopbackHost
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// unflushableWriter is an http.ResponseWriter that does not support flushing, so
// the events handler must refuse it.
type unflushableWriter struct {
	header http.Header
	code   int
}

func (u *unflushableWriter) Header() http.Header {
	if u.header == nil {
		u.header = http.Header{}
	}

	return u.header
}

func (u *unflushableWriter) Write(b []byte) (int, error) { return len(b), nil }
func (u *unflushableWriter) WriteHeader(code int)        { u.code = code }

// failingFlushWriter can flush but fails every write, standing in for a client
// that has hung up mid-stream.
type failingFlushWriter struct {
	header http.Header
}

func (f *failingFlushWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}

	return f.header
}

func (f *failingFlushWriter) Write([]byte) (int, error) { return 0, errSeam }
func (f *failingFlushWriter) WriteHeader(int)           {}
func (f *failingFlushWriter) Flush()                    {}

// openedStream is the events handler serving a page's connection on a
// goroutine of its own, as the HTTP server runs it.
type openedStream struct {
	cancel   context.CancelFunc
	done     chan struct{}
	recorder *httptest.ResponseRecorder
}

// openStream connects to target on handler, and disconnects when the test
// ends if close has not already.
func openStream(t *testing.T, handler http.Handler, target string) *openedStream {
	t.Helper()

	return openStreamOf(t, handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
}

// openStreamOf is openStream for request, disconnected by ending a context
// derived from its own: one derived from the server's lifetime, as Serve
// derives each, ends when the server stops too.
func openStreamOf(t *testing.T, handler http.Handler, request *http.Request) *openedStream {
	t.Helper()

	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	request.Host = loopbackHost
	stream := &openedStream{cancel: cancel, done: make(chan struct{}), recorder: httptest.NewRecorder()}

	go func() {
		defer close(stream.done)

		handler.ServeHTTP(stream.recorder, request)
	}()

	t.Cleanup(func() { stream.close() })

	return stream
}

// close disconnects, waits for the handler to return, and answers what the
// stream wrote.
func (s *openedStream) close() string {
	s.cancel()
	<-s.done

	return s.recorder.Body.String()
}

// streamedFrames is how many snapshots the stream tests let a stream push:
// enough for a read made once to show against one made on every frame.
const streamedFrames = 3

// cancelingFlusher records the stream and cancels its request from inside the
// flush that completes the last frame wanted, so a test sees a known number of
// pushes without timing them. It embeds the recorder for everything else a
// ResponseWriter does.
type cancelingFlusher struct {
	*httptest.ResponseRecorder

	cancel  context.CancelFunc
	frames  int
	flushed int

	// between, when set, runs after each flush but the last wanted, as time
	// passing between two frames would; a frame that slips out after the
	// cancel lands at the last frame's time.
	between func()
}

func (c *cancelingFlusher) Flush() {
	c.ResponseRecorder.Flush()

	c.flushed++
	if c.flushed < c.frames && c.between != nil {
		c.between()
	}

	if c.flushed == c.frames {
		c.cancel()
	}
}

// streamPaced is streamFrames over cfg, on a clock that starts at paceStart and
// that step moves on between two frames, as that much time passing would.
func streamPaced(t *testing.T, deps webserver.Deps, cfg config.Config, step time.Duration) string {
	t.Helper()

	now := paceStart()
	deps.Clock = func() time.Time { return now }

	info := webserver.Info{Version: testVersion, StreamInterval: time.Millisecond}
	handler := serveWith(t, deps, cfg, info)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	request.Host = loopbackHost
	writer := &cancelingFlusher{
		ResponseRecorder: httptest.NewRecorder(), cancel: cancel, frames: streamedFrames,
		between: func() { now = now.Add(step) },
	}
	handler.ServeHTTP(writer, request)

	return writer.Body.String()
}

// paceStart is the moment a paced stream's clock starts at.
func paceStart() time.Time {
	return time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC)
}
