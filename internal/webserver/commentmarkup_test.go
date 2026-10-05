// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestACommentFollowsMarkdownTurnedOnInSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	// Settings shows the Markdown composer the moment the setting is saved, so
	// the comment it writes must be converted from then on, not after a restart.
	var comments []commentCall

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	handler := serve(t, commentingDeps(&comments), cfg)

	next := config.Default()
	next.Jira.MarkdownComments = true

	if saved := putConfig(t, handler, marshal(t, next)); saved.Code != http.StatusOK {
		t.Fatalf("saving Settings: status %d (%s)", saved.Code, saved.Body.String())
	}

	// Act
	recorder := send(t, handler, http.MethodPost, commentPath, `{"text":"See **the docs** at `+"`run()`"+`."}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if len(comments) != 1 || comments[0].text != "See *the docs* at {{run()}}." {
		t.Errorf("comments = %+v, want the wiki markup Markdown converts to", comments)
	}
}
