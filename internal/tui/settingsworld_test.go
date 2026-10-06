// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// settingsToken is the Jira token in the configuration Settings reads. The
// fake hands it over unmasked, as the wiring never does, so a test can show
// the screen masks it all the same.
const settingsToken = "jira-secret-token-9999"

// settingsFile is the configuration Settings reads and saves.
func settingsFile() config.Config {
	cfg := config.Default()
	cfg.Jira.BaseURL, cfg.Jira.Token, cfg.Jira.Project = "https://jira.example.com", settingsToken, "PROJ"
	cfg.Messaging.Channel = devChannel
	cfg.Path = "/home/ana/src/api/.workflow.json"

	return cfg
}

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
		Read: func() (config.Config, config.Revision, error) {
			w.record("read-settings")

			w.mu.Lock()
			defer w.mu.Unlock()

			return w.settings, config.Revision{}, w.readSettingsErr
		},
		Save: func(edited config.Config, _ config.Revision) (config.Config, config.Revision, error) {
			w.record("save-settings")

			w.mu.Lock()
			defer w.mu.Unlock()

			if w.saveSettingsErr != nil {
				return config.Config{}, config.Revision{}, w.saveSettingsErr
			}

			w.settings = edited

			return edited, config.Revision{}, nil
		},
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
