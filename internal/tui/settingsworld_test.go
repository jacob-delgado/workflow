// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// anaStore is where the fake keeps its local data.
const anaStore = "/home/ana/.local/state/workflow"

// localFiles are the store's two files, as db-clean lists them.
func localFiles() []store.DataFile {
	return []store.DataFile{
		{Name: "workflow.db", Kind: store.DataCache, Bytes: 94208, Holds: []store.Held{{What: "scopes", Count: 3}}},
		{Name: "kept.db", Kind: store.DataKept, Bytes: 24576, Holds: []store.Held{{What: "favorite directories", Count: 2}}},
	}
}

// settingsDeps fakes the configuration files and the local data.
func (w *world) settingsDeps() seams.Settings {
	return seams.Settings{
		LocalData: func() (string, []store.DataFile, error) {
			w.record("local-data")

			w.mu.Lock()
			defer w.mu.Unlock()

			return anaStore, w.localFiles, w.localDataErr
		},
		RemoveLocalData: func(scope store.CleanScope) error {
			w.record("remove-local-data " + map[store.CleanScope]string{store.CleanCache: "cache", store.CleanAll: "all"}[scope])

			return w.removeErr
		},
	}
}
