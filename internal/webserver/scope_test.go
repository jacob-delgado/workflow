// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The scopes the suggestion tests tell apart: one learned from an earlier
// commit, one the configuration names, and one a new commit's message names.
const (
	learnedScope    = "api"
	configuredScope = "web"
	committedScope  = "cli"
)

// scopeStore is the on-disk store as the server sees it: the scope it holds,
// if any, how often it was read, and each scope recorded into it. A disabled
// store takes each scope and keeps none, as store.disabled does. The stream
// reads it from the connection's goroutine, so it is locked.
type scopeStore struct {
	mu       sync.Mutex
	scope    string
	holds    bool
	disabled bool
	reads    int
	recorded []string
}

// wire hands the store's seams to deps.
func (s *scopeStore) wire(deps *webserver.Deps) {
	deps.LastScope = func() (string, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.reads++

		return s.scope, s.holds
	}
	deps.RecordScope = func(scope string) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.recorded = append(s.recorded, scope)
		if !s.disabled {
			s.scope, s.holds = scope, true
		}
	}
}

// readCount is how often the store was read.
func (s *scopeStore) readCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reads
}

// withDefaultScope is the default configuration naming scope as the default.
func withDefaultScope(scope string) config.Config {
	cfg := config.Default()
	cfg.Commit.DefaultScope = scope

	return cfg
}

// committable is filledDeps with a staged change and a commit that lands.
func committable() webserver.Deps {
	deps := filledDeps()
	deps.Commit = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }

	return deps
}

func TestSnapshotCarriesTheSuggestedScope(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		store      *scopeStore
		defaultsTo string
		want       string
	}{
		"the learned scope, over the default": {
			store: &scopeStore{scope: learnedScope, holds: true}, defaultsTo: configuredScope, want: learnedScope,
		},
		"the default, with nothing learned": {
			store: &scopeStore{}, defaultsTo: configuredScope, want: configuredScope,
		},
		"the default, with no store": {store: nil, defaultsTo: configuredScope, want: configuredScope},
		"none, with neither":         {store: &scopeStore{}, defaultsTo: "", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			if tt.store != nil {
				tt.store.wire(&deps)
			}

			// Act
			recorder := streamOnce(t, serve(t, deps, withDefaultScope(tt.defaultsTo)), "/api/events")

			// Assert
			if got := firstSnapshot(t, recorder.Body.String()).SuggestedScope; got != tt.want {
				t.Errorf("suggested scope = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEveryFrameNamesTheSuggestedScopeEvenWhenEmpty(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, filledDeps(), config.Default()), "/api/events")

	// Assert
	// The page's schema requires the key, so an empty suggestion is sent as
	// one rather than left out, which would drop the frame.
	if body := recorder.Body.String(); !strings.Contains(body, `"suggested_scope":""`) {
		t.Errorf("frame = %q, want the suggested_scope key with an empty value", body)
	}
}

func TestTheStoreIsReadOnceAcrossFrames(t *testing.T) {
	t.Parallel()

	// Opening the store's database on every five-second frame is the cost the
	// cache exists to avoid, and a new repository — nothing learned yet — is
	// the usual case; a fast stream pushes several frames.
	cases := map[string]*scopeStore{
		"a learned scope": {scope: learnedScope, holds: true},
		"nothing learned": {},
	}

	for name, store := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			store.wire(&deps)

			ctx, cancel := context.WithCancel(context.Background())
			info := webserver.Info{Version: testVersion, StreamInterval: 2 * time.Millisecond}
			handler := serveWith(t, deps, config.Default(), info)
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
			pushes := strings.Count(recorder.Body.String(), "event: snapshot")
			if reads := store.readCount(); pushes < 2 || reads != 1 {
				t.Errorf("read the store %d times over %d frames, want once over at least 2", reads, pushes)
			}
		})
	}
}

