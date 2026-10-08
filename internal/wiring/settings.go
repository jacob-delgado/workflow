// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
	"sync"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/store"
)

// settingsDeps reads and changes the configuration files and the local data,
// for the terminal's Settings and Local data. Slack user-token secrets and a
// Jira token typed into Settings are kept where controls keep them.
func (e Environment) settingsDeps(ctx context.Context, files config.Files, controls Controls) seams.Settings {
	editor := &configEditor{files: files, place: controls.PlaceSlackCredentials, keep: controls.KeepJiraToken}

	return seams.Settings{
		Read:            editor.read,
		Save:            editor.save,
		LocalData:       func() (string, []store.DataFile, error) { return e.localData(ctx) },
		RemoveLocalData: e.removeLocalData,
	}
}

// SetupDeps is the first run's seams where it runs: Jira checked over the
// redirect-refusing client, outlined in log unless it is nil, and the token
// kept in the keychain item for its address through storeSecret, nil where
// none is wired.
func SetupDeps(
	ctx context.Context, where setup.Where, log *RequestLog, storeSecret func(service, secret string) error,
) seams.Setup {
	guide := setup.Guide{
		//nolint:bodyclose // Wrap only relays the response; the Jira client reads and closes its body.
		Where: where, Doer: log.Wrap("jira", httpx.Client(config.DefaultRequestTimeout).Do), StoreSecret: storeSecret,
	}

	return seams.Setup{
		Offer: guide.Offer,
		Check: func(settings config.Jira) (string, error) { return guide.Check(ctx, settings) },
		Write: func(request setup.Request) (setup.Written, error) { return guide.Write(ctx, request) },
	}
}

// configEditor is the configuration as the terminal's Settings last read it,
// unmasked, so a save can keep each credential Settings was shown masked. It
// never hands a credential to the interface.
type configEditor struct {
	files config.Files
	place func(config.Config) (config.Config, error)
	keep  func(service, secret string) error

	mu   sync.Mutex
	last config.Config
	over config.Revision
}

// read reads the files as they are now, keeping what was read for the save
// made over it, and answers it masked.
func (e *configEditor) read() (config.Config, config.Revision, error) {
	cfg, over, err := config.LoadLayersAt(e.files)
	if err != nil {
		return config.Config{}, config.Revision{}, fmt.Errorf("reading %s: %w", e.files, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.last, e.over = cfg, over

	return cfg.Redacted(), over, nil
}

// save writes edited over the read at over, which must be the last one made:
// its masked credentials stand for that read's. Each credential in removed is
// written empty.
func (e *configEditor) save(
	edited config.Config, removed []config.Credential, over config.Revision,
) (config.Config, config.Revision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if over != e.over {
		return config.Config{}, config.Revision{}, fmt.Errorf("%s: %w", e.files, config.ErrChangedOnDisk)
	}

	saved, written, err := config.SaveEdit(config.Edit{
		Files: e.files, Read: e.last, Over: over, Edited: edited, Removed: removed, PlaceSlackCredentials: e.place,
		KeepJiraToken: e.keep,
	})
	if err != nil {
		return config.Config{}, config.Revision{}, err
	}

	e.last, e.over = saved, written

	return saved.Redacted(), written, nil
}

// localData is the store's directory and the database files in it, as
// db-clean and the web's Local data list them.
func (e Environment) localData(ctx context.Context) (string, []store.DataFile, error) {
	dir, err := e.StateDir()
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
func (e Environment) removeLocalData(scope store.CleanScope) error {
	dir, err := e.StateDir()
	if err != nil {
		return fmt.Errorf("finding the local data: %w", err)
	}

	err = store.Clean(dir, scope)
	if err != nil {
		return fmt.Errorf("removing the local data: %w", err)
	}

	return nil
}
