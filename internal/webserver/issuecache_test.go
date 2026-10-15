// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// keptList is one list the store was asked to keep: the view's JQL, the keys
// of its issues, and how the context it was kept under stood.
type keptList struct {
	jql    string
	keys   []jira.Key
	ctxErr error
}

// keptLists records each list the store is asked to keep, behind a lock since
// the streams keep on their own goroutines.
type keptLists struct {
	mu   sync.Mutex
	kept []keptList
	err  error
}

// keep is the store's CacheIssues, recording what it was asked and failing
// with the record's err.
func (k *keptLists) keep(ctx context.Context, jql string, issues []jira.Issue) error {
	keys := make([]jira.Key, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key)
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	k.kept = append(k.kept, keptList{jql: jql, keys: keys, ctxErr: ctx.Err()})

	return k.err
}

// all is every list kept, in order.
func (k *keptLists) all() []keptList {
	k.mu.Lock()
	defer k.mu.Unlock()

	return slices.Clone(k.kept)
}

// bugsView is a configuration offering the Bugs view.
func bugsView() config.Config {
	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: webtest.TestBugView, JQL: webtest.TestBugJQL}}

	return cfg
}

func TestASearchKeepsTheViewsFirstPageForTheNextSession(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cfg   config.Config
		query string
		want  string
	}{
		"the issues assigned to you": {cfg: config.Default(), want: jira.AssignedToMe},
		// The terminal keys the cache by the scoped query too, so the two
		// surfaces share what is kept.
		"a named view": {cfg: bugsView(), query: "?view=" + webtest.TestBugView, want: jira.ScopedToMe(webtest.TestBugJQL)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept, deps := &keptLists{}, webtest.FilledDeps()
			deps.Store.CacheIssues = kept.keep

			// Act
			webtest.StreamOnce(t, webtest.Serve(t, deps, tt.cfg), "/api/events"+tt.query)

			// Assert
			got := kept.all()
			if len(got) != 1 || got[0].jql != tt.want || !slices.Equal(got[0].keys, []jira.Key{webtest.TestKey}) {
				t.Errorf("kept %+v, want %s's first page, [%s], once", got, tt.want, webtest.TestKey)
			}
		})
	}
}

func TestAViewsFirstPageIsKeptOnceASearch(t *testing.T) {
	t.Parallel()

	// Arrange
	clock, tracker, kept := newSharedClock(), &countedSearch{}, &keptLists{}
	deps := tracker.wire(webtest.FilledDeps(), clock)
	deps.Store.CacheIssues = kept.keep
	handler := webtest.Serve(t, deps, config.Default())

	// Act: two frames in one interval
	webtest.StreamOnce(t, handler, "/api/events")
	webtest.StreamOnce(t, handler, "/api/events")

	// Assert: one search, so one keep
	if got := len(kept.all()); got != 1 {
		t.Errorf("kept %d in one interval, want 1", got)
	}

	// Act: a frame past the interval
	clock.pastTheForgeInterval()
	webtest.StreamOnce(t, handler, "/api/events")

	// Assert: a keep for each search, not for each frame
	if got := len(kept.all()); got != 2 {
		t.Errorf("kept %d after the interval, want 2", got)
	}
}

func TestNoIssueListIsKept(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*webserver.Deps)
		info   webserver.Info
	}{
		"after a search that failed": {
			unwire: func(deps *webserver.Deps) {
				deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
					return jira.SearchResult{}, webtest.ErrSeam
				}
			},
		},
		"under --dry-run": {unwire: func(*webserver.Deps) {}, info: webserver.Info{DryRun: true}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept, deps := &keptLists{}, webtest.FilledDeps()
			deps.Store.CacheIssues = kept.keep
			tt.unwire(&deps)

			// Act
			webtest.StreamOnce(t, webtest.ServeWith(t, deps, config.Default(), tt.info), "/api/events")

			// Assert
			if got := kept.all(); len(got) != 0 {
				t.Errorf("kept %+v, want nothing", got)
			}
		})
	}
}

