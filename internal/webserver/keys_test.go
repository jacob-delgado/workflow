// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The action and the review noun the keys cases read.
const (
	commentAction = "comment"
	pullNoun      = "pull request"
)

// echoKeyActions lists one action whose words carry what it was asked with —
// the review noun, the messaging service and comment's override — so a test
// reads back what the server handed the seam.
func echoKeyActions(reviewNoun, messagingService string, overrides map[string]string) []seams.KeyAction {
	return []seams.KeyAction{{
		Action: commentAction, Help: reviewNoun, Group: messagingService,
		Shown: overrides[commentAction], Keys: []string{overrides[commentAction]},
	}}
}

func TestGetKeysListsTheActionsUnderTheKeysInEffect(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.UI.Keys = map[string]string{commentAction: "C"}
	cfg.UI.WebShortcuts = true
	handler := serve(t, webserver.Deps{KeyActions: echoKeyActions}, cfg)

	// Act
	recorder := get(t, handler, "/api/keys")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	listed := decode[api.KeyList](t, recorder)

	want := api.KeyAction{Action: commentAction, Help: pullNoun, Group: "Slack", Shown: "C", Keys: []string{"C"}}
	if !listed.SingleKeyShortcuts || len(listed.Actions) != 1 || listed.Actions[0].Shown != want.Shown ||
		listed.Actions[0].Help != want.Help || listed.Actions[0].Group != want.Group ||
		!slices.Equal(listed.Actions[0].Keys, want.Keys) {
		t.Errorf("keys = %+v, want shortcuts on and %+v", listed, want)
	}
}

func TestGetKeysListsNoActionWhereNoneIsWired(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, webserver.Deps{}, config.Default())

	// Act
	recorder := get(t, handler, "/api/keys")

	// Assert
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"actions":[],"single_key_shortcuts":false}`+"\n" {
		t.Errorf("GET /api/keys = %d %s, want an empty list with the shortcuts off", recorder.Code, recorder.Body.String())
	}
}

func TestGetKeysKeepsTheKeysInEffectOverAFileThatIsNotValid(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.UI.WebShortcuts = true
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	handler := serve(t, webserver.Deps{KeyActions: echoKeyActions}, cfg)

	err := os.WriteFile(cfg.Path, []byte(`{"ui": {"web_shortcuts": "no"}}`), 0o600)
	if err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	// Act
	recorder := get(t, handler, "/api/keys")

	// Assert
	listed := decode[api.KeyList](t, recorder)
	if recorder.Code != http.StatusOK || !listed.SingleKeyShortcuts || len(listed.Actions) != 1 {
		t.Errorf("GET /api/keys = %d %+v, want the keys in effect, shortcuts on", recorder.Code, listed)
	}
}

func TestGetKeysTakesUpARebindingSavedByHand(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	handler := serve(t, webserver.Deps{KeyActions: echoKeyActions}, cfg)

	err := os.WriteFile(cfg.Path, []byte(`{"ui": {"keys": {"comment": "C"}}}`), 0o600)
	if err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	// Act
	recorder := get(t, handler, "/api/keys")

	// Assert
	listed := decode[api.KeyList](t, recorder)
	if len(listed.Actions) != 1 || listed.Actions[0].Shown != "C" {
		t.Errorf("GET /api/keys = %+v, want comment on the C the file now names", listed)
	}
}
