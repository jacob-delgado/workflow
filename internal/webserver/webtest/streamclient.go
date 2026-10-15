// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

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

// SnapshotMessage and SeedMessage name the stream's two kinds of message.
const (
	SnapshotMessage = "snapshot"
	SeedMessage     = "seed"
)

// Message is one event-stream message: its id, its event and its data.
type Message struct {
	ID    string
	Event string
	Data  string
}

// Messages splits an event-stream body into its messages, in the order sent:
// each is separated from the next by a blank line.
func Messages(body string) []Message {
	var sent []Message

	for block := range strings.SplitSeq(body, "\n\n") {
		var read Message

		for line := range strings.SplitSeq(block, "\n") {
			if id, ok := strings.CutPrefix(line, "id: "); ok {
				read.ID = id
			}

			if event, ok := strings.CutPrefix(line, "event: "); ok {
				read.Event = event
			}

			if data, ok := strings.CutPrefix(line, "data: "); ok {
				read.Data = data
			}
		}

		if read.Event != "" {
			sent = append(sent, read)
		}
	}

	return sent
}

// Snapshots decodes every snapshot event's data from an event-stream body, in
// the order the stream pushed them.
func Snapshots(t *testing.T, body string) []api.Snapshot {
	t.Helper()

	var pushed []api.Snapshot

	for _, sent := range Messages(body) {
		if sent.Event != SnapshotMessage {
			continue
		}

		var snap api.Snapshot

		err := json.Unmarshal([]byte(sent.Data), &snap)
		if err != nil {
			t.Fatalf("decoding the snapshot from %q: %v", sent.Data, err)
		}

		pushed = append(pushed, snap)
	}

	if len(pushed) == 0 {
		t.Fatalf("no snapshot event in the stream body: %q", body)
	}

	return pushed
}

// FirstSnapshot is the first snapshot an event-stream body carries.
func FirstSnapshot(t *testing.T, body string) api.Snapshot {
	t.Helper()

	return Snapshots(t, body)[0]
}

// StreamOnce runs the events handler with a context that is canceled at once, so
// it pushes a single snapshot and returns. It returns the recorder.
func StreamOnce(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	request.Host = LoopbackHost
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// UnflushableWriter is an http.ResponseWriter that does not support flushing, so
// the events handler must refuse it.
type UnflushableWriter struct {
	header http.Header
	Code   int
}

// Header is the response's header, made on first use.
func (u *UnflushableWriter) Header() http.Header {
	if u.header == nil {
		u.header = http.Header{}
	}

	return u.header
}

// Write accepts b and discards it.
func (u *UnflushableWriter) Write(b []byte) (int, error) { return len(b), nil }

// WriteHeader keeps the status code in Code.
func (u *UnflushableWriter) WriteHeader(code int) { u.Code = code }

// FailingFlushWriter can flush but fails every write, standing in for a client
// that has hung up mid-stream.
type FailingFlushWriter struct {
	header http.Header
}

// Header is the response's header, made on first use.
func (f *FailingFlushWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}

	return f.header
}

// Write fails with ErrSeam, as a write to a client that hung up does.
func (f *FailingFlushWriter) Write([]byte) (int, error) { return 0, ErrSeam }

// WriteHeader does nothing.
func (f *FailingFlushWriter) WriteHeader(int) {}

// Flush does nothing; that it exists is what lets the stream start.
func (f *FailingFlushWriter) Flush() {}

// OpenedStream is the events handler serving a page's connection on a
// goroutine of its own, as the HTTP server runs it.
type OpenedStream struct {
	Cancel   context.CancelFunc
	Done     chan struct{}
	recorder *httptest.ResponseRecorder
}

// OpenStream connects to target on handler, and disconnects when the test
// ends if Close has not already.
func OpenStream(t *testing.T, handler http.Handler, target string) *OpenedStream {
	t.Helper()

	return OpenStreamOf(t, handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
}

// OpenStreamOf is OpenStream for request, disconnected by ending a context
// derived from its own: one derived from the server's lifetime, as
// webserver.Serve derives each, ends when the server stops too.
func OpenStreamOf(t *testing.T, handler http.Handler, request *http.Request) *OpenedStream {
	t.Helper()

	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	request.Host = LoopbackHost
	stream := &OpenedStream{Cancel: cancel, Done: make(chan struct{}), recorder: httptest.NewRecorder()}

	go func() {
		defer close(stream.Done)

		handler.ServeHTTP(stream.recorder, request)
	}()

	t.Cleanup(func() { stream.Close() })

	return stream
}

// Close disconnects, waits for the handler to return, and answers what the
// stream wrote.
func (s *OpenedStream) Close() string {
	s.Cancel()
	<-s.Done

	return s.recorder.Body.String()
}

// StreamedFrames is how many snapshots the stream tests let a stream push:
// enough for a read made once to show against one made on every frame.
const StreamedFrames = 3

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

var _ http.Flusher = (*cancelingFlusher)(nil)

// Flush flushes the recorder, runs between unless it completes the last frame
// wanted, and cancels the request once it does.
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

// StreamPaced streams StreamedFrames snapshots over cfg, on a clock that starts
// at PaceStart and that step moves on between two frames, as that much time
// passing would.
func StreamPaced(t *testing.T, deps webserver.Deps, cfg config.Config, step time.Duration) string {
	t.Helper()

	now := PaceStart()
	deps.Clock = func() time.Time { return now }

	info := webserver.Info{Version: TestVersion, StreamInterval: time.Millisecond}
	handler := ServeWith(t, deps, cfg, info)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	request.Host = LoopbackHost
	writer := &cancelingFlusher{
		ResponseRecorder: httptest.NewRecorder(), cancel: cancel, frames: StreamedFrames,
		between: func() { now = now.Add(step) },
	}
	handler.ServeHTTP(writer, request)

	return writer.Body.String()
}

// PaceStart is the moment a paced stream's clock starts at.
func PaceStart() time.Time {
	return time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC)
}
