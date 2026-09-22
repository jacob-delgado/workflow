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
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// firstSnapshot decodes the first snapshot event's data from an event-stream body.
func firstSnapshot(t *testing.T, body string) api.Snapshot {
	t.Helper()

	for line := range strings.SplitSeq(body, "\n") {
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}

		var snap api.Snapshot

		err := json.Unmarshal([]byte(payload), &snap)
		if err != nil {
			t.Fatalf("decoding the snapshot from %q: %v", payload, err)
		}

		return snap
	}

	t.Fatalf("no snapshot event in the stream body: %q", body)

	return api.Snapshot{}
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

func TestStreamPushesASnapshotOnConnect(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, filledDeps(), config.Default()), "/api/events")

	// Assert
	if ct := recorder.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	snap := firstSnapshot(t, recorder.Body.String())
	if snap.Issues.Total != 1 || snap.Branch.Name != testBranchName || !snap.Review.Found {
		t.Errorf("snapshot = %+v, want the filled read state", snap)
	}

	if len(snap.Changes.Changes) != 1 || snap.Messaging.Author != testAuthor {
		t.Errorf("snapshot = %+v, want the changes and messaging author", snap)
	}
}

func TestStreamUsesTheNamedView(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotJQL string

	deps := filledDeps()
	deps.Search = func(jql string, _ int) (jira.SearchResult, error) {
		gotJQL = jql

		return jira.SearchResult{}, nil
	}

	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: "Bugs", JQL: testBugJQL}}

	// Act
	_ = streamOnce(t, serve(t, deps, cfg), "/api/events?view=Bugs")

	// Assert
	if gotJQL != testBugJQL {
		t.Errorf("searched %q, want the named view's JQL", gotJQL)
	}
}

func TestStreamSnapshotIsEmptyWithoutSeams(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, webserver.Deps{}, config.Default()), "/api/events")

	// Assert
	snap := firstSnapshot(t, recorder.Body.String())
	if snap.Issues.Total != 0 || snap.Branch.Name != "" || len(snap.Changes.Changes) != 0 || snap.Review.Found {
		t.Errorf("snapshot = %+v, want empty panels when nothing is configured", snap)
	}
}

func TestStreamSnapshotDegradesWhenSeamsFail(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every read fails; the stream must still push a snapshot, with each failing
	// panel empty rather than the whole snapshot lost.
	deps := filledDeps()
	deps.Search = func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, errSeam }
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errSeam }

	// Act
	recorder := streamOnce(t, serve(t, deps, config.Default()), "/api/events")

	// Assert
	snap := firstSnapshot(t, recorder.Body.String())
	if snap.Issues.Total != 0 || snap.Branch.Name != "" || len(snap.Changes.Changes) != 0 || snap.Review.Found {
		t.Errorf("snapshot = %+v, want empty panels when the seams fail", snap)
	}
}

func TestStreamRepushesOnTheInterval(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	info := webserver.Info{Version: testVersion, StreamInterval: 2 * time.Millisecond}
	handler := serveWith(t, filledDeps(), config.Default(), info)
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	request.Host = loopbackHost
	recorder := httptest.NewRecorder()
	done := make(chan struct{})

	// Act
	go func() {
		handler.ServeHTTP(recorder, request)
		close(done)
	}()

	time.Sleep(40 * time.Millisecond)
	cancel()
	<-done

	// Assert
	if pushes := strings.Count(recorder.Body.String(), "event: snapshot"); pushes < 2 {
		t.Errorf("got %d snapshots, want at least 2 (connect, then a re-push)", pushes)
	}
}

func TestStreamRequiresAFlushableWriter(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/events", nil)
	request.Host = loopbackHost
	writer := &unflushableWriter{}

	// Act
	handler.ServeHTTP(writer, request)

	// Assert
	if writer.code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the writer cannot stream", writer.code)
	}
}

func TestStreamStopsWhenTheConnectionFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The context never cancels, so only a failed write can end the stream; a
	// writer that fails every write proves the handler stops on a broken pipe.
	handler := serve(t, filledDeps(), config.Default())
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/events", nil)
	request.Host = loopbackHost
	done := make(chan struct{})

	// Act
	go func() {
		handler.ServeHTTP(&failingFlushWriter{}, request)
		close(done)
	}()

	// Assert
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stream did not stop after a failed write")
	}
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
