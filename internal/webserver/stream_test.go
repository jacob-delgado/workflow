// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// goldenFrames is the file the web client's frame test reads: what the stream
// writes for a filled workspace and an empty one, byte for byte as the browser
// receives it. It lives beside the client's test helpers, where it is read.
const goldenFrames = "../../web/src/test/snapshot-frames.sse"

// regenerateFrames is how to bring goldenFrames up to date with the stream.
const regenerateFrames = "regenerate it with: " +
	"go test ./internal/webserver -run TestStreamFrameMatchesTheClientGolden -update"

//nolint:gochecknoglobals // go test's -update flag has to be registered before the tests run
var updateGolden = flag.Bool("update", false, "rewrite the golden files from what the code writes")

// snapshots decodes every snapshot event's data from an event-stream body, in
// the order the stream pushed them.
func snapshots(t *testing.T, body string) []api.Snapshot {
	t.Helper()

	var pushed []api.Snapshot

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

// lastSnapshot is the most recent snapshot an event-stream body carries.
func lastSnapshot(t *testing.T, body string) api.Snapshot {
	t.Helper()

	pushed := snapshots(t, body)

	return pushed[len(pushed)-1]
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
	cfg.Jira.Views = []config.JiraView{{Name: testBugView, JQL: testBugJQL}}

	// Act
	_ = streamOnce(t, serve(t, deps, cfg), "/api/events?view="+testBugView)

	// Assert
	if want := jira.ScopedToMe(testBugJQL); gotJQL != want {
		t.Errorf("searched %q, want the named view's JQL scoped to you, %q", gotJQL, want)
	}
}

func TestViewsDTOScopesEachViewToYou(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jql, want string
	}{
		"a view of everyone's issues": {jql: testBugJQL, want: "(" + testBugJQL + ") AND assignee = currentUser()"},
		"a view that names the assignee": {
			jql: "assignee is EMPTY ORDER BY created", want: "assignee is EMPTY ORDER BY created",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var searched string

			deps := filledDeps()
			deps.Search = func(jql string, _ int) (jira.SearchResult, error) {
				searched = jql

				return jira.SearchResult{}, nil
			}
			cfg := config.Default()
			cfg.Jira.Views = []config.JiraView{{Name: testBugView, JQL: tt.jql}}

			// Act
			streamOnce(t, serve(t, deps, cfg), "/api/events?view="+testBugView)

			// Assert
			if searched != tt.want {
				t.Errorf("searched %q, want %q", searched, tt.want)
			}
		})
	}
}

func TestStreamRefusesAnUnknownViewBeforeUpgrading(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, filledDeps(), config.Default()), "/api/events?view=nope")

	// Assert
	// A problem, not an event stream: the refusal comes before the upgrade, so
	// the browser's EventSource sees a 404 rather than a stream of the wrong view.
	if ct := recorder.Header().Get("Content-Type"); ct != problemJSON {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}

	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.ProblemCodeNotFound {
		t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
	}
}

func TestStreamEmptiesTheIssuesWhenItsViewIsRemoved(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The view is removed by a configuration save while the stream is
		// open; the next snapshot empties the list rather than showing
		// another view's issues.
		cfg := config.Default()
		cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
		cfg.Jira.Views = []config.JiraView{{Name: testBugView, JQL: testBugJQL}}
		info := webserver.Info{Version: testVersion, StreamInterval: streamTick}
		handler := serveWith(t, filledDeps(), cfg, info)
		stream := openStream(t, handler, "/api/events?view="+testBugView)

		synctest.Wait()

		// Act
		saved := putConfig(t, handler, marshal(t, config.Default()))

		advance(streamTick)

		// Assert
		if saved.Code != http.StatusOK {
			t.Fatalf("saving the config: status = %d, want 200", saved.Code)
		}

		if last := lastSnapshot(t, stream.close()); last.Issues.Total != 0 {
			t.Errorf("last snapshot's issues = %+v, want an empty page once the view is gone", last.Issues)
		}
	})
}

