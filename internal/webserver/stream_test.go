// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
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

// lastSnapshot is the most recent snapshot an event-stream body carries.
func lastSnapshot(t *testing.T, body string) api.Snapshot {
	t.Helper()

	pushed := webtest.Snapshots(t, body)

	return pushed[len(pushed)-1]
}

func TestStreamPushesASnapshotOnConnect(t *testing.T) {
	t.Parallel()

	// Act
	recorder := webtest.StreamOnce(t, webtest.Serve(t, webtest.FilledDeps(), config.Default()), "/api/events")

	// Assert
	if ct := recorder.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	snap := webtest.FirstSnapshot(t, recorder.Body.String())
	if snap.Issues.Total != 1 || snap.Branch.Name != webtest.TestBranchName || !snap.Review.Found {
		t.Errorf("snapshot = %+v, want the filled read state", snap)
	}

	if len(snap.Changes.Changes) != 1 || snap.Messaging.Author != webtest.TestAuthor {
		t.Errorf("snapshot = %+v, want the changes and messaging author", snap)
	}
}

func TestStreamUsesTheNamedView(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotJQL string

	deps := webtest.FilledDeps()
	deps.Jira.Search = func(_ context.Context, jql string, _ int) (jira.SearchResult, error) {
		gotJQL = jql

		return jira.SearchResult{}, nil
	}

	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: webtest.TestBugView, JQL: webtest.TestBugJQL}}

	// Act
	_ = webtest.StreamOnce(t, webtest.Serve(t, deps, cfg), "/api/events?view="+webtest.TestBugView)

	// Assert
	if want := jira.ScopedToMe(webtest.TestBugJQL); gotJQL != want {
		t.Errorf("searched %q, want the named view's JQL scoped to you, %q", gotJQL, want)
	}
}

func TestViewsDTOScopesEachViewToYou(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jql, want string
	}{
		"a view of everyone's issues": {
			jql:  webtest.TestBugJQL,
			want: "(" + webtest.TestBugJQL + ") AND assignee = currentUser()",
		},
		"a view that names the assignee": {
			jql: "assignee is EMPTY ORDER BY created", want: "assignee is EMPTY ORDER BY created",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var searched string

			deps := webtest.FilledDeps()
			deps.Jira.Search = func(_ context.Context, jql string, _ int) (jira.SearchResult, error) {
				searched = jql

				return jira.SearchResult{}, nil
			}
			cfg := config.Default()
			cfg.Jira.Views = []config.JiraView{{Name: webtest.TestBugView, JQL: tt.jql}}

			// Act
			webtest.StreamOnce(t, webtest.Serve(t, deps, cfg), "/api/events?view="+webtest.TestBugView)

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
	recorder := webtest.StreamOnce(t, webtest.Serve(t, webtest.FilledDeps(), config.Default()), "/api/events?view=nope")

	// Assert
	// A problem, not an event stream: the refusal comes before the upgrade, so
	// the browser's EventSource sees a 404 rather than a stream of the wrong view.
	if ct := recorder.Header().Get("Content-Type"); ct != problemJSON {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}

	failure := webtest.Decode[api.Problem](t, recorder)
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
		cfg.Jira.Views = []config.JiraView{{Name: webtest.TestBugView, JQL: webtest.TestBugJQL}}
		info := webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick}
		handler := webtest.ServeWith(t, webtest.FilledDeps(), cfg, info)
		stream := webtest.OpenStream(t, handler, "/api/events?view="+webtest.TestBugView)

		synctest.Wait()

		// Act
		saved := webtest.PutConfig(t, handler, webtest.Marshal(t, config.Default()))

		advance(streamTick)

		// Assert
		if saved.Code != http.StatusOK {
			t.Fatalf("saving the config: status = %d, want 200", saved.Code)
		}

		if last := lastSnapshot(t, stream.Close()); last.Issues.Total != 0 {
			t.Errorf("last snapshot's issues = %+v, want an empty page once the view is gone", last.Issues)
		}
	})
}

