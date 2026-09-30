// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// Forge settings saved in Settings apply at once: the next forge call uses
// them, and the page names the forge they point at, with no restart.

import (
	"net/http"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// savedHost is the host the saved forge settings name.
const savedHost = "git.example.com"

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

	return serveWith(t, deps, cfg, webserver.Info{Version: testVersion, ForgeKind: forge.KindGitHub}), cfg
}

func TestSavedForgeSettingsAreHandedOnAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	recorder := &settingsRecorder{kind: forge.KindGitLab}
	handler, cfg := savingForge(t, recorder)
	next := cfg
	next.Forge = config.Forge{Token: "glpat-saved-in-settings", Kind: "gitlab", Host: savedHost}

	// Act
	saved := putConfig(t, handler, marshal(t, next))

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
	putConfig(t, handler, marshal(t, next))

	// Act
	health := decode[api.Health](t, get(t, handler, "/api/health"))

	// Assert
	if health.ForgeNoun != forge.KindGitLab.Noun() {
		t.Errorf("health names a %q, want the %q the saved settings point at", health.ForgeNoun, forge.KindGitLab.Noun())
	}
}

func TestAForgeTokenEditedOnDiskIsHandedOnOnceRead(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)
	rewrite(t, path, fileAtStart)

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the served file: %v", err)
	}

	recorder := &settingsRecorder{kind: forge.KindGitLab}
	handler := serveWith(t, webserver.Deps{UseForgeSettings: recorder.use}, cfg, webserver.Info{Version: testVersion})
	rewrite(t, path, `{"jira": {"token": "`+fileToken+`"}, "forge": {"token": "glpat-edited-on-disk"}}`)

	// Act
	get(t, handler, "/api/config")

	// Assert
	used, ok := recorder.last()
	if !ok || used.Token.Reveal() != "glpat-edited-on-disk" {
		t.Errorf("the forge was handed %+v (%t), want the token edited on disk", used, ok)
	}
}