func TestStreamSnapshotIsEmptyWithoutSeams(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, webserver.Deps{}, config.Default()), "/api/events")

	// Assert
	// Nothing configured is nothing to ask, so no panel says it failed.
	snap := firstSnapshot(t, recorder.Body.String())
	if snap.Issues.Total != 0 || snap.Branch.Name != "" || len(snap.Changes.Changes) != 0 || snap.Review.Found ||
		snap.Problems != nil {
		t.Errorf("snapshot = %+v, want empty panels and no problem when nothing is configured", snap)
	}
}

func TestStreamSnapshotDegradesWhenSeamsFail(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every read fails; the stream must still push a snapshot, with each failing
	// panel empty rather than the whole snapshot lost, and saying why beside it.
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

	wantEveryPanelProblem(t, snap.Problems)
}

// wantEveryPanelProblem fails unless each panel whose read can fail carries a
// problem, the curated one a seam's unclassified error gets.
func wantEveryPanelProblem(t *testing.T, problems *api.PanelProblems) {
	t.Helper()

	if problems == nil {
		t.Fatal("problems = nil, want one for each panel whose read failed")
	}

	for panel, prob := range map[string]*api.Problem{
		"issues": problems.Issues, "branch": problems.Branch, "changes": problems.Changes, "review": problems.Review,
	} {
		if prob == nil || prob.Code != api.ProblemCodeInternal || prob.Detail == "" {
			t.Errorf("%s problem = %+v, want the curated internal problem", panel, prob)
		}
	}
}

func TestStreamRepushesOnTheInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		info := webserver.Info{Version: testVersion, StreamInterval: streamTick}
		stream := openStream(t, serveWith(t, filledDeps(), config.Default(), info), "/api/events")

		// Act
		advance(streamTick)

		// Assert
		if pushes := len(snapshots(t, stream.close())); pushes != 2 {
			t.Errorf("got %d snapshots over one interval, want 2: on connect, and once it had passed", pushes)
		}
	})
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

// streamTick is the interval a stream on a fake clock pushes at: in a bubble
// its length costs nothing, and it is far from defaultStreamInterval, so a
// stream that ignored the interval it was given would push a different count.
const streamTick = time.Minute

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

	ctx, cancel := context.WithCancel(t.Context())
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
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

// advance moves a bubble's clock on by d, then waits until everything that
// came due has run and is blocked again. Inside a synctest bubble the sleep
// takes no time; it is how the bubble's clock is moved.
func advance(d time.Duration) {
	time.Sleep(d)
	synctest.Wait()
}

func TestStreamFrameMatchesTheClientGolden(t *testing.T) {
	t.Parallel()

	// Arrange
	// A workspace with every part of the snapshot filled — issues, the branch
	// and its commit, a change, the pull request and its CI, the issues in
	// flight, one only the remote has, a learned scope, the started task and
	// the linked ones — and one with nothing wired at all.
	filled := filledDeps()
	filled.Branches = func() ([]string, error) { return []string{testBranchName, targetBranch}, nil }
	filled.RemoteBranches = func() ([]string, error) { return []string{testBranchName, "feat/PROJ-7-flag"}, nil }
	filled.LastScope = func() (string, bool) { return "api", true }
	filled.Tasks = fakeTaskwarrior().seams()

	cfg := config.Default()
	cfg.Messaging.Channel = testChannel

	// Act
	frames := streamOnce(t, serve(t, filled, cfg), "/api/events").Body.String() +
		streamOnce(t, serve(t, webserver.Deps{}, config.Default()), "/api/events").Body.String()

	// Assert
	path := filepath.FromSlash(goldenFrames)
	if *updateGolden {
		err := os.WriteFile(path, []byte(frames), 0o644)
		if err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v; %s", path, err, regenerateFrames)
	}

	if string(want) != frames {
		t.Errorf("the stream's frames differ from %s, which the web client's test reads; %s\n got: %s\nwant: %s",
			path, regenerateFrames, frames, want)
	}
}
