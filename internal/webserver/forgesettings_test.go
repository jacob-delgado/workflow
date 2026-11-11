// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// Forge settings saved in Settings apply at once: the next forge call uses
// them, and the page names the forge they point at, with no restart.

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// savedHost is the host the saved forge settings name.
const savedHost = "git.example.com"

// savedAuthor is whom the forge token Settings saves names.
const savedAuthor = "hubot"

// settingsRecorder is a UseForgeSettings that remembers the settings it was
// handed and answers kind for them.
type settingsRecorder struct {
	lock  sync.Mutex
	saved []config.Forge
	kind  forge.Kind
}

func (r *settingsRecorder) use(settings config.Forge) forge.Kind {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.saved = append(r.saved, settings)

	return r.kind
}

func (r *settingsRecorder) last() (config.Forge, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.saved) == 0 {
		return config.Forge{}, false
	}

	return r.saved[len(r.saved)-1], true
}

// savingForge is a server over a configuration file of its own, whose saved
// forge settings reach recorder.
func savingForge(t *testing.T, recorder *settingsRecorder) (http.Handler, config.Config) {
	t.Helper()

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	deps := webserver.Deps{UseForgeSettings: recorder.use}

	return webtest.ServeWith(t, deps, cfg, webserver.Info{Version: webtest.TestVersion, ForgeKind: forge.KindGitHub}), cfg
}

func TestSavedForgeSettingsAreHandedOnAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	recorder := &settingsRecorder{kind: forge.KindGitLab}
	handler, cfg := savingForge(t, recorder)
	next := cfg
	next.Forge = config.Forge{Token: "glpat-saved-in-settings", Kind: "gitlab", Host: savedHost}

	// Act
	saved := webtest.PutConfig(t, handler, webtest.Marshal(t, next))

	// Assert
	if saved.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", saved.Code, saved.Body.String())
	}

	used, ok := recorder.last()
	if !ok || used.Token.Reveal() != "glpat-saved-in-settings" || used.Host != savedHost {
		t.Errorf("the forge was handed %+v (%t), want the settings just saved", used, ok)
	}
}

func TestThePageNamesTheForgeTheSavedSettingsPointAt(t *testing.T) {
	t.Parallel()

	// Arrange
	recorder := &settingsRecorder{kind: forge.KindGitLab}
	handler, cfg := savingForge(t, recorder)
	next := cfg
	next.Forge = config.Forge{Kind: "gitlab", Host: savedHost}
	webtest.PutConfig(t, handler, webtest.Marshal(t, next))

	// Act
	health := webtest.Decode[api.Health](t, webtest.Get(t, handler, "/api/health"))

	// Assert
	if health.ForgeNoun != forge.KindGitLab.Noun() {
		t.Errorf("health names a %q, want the %q the saved settings point at", health.ForgeNoun, forge.KindGitLab.Noun())
	}
}

func TestAForgeTokenEditedOnDiskIsHandedOnOnceRead(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)
	webtest.Rewrite(t, path, fileAtStart)

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("loading the served file: %v", err)
	}

	recorder := &settingsRecorder{kind: forge.KindGitLab}
	handler := webtest.ServeWith(t, webserver.Deps{UseForgeSettings: recorder.use}, cfg,
		webserver.Info{Version: webtest.TestVersion})
	webtest.Rewrite(t, path, `{"jira": {"token": "`+fileToken+`"}, "forge": {"token": "glpat-edited-on-disk"}}`)

	// Act
	webtest.Get(t, handler, "/api/config")

	// Assert
	used, ok := recorder.last()
	if !ok || used.Token.Reveal() != "glpat-edited-on-disk" {
		t.Errorf("the forge was handed %+v (%t), want the token edited on disk", used, ok)
	}
}

func TestAForgeTokenSavedInSettingsNamesItsOwnAuthor(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge answers for whichever token its connection holds: the one
	// the server started with, then the one Settings saved.
	recorder := &settingsRecorder{kind: forge.KindGitHub}
	deps := webtest.FilledDeps()
	deps.UseForgeSettings = recorder.use
	deps.Forge.Author = func(context.Context) (string, error) {
		if _, saved := recorder.last(); saved {
			return savedAuthor, nil
		}

		return webtest.TestAuthor, nil
	}

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	handler := webtest.Serve(t, deps, cfg)
	webtest.Get(t, handler, "/api/messaging")

	next := cfg

	next.Forge = config.Forge{Token: "ghp-another-account", Kind: "github"}
	if saved := webtest.PutConfig(t, handler, webtest.Marshal(t, next)); saved.Code != http.StatusOK {
		t.Fatalf("saving the forge token answered %d: %s", saved.Code, saved.Body.String())
	}

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, handler, "/api/messaging"))

	// Assert
	if destination.Author != savedAuthor {
		t.Errorf("author = %q after a new forge token was saved, want %q, whom it names", destination.Author, savedAuthor)
	}
}

func TestAnAuthorAskedForBeforeAForgeTokenWasSavedIsNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first ask goes out over the old token and answers after the save;
	// every later one goes out over the token saved.
	var asks atomic.Int32

	held := webtest.NewHeld()
	recorder := &settingsRecorder{kind: forge.KindGitHub}
	deps := webtest.FilledDeps()
	deps.UseForgeSettings = recorder.use
	deps.Forge.Author = func(ctx context.Context) (string, error) {
		if asks.Add(1) > 1 {
			return savedAuthor, nil
		}

		return webtest.TestAuthor, held.Wait(ctx)
	}

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	handler := webtest.Serve(t, deps, cfg)

	var asking sync.WaitGroup

	asking.Go(func() { webtest.Get(t, handler, "/api/messaging") })
	<-held.Entered

	next := cfg

	next.Forge = config.Forge{Token: "ghp-another-account", Kind: "github"}
	if saved := webtest.PutConfig(t, handler, webtest.Marshal(t, next)); saved.Code != http.StatusOK {
		t.Fatalf("saving the forge token answered %d: %s", saved.Code, saved.Body.String())
	}

	close(held.Release)
	asking.Wait()

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, handler, "/api/messaging"))

	// Assert
	if destination.Author != savedAuthor {
		t.Errorf("author = %q, want %q: the ask begun before the save named the old account", destination.Author, savedAuthor)
	}
}
