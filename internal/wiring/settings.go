// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// settingsDeps reads and changes the configuration files and the local data,
// for the terminal's Settings and Local data.
func settingsDeps(ctx context.Context) seams.Settings {
	return seams.Settings{
		LocalData:       func() (string, []store.DataFile, error) { return localData(ctx) },
		RemoveLocalData: removeLocalData,
	}
}

// localData is the store's directory and the database files in it, as
// db-clean and the web's Local data list them.
func localData(ctx context.Context) (string, []store.DataFile, error) {
	dir, err := store.DefaultDir()
	if err != nil {
		return "", nil, fmt.Errorf("finding the local data: %w", err)
	}

	files, err := store.Files(ctx, dir)
	if err != nil {
		return "", nil, fmt.Errorf("reading the local data: %w", err)
	}

	return dir, files, nil
}

// removeLocalData removes the store's files a clean of scope reaches.
func removeLocalData(scope store.CleanScope) error {
	dir, err := store.DefaultDir()
	if err != nil {
		return fmt.Errorf("finding the local data: %w", err)
	}

	err = store.Clean(dir, scope)
	if err != nil {
		return fmt.Errorf("removing the local data: %w", err)
	}

	return nil
}