func TestADryRunSuggestsTheDefaultWithoutOpeningTheStore(t *testing.T) {
	t.Parallel()

	// Arrange
	// A dry run opens no store, so it reads no learned scope; the configured
	// default still applies.
	store := &scopeStore{scope: learnedScope, holds: true}
	deps := filledDeps()
	store.wire(&deps)

	dryRun := webserver.Info{Version: testVersion, DryRun: true}
	handler := serveWith(t, deps, withDefaultScope(configuredScope), dryRun)

	// Act
	recorder := streamOnce(t, handler, "/api/events")

	// Assert
	got := firstSnapshot(t, recorder.Body.String()).SuggestedScope
	if got != configuredScope || store.readCount() != 0 {
		t.Errorf("suggested %q after %d store reads, want %q and none", got, store.readCount(), configuredScope)
	}
}

func TestCommitRecordsTheScope(t *testing.T) {
	t.Parallel()

	learned := func() *scopeStore { return &scopeStore{scope: learnedScope, holds: true} }
	cases := map[string]struct {
		store         *scopeStore
		body          string
		commitFails   bool
		wantFirst     string
		wantRecorded  []string
		wantSuggested string
	}{
		"a scope, as the message wrote it": {
			store: learned(), body: `{"type":"fix","scope":" cli ","subject":"redact"}`,
			wantFirst: learnedScope, wantRecorded: []string{committedScope}, wantSuggested: committedScope,
		},
		"no scope, which keeps the one learned": {
			store: learned(), body: `{"type":"fix","subject":"redact"}`,
			wantFirst: learnedScope, wantRecorded: nil, wantSuggested: learnedScope,
		},
		"a blank scope, likewise": {
			store: learned(), body: `{"type":"fix","scope":"  ","subject":"redact"}`,
			wantFirst: learnedScope, wantRecorded: nil, wantSuggested: learnedScope,
		},
		"a commit that does not land": {
			store: learned(), body: `{"type":"fix","scope":"cli","subject":"redact"}`, commitFails: true,
			wantFirst: learnedScope, wantRecorded: nil, wantSuggested: learnedScope,
		},
		// The terminal learns nothing with the store off, so neither does the
		// server: the configured default still opens the next form.
		"a store that keeps nothing (disabled)": {
			store: &scopeStore{disabled: true}, body: `{"type":"fix","scope":"cli","subject":"redact"}`,
			wantFirst: configuredScope, wantRecorded: []string{committedScope}, wantSuggested: configuredScope,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := committable()
			tt.store.wire(&deps)

			if tt.commitFails {
				deps.Commit = func(string) (proc.Output, error) { return fakeOutput(nil, errSeam), nil }
			}

			handler := serve(t, deps, withDefaultScope(configuredScope))
			first := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

			// Act
			send(t, handler, http.MethodPost, "/api/commit", tt.body)

			// Assert
			if !slices.Equal(tt.store.recorded, tt.wantRecorded) {
				t.Errorf("recorded %q, want %q", tt.store.recorded, tt.wantRecorded)
			}

			next := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())
			if first.SuggestedScope != tt.wantFirst || next.SuggestedScope != tt.wantSuggested {
				t.Errorf("suggested %q, then %q; want %q, then %q",
					first.SuggestedScope, next.SuggestedScope, tt.wantFirst, tt.wantSuggested)
			}

			// Read once, and once more after a commit that recorded a scope, to
			// learn what the store kept of it.
			if wantReads := 1 + len(tt.wantRecorded); tt.store.readCount() != wantReads {
				t.Errorf("read the store %d times, want %d", tt.store.readCount(), wantReads)
			}
		})
	}
}

func TestADefaultScopeSavedInSettingsIsSuggestedWhileNoneIsLearned(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		store *scopeStore
		want  string
	}{
		"nothing learned, so the saved default": {store: &scopeStore{}, want: configuredScope},
		"a scope learned, which still comes first": {
			store: &scopeStore{scope: learnedScope, holds: true}, want: learnedScope,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			tt.store.wire(&deps)

			cfg := config.Default()
			cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
			handler := serve(t, deps, cfg)
			saved := withDefaultScope(configuredScope)

			// Act
			recorder := putConfig(t, handler, marshal(t, saved))

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("saving the configuration: status = %d: %s", recorder.Code, recorder.Body.String())
			}

			next := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())
			if next.SuggestedScope != tt.want {
				t.Errorf("suggested %q after saving default_scope %q, want %q", next.SuggestedScope, configuredScope, tt.want)
			}
		})
	}
}