func TestAnIssueListTheStoreCouldNotKeepIsNoted(t *testing.T) {
	t.Parallel()

	// Arrange
	var noted []error

	kept, deps := &keptLists{err: webtest.ErrSeam}, webtest.FilledDeps()
	deps.Store.CacheIssues = kept.keep
	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err) }

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if len(noted) != 1 || !errors.Is(noted[0], webtest.ErrSeam) {
		t.Errorf("noted %v, want the store's failure once", noted)
	}

	if len(frame.Issues.Issues) != 1 || frame.Issues.Issues[0].Key != webtest.TestKey || frame.Problems != nil {
		t.Errorf("frame issues = %+v, problems %+v; want the list shown all the same", frame.Issues, frame.Problems)
	}
}

func TestAKeptIssueListOutlivesThePageThatSearchedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	held, kept, deps := webtest.NewHeld(), &keptLists{}, webtest.FilledDeps()
	deps.Jira.Search = heldFirst(held, deps.Jira.Search)
	deps.Store.CacheIssues = kept.keep
	handler := webtest.Serve(t, deps, config.Default())

	// Act
	leftMidRead(t, handler, held)

	// Assert
	// The page has gone; the search it began went on, and so did its keep.
	got := kept.all()
	if len(got) == 0 || got[0].ctxErr != nil {
		t.Errorf("kept %+v, want the first page kept under a context still live", got)
	}
}

// keptKey is the issue the store holds for a view in the seed tests.
const keptKey = "PROJ-31"

// keptIssue is the one issue the store holds for a view in the seed tests.
func keptIssue() jira.Issue {
	return jira.Issue{
		Key: keptKey, Summary: "Kept from last time", Status: "To Do", StatusCategory: jira.CategoryNew, Type: "Task",
	}
}

// keptFor is a store's CachedIssues holding issues for every view, recording
// the JQL of each view it is asked for.
func keptFor(issues []jira.Issue, asked *[]string) func(context.Context, string) ([]jira.Issue, bool) {
	return func(_ context.Context, jql string) ([]jira.Issue, bool) {
		*asked = append(*asked, jql)

		return issues, true
	}
}

// seedOf decodes the seed a message carries, failing t when it is not one.
func seedOf(t *testing.T, sent webtest.Message) api.Seed {
	t.Helper()

	var seed api.Seed

	err := json.Unmarshal([]byte(sent.Data), &seed)
	if sent.Event != webtest.SeedMessage || err != nil {
		t.Fatalf("message %+v is not a seed: %v", sent, err)
	}

	return seed
}

func TestStreamSendsTheKeptIssuesBeforeAskingJira(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		asked        []string
		whenSearched string
	)

	recorder := httptest.NewRecorder()
	deps := webtest.FilledDeps()
	deps.Store.CachedIssues = keptFor([]jira.Issue{keptIssue()}, &asked)
	search := deps.Jira.Search
	deps.Jira.Search = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
		whenSearched = recorder.Body.String()

		return search(ctx, jql, startAt)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	request.Host = webtest.LoopbackHost

	// Act
	webtest.Serve(t, deps, config.Default()).ServeHTTP(recorder, request)

	// Assert
	before := webtest.Messages(whenSearched)
	if len(before) != 1 {
		t.Fatalf("Jira was asked after %d messages (%q), want after the seed alone", len(before), whenSearched)
	}

	want := api.IssuesPage{
		Issues: []api.Issue{{
			Key: keptKey, Tracker: api.IssueTrackerJira, Summary: "Kept from last time", Status: "To Do",
			StatusCategory: api.StatusCategoryNew, Type: "Task",
		}},
		Total: 1, StartAt: 0, Unavailable: []string{},
	}
	if got := seedOf(t, before[0]).Issues; !reflect.DeepEqual(got, want) {
		t.Errorf("seed = %+v, want %+v", got, want)
	}

	sent := webtest.Messages(recorder.Body.String())
	if len(sent) != 2 || sent[0].ID != "1" || sent[1].Event != webtest.SnapshotMessage || sent[1].ID != "2" {
		t.Errorf("messages = %+v, want the seed as 1, then a snapshot as 2", sent)
	}
}

func TestStreamSeedsANamedViewFromItsOwnKeptList(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	deps := webtest.FilledDeps()
	deps.Store.CachedIssues = keptFor([]jira.Issue{keptIssue()}, &asked)

	// Act
	webtest.StreamOnce(t, webtest.Serve(t, deps, bugsView()), "/api/events?view="+webtest.TestBugView)

	// Assert
	if want := []string{jira.ScopedToMe(webtest.TestBugJQL)}; !slices.Equal(asked, want) {
		t.Errorf("the store was asked for %q, want %q", asked, want)
	}
}

func TestStreamSeedsAViewKeptEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	deps := webtest.FilledDeps()
	deps.Store.CachedIssues = keptFor([]jira.Issue{}, &asked)

	// Act
	sent := webtest.Messages(webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if seed := seedOf(t, sent[0]); len(seed.Issues.Issues) != 0 || seed.Issues.Total != 0 {
		t.Errorf("seed = %+v, want an empty page", seed)
	}
}

func TestStreamSendsNoSeed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire   func(*webserver.Deps)
		info     webserver.Info
		asksNone bool
	}{
		"with nothing kept for the view": {
			unwire: func(deps *webserver.Deps) {
				deps.Store.CachedIssues = func(context.Context, string) ([]jira.Issue, bool) { return nil, false }
			},
		},
		"with no store to read": {unwire: func(deps *webserver.Deps) { deps.Store.CachedIssues = nil }},
		"with no tracker to ask again": {
			unwire: func(deps *webserver.Deps) { deps.Jira.Search = nil }, asksNone: true,
		},
		"under --dry-run": {unwire: func(*webserver.Deps) {}, info: webserver.Info{DryRun: true}, asksNone: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var asked []string

			deps := webtest.FilledDeps()
			deps.Store.CachedIssues = keptFor([]jira.Issue{keptIssue()}, &asked)
			tt.unwire(&deps)

			// Act
			sent := webtest.Messages(webtest.StreamOnce(t, webtest.ServeWith(t, deps, config.Default(), tt.info),
				"/api/events").Body.String())

			// Assert
			if len(sent) == 0 || sent[0].Event != webtest.SnapshotMessage || sent[0].ID != "1" {
				t.Errorf("messages = %+v, want a snapshot first, as 1", sent)
			}

			if tt.asksNone && len(asked) != 0 {
				t.Errorf("the store was asked for %q, want it not asked", asked)
			}
		})
	}
}

func TestASeedCarriesAStatusUnderNoCategoryAsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A row read back from disk is not trusted to fit the contract: carried as
	// it was kept, the page would refuse the whole seed.
	var asked []string

	odd := keptIssue()
	odd.StatusCategory = noCategory

	deps := webtest.FilledDeps()
	deps.Store.CachedIssues = keptFor([]jira.Issue{odd}, &asked)

	// Act
	sent := webtest.Messages(webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	seed := seedOf(t, sent[0])
	if len(seed.Issues.Issues) != 1 || seed.Issues.Issues[0].StatusCategory != api.StatusCategoryNew {
		t.Errorf("seed = %+v, want the kept row carried as not yet started", seed)
	}
}

func TestTheSeedsReadOfTheStoreStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := webtest.NewHeldUntilEnded(), webtest.FilledDeps()
	deps.Store.CachedIssues = func(ctx context.Context, _ string) ([]jira.Issue, bool) {
		return nil, held.Wait(ctx) == nil
	}
	stream := webtest.OpenStream(t, webtest.Serve(t, deps, config.Default()), "/api/events")

	select {
	case <-held.Entered:
	case <-time.After(webtest.EndsWithin):
		t.Fatal("the stream never read the store for a seed")
	}

	// Act
	stream.Cancel()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the store's read ended with %v, want it canceled once its page left", err)
	}

	<-stream.Done
}

func TestAStreamWhoseSeedCannotBeWrittenAsksJiraNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The context never cancels, so only the failed seed can end the stream,
	// before any snapshot is read.
	var asked, searched []string

	deps := webtest.FilledDeps()
	deps.Store.CachedIssues = keptFor([]jira.Issue{keptIssue()}, &asked)
	deps.Jira.Search = func(_ context.Context, jql string, _ int) (jira.SearchResult, error) {
		searched = append(searched, jql)

		return jira.SearchResult{}, nil
	}

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/events", nil)
	request.Host = webtest.LoopbackHost
	done := make(chan struct{})

	// Act
	go func() {
		webtest.Serve(t, deps, config.Default()).ServeHTTP(&webtest.FailingFlushWriter{}, request)
		close(done)
	}()

	// Assert
	select {
	case <-done:
	case <-time.After(webtest.EndsWithin):
		t.Fatal("the stream did not stop after its seed could not be written")
	}

	if len(asked) != 1 || len(searched) != 0 {
		t.Errorf("the store was asked %q and Jira searched %q; want the seed read, and Jira never asked", asked,
			searched)
	}
}