func TestStreamSnapshotIsEmptyWithoutSeams(t *testing.T) {
	t.Parallel()

	// Act
	recorder := webtest.StreamOnce(t, webtest.Serve(t, webserver.Deps{}, config.Default()), "/api/events")

	// Assert
	// Nothing configured is nothing to ask, so no panel says it failed.
	snap := webtest.FirstSnapshot(t, recorder.Body.String())
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
	deps := webtest.FilledDeps()
	deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
		return jira.SearchResult{}, webtest.ErrSeam
	}
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, webtest.ErrSeam }
	deps.Git.Changes = func(context.Context) ([]gitrepo.Change, error) { return nil, webtest.ErrSeam }
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, webtest.ErrSeam
	}

	// Act
	recorder := webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()), "/api/events")

	// Assert
	snap := webtest.FirstSnapshot(t, recorder.Body.String())
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
		info := webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick}
		stream := webtest.OpenStream(t, webtest.ServeWith(t, webtest.FilledDeps(), config.Default(), info), "/api/events")

		// Act
		advance(streamTick)

		// Assert
		if pushes := len(webtest.Snapshots(t, stream.Close())); pushes != 2 {
			t.Errorf("got %d snapshots over one interval, want 2: on connect, and once it had passed", pushes)
		}
	})
}

func TestStreamRequiresAFlushableWriter(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := webtest.Serve(t, webtest.FilledDeps(), config.Default())
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/events", nil)
	request.Host = webtest.LoopbackHost
	writer := &webtest.UnflushableWriter{}

	// Act
	handler.ServeHTTP(writer, request)

	// Assert
	if writer.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the writer cannot stream", writer.Code)
	}
}

func TestStreamStopsWhenTheConnectionFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The context never cancels, so only a failed write can end the stream; a
	// writer that fails every write proves the handler stops on a broken pipe.
	handler := webtest.Serve(t, webtest.FilledDeps(), config.Default())
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/events", nil)
	request.Host = webtest.LoopbackHost
	done := make(chan struct{})

	// Act
	go func() {
		handler.ServeHTTP(&webtest.FailingFlushWriter{}, request)
		close(done)
	}()

	// Assert
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stream did not stop after a failed write")
	}
}

// streamTick is the interval a stream on a fake clock pushes at: in a bubble
// its length costs nothing, and it is far from defaultStreamInterval, so a
// stream that ignored the interval it was given would push a different count.
const streamTick = time.Minute

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
	// A workspace with every part of the snapshot filled, its view's issues
	// kept from last time — issues, the branch
	// and its commit, a change, the pull request and its CI, the issues in
	// flight, one only the remote has, a learned scope, the started task and
	// the linked ones — and one with nothing wired at all.
	filled := webtest.FilledDeps()
	filled.Git.Branches = func(context.Context) ([]string, error) {
		return []string{webtest.TestBranchName, targetBranch},
			nil
	}
	filled.Git.RemoteBranches = func(context.Context) ([]string, error) {
		return []string{webtest.TestBranchName, "feat/PROJ-7-flag"}, nil
	}
	filled.Store.LastScope = func(context.Context) (string, bool) { return "api", true }
	filled.Store.CachedIssues = func(context.Context, string) ([]jira.Issue, bool) {
		return []jira.Issue{keptIssue()}, true
	}
	filled.Tasks = webtest.FakeTaskwarrior().Seams()

	cfg := config.Default()
	cfg.Messaging.Channel = webtest.TestChannel

	// Act
	frames := webtest.StreamOnce(t, webtest.Serve(t, filled, cfg), "/api/events").Body.String() +
		webtest.StreamOnce(t, webtest.Serve(t, webserver.Deps{}, config.Default()), "/api/events").Body.String()

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

// uncategorized is a status Jira files under No Category, whose key is
// noCategory: one no one sorted into a category.
const (
	uncategorized                     = "Triage"
	noCategory    jira.StatusCategory = "undefined"
)

func TestAFrameCarriesAnIssueWhoseStatusIsUnderNoCategory(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
		return jira.SearchResult{
			Issues: []jira.Issue{{
				Key: webtest.TestKey, Summary: webtest.TestSummary, Status: uncategorized,
				StatusCategory: noCategory,
			}},
			Total: 1,
		}, nil
	}

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if len(frame.Issues.Issues) != 1 || frame.Issues.Issues[0].StatusCategory != api.StatusCategoryNew {
		t.Errorf("frame issues = %+v, want the issue carried as not yet started", frame.Issues.Issues)
	}
}
