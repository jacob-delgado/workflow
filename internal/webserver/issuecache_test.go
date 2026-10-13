// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
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
	cfg.Jira.Views = []config.JiraView{{Name: testBugView, JQL: testBugJQL}}

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
		"a named view": {cfg: bugsView(), query: "?view=" + testBugView, want: jira.ScopedToMe(testBugJQL)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept, deps := &keptLists{}, filledDeps()
			deps.Store.CacheIssues = kept.keep

			// Act
			streamOnce(t, serve(t, deps, tt.cfg), "/api/events"+tt.query)

			// Assert
			got := kept.all()
			if len(got) != 1 || got[0].jql != tt.want || !slices.Equal(got[0].keys, []jira.Key{testKey}) {
				t.Errorf("kept %+v, want %s's first page, [%s], once", got, tt.want, testKey)
			}
		})
	}
}

func TestAViewsFirstPageIsKeptOnceASearch(t *testing.T) {
	t.Parallel()

	// Arrange
	clock, tracker, kept := newSharedClock(), &countedSearch{}, &keptLists{}
	deps := tracker.wire(filledDeps(), clock)
	deps.Store.CacheIssues = kept.keep
	handler := serve(t, deps, config.Default())

	// Act: two frames in one interval
	streamOnce(t, handler, "/api/events")
	streamOnce(t, handler, "/api/events")

	// Assert: one search, so one keep
	if got := len(kept.all()); got != 1 {
		t.Errorf("kept %d in one interval, want 1", got)
	}

	// Act: a frame past the interval
	clock.pastTheForgeInterval()
	streamOnce(t, handler, "/api/events")

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
					return jira.SearchResult{}, errSeam
				}
			},
		},
		"under --dry-run": {unwire: func(*webserver.Deps) {}, info: webserver.Info{DryRun: true}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept, deps := &keptLists{}, filledDeps()
			deps.Store.CacheIssues = kept.keep
			tt.unwire(&deps)

			// Act
			streamOnce(t, serveWith(t, deps, config.Default(), tt.info), "/api/events")

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

	kept, deps := &keptLists{err: errSeam}, filledDeps()
	deps.Store.CacheIssues = kept.keep
	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err) }

	// Act
	frame := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if len(noted) != 1 || !errors.Is(noted[0], errSeam) {
		t.Errorf("noted %v, want the store's failure once", noted)
	}

	if len(frame.Issues.Issues) != 1 || frame.Issues.Issues[0].Key != testKey || frame.Problems != nil {
		t.Errorf("frame issues = %+v, problems %+v; want the list shown all the same", frame.Issues, frame.Problems)
	}
}

func TestAKeptIssueListOutlivesThePageThatSearchedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	held, kept, deps := newHeld(), &keptLists{}, filledDeps()
	deps.Jira.Search = heldFirst(held, deps.Jira.Search)
	deps.Store.CacheIssues = kept.keep
	handler := serve(t, deps, config.Default())

	// Act
	leftMidRead(t, handler, held)

	// Assert
	// The page has gone; the search it began went on, and so did its keep.
	got := kept.all()
	if len(got) == 0 || got[0].ctxErr != nil {
		t.Errorf("kept %+v, want the first page kept under a context still live", got)
	}
}
