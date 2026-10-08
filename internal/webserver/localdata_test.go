// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The Settings page's Local data area lists the store's database files and
// cleans them: the cache alone, or the kept associations too.

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// storeDir is where the fake store keeps its files.
const storeDir = "/home/ana/.local/state/workflow"

// fakeStore is a store directory holding both database files until a clean
// removes them, failing each clean with failure when it is set.
type fakeStore struct {
	files   []store.DataFile
	cleaned []store.CleanScope
	failure error
}

// newFakeStore holds a cache and a kept file.
func newFakeStore() *fakeStore {
	return &fakeStore{files: []store.DataFile{
		{Name: "workflow.db", Kind: store.DataCache, Bytes: 12288, Holds: []store.Held{{What: "scopes", Count: 2}}},
		{Name: "kept.db", Kind: store.DataKept, Bytes: 8192, Holds: []store.Held{{What: "repository groups", Count: 1}}},
	}}
}

// wire hands the fake to deps.
func (f *fakeStore) wire(deps *webserver.Deps) {
	deps.LocalData = func(context.Context) (string, []store.DataFile, error) { return storeDir, f.files, nil }
	deps.RemoveLocalData = func(scope store.CleanScope) error {
		if f.failure != nil {
			return f.failure
		}

		f.cleaned = append(f.cleaned, scope)

		kept := f.files[:0]
		for _, file := range f.files {
			if file.Kind == store.DataKept && scope == store.CleanCache {
				kept = append(kept, file)
			}
		}

		f.files = kept

		return nil
	}
}

// storeServer is a server over a fake store, under info.
func storeServer(t *testing.T, fake *fakeStore, info webserver.Info) http.Handler {
	t.Helper()

	deps := filledDeps()
	fake.wire(&deps)

	return serveWith(t, deps, config.Default(), info)
}

func TestLocalDataListsEachFileWithWhatItHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := storeServer(t, newFakeStore(), webserver.Info{Version: testVersion})

	// Act
	answer := get(t, handler, "/api/local-data")

	// Assert
	if answer.Code != http.StatusOK {
		t.Fatalf("status %d: %s", answer.Code, answer.Body.String())
	}

	data := decode[api.LocalData](t, answer)
	if data.Dir != storeDir || len(data.Files) != 2 {
		t.Fatalf("listed %+v, want both files in %s", data, storeDir)
	}

	wantCache := api.LocalDataFile{
		Name: "workflow.db", Kind: api.LocalDataFileKindCache, Bytes: 12288, Size: "12.0 KiB",
		Holds: []api.LocalDataHeld{{What: "scopes", Count: 2}},
	}
	if !reflect.DeepEqual(data.Files[0], wantCache) {
		t.Errorf("the cache reads %+v, want %+v", data.Files[0], wantCache)
	}

	// The page warns in the words the terminal does, not a copy of its own.
	wantSaid := api.LocalDataConsequences{
		Cache: store.CleanCache.Consequence(), All: store.CleanAll.Consequence(),
	}
	if data.Consequences != wantSaid {
		t.Errorf("consequences = %+v, want %+v", data.Consequences, wantSaid)
	}

	if data.Files[1].Kind != api.LocalDataFileKindKept {
		t.Errorf("the kept file reads as %q", data.Files[1].Kind)
	}
}

func TestLocalDataSaysWhenTheServerRunsDry(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := storeServer(t, newFakeStore(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	data := decode[api.LocalData](t, get(t, handler, "/api/local-data"))

	// Assert
	if data.DryRun == nil || !*data.DryRun {
		t.Errorf("dry_run = %v, want true", data.DryRun)
	}
}

func TestCleaningLocalDataRemovesTheScopeAsked(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		query     string
		wantScope store.CleanScope
		wantLeft  int
	}{
		"the cache alone":    {query: "cache", wantScope: store.CleanCache, wantLeft: 1},
		"everything with it": {query: "all", wantScope: store.CleanAll, wantLeft: 0},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := newFakeStore()
			handler := storeServer(t, fake, webserver.Info{Version: testVersion})

			// Act
			answer := send(t, handler, http.MethodDelete, "/api/local-data?scope="+test.query, "")

			// Assert
			if answer.Code != http.StatusOK {
				t.Fatalf("status %d: %s", answer.Code, answer.Body.String())
			}

			if len(fake.cleaned) != 1 || fake.cleaned[0] != test.wantScope {
				t.Errorf("cleaned %v, want %v", fake.cleaned, test.wantScope)
			}

			if left := decode[api.LocalData](t, answer).Files; len(left) != test.wantLeft {
				t.Errorf("answered %d files after the clean, want %d", len(left), test.wantLeft)
			}
		})
	}
}

func TestCleaningLocalDataAnswersAFailureByItsKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		failure  error
		wantCode int
		wantKind api.ProblemCode
	}{
		"a file held open": {
			failure: fmt.Errorf("%w: busy", store.ErrNotCleaned), wantCode: http.StatusConflict, wantKind: api.Conflict,
		},
		"a symlink in a file's place": {
			failure: store.ErrCleanRefused, wantCode: http.StatusUnprocessableEntity, wantKind: api.Unprocessable,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := newFakeStore()
			fake.failure = test.failure
			handler := storeServer(t, fake, webserver.Info{Version: testVersion})

			// Act
			answer := send(t, handler, http.MethodDelete, "/api/local-data?scope=all", "")

			// Assert
			if answer.Code != test.wantCode {
				t.Fatalf("status %d, want %d: %s", answer.Code, test.wantCode, answer.Body.String())
			}

			if problem := decode[api.Problem](t, answer); problem.Code != test.wantKind {
				t.Errorf("code %q, want %q", problem.Code, test.wantKind)
			}
		})
	}
}

func TestCleaningLocalDataRefusesAnUnknownScope(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeStore()
	handler := storeServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	answer := send(t, handler, http.MethodDelete, "/api/local-data?scope=kept", "")

	// Assert
	if answer.Code != http.StatusBadRequest || len(fake.cleaned) != 0 {
		t.Errorf("status %d, cleaned %v; want 400 and nothing cleaned", answer.Code, fake.cleaned)
	}
}

func TestCleaningLocalDataIsRefusedInADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeStore()
	handler := storeServer(t, fake, webserver.Info{Version: testVersion, DryRun: true})

	// Act
	answer := send(t, handler, http.MethodDelete, "/api/local-data?scope=all", "")

	// Assert
	if answer.Code != http.StatusForbidden || len(fake.cleaned) != 0 {
		t.Errorf("status %d, cleaned %v; want 403 and nothing cleaned", answer.Code, fake.cleaned)
	}
}

func TestLocalDataIsUnprocessableWithoutAStore(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method string
		target string
	}{
		"listing":  {method: http.MethodGet, target: "/api/local-data"},
		"cleaning": {method: http.MethodDelete, target: "/api/local-data?scope=cache"},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := serve(t, filledDeps(), config.Default())

			// Act
			answer := send(t, handler, test.method, test.target, "")

			// Assert
			if answer.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d, want 422 with no store wired: %s", answer.Code, answer.Body.String())
			}
		})
	}
}

func TestLocalDataIsUnprocessableWithNowhereToKeepIt(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.LocalData = func(context.Context) (string, []store.DataFile, error) { return "", nil, store.ErrNoDir }
	handler := serve(t, deps, config.Default())

	// Act
	answer := get(t, handler, "/api/local-data")

	// Assert
	if answer.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422 with no store directory: %s", answer.Code, answer.Body.String())
	}
}
