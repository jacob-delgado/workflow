// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheCommitComposerOpensWithTheConfiguredDefaultScope(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	scoped := completeConfig()
	scoped.Commit.DefaultScope = "api"
	model := sized(t, tui.New(scoped, nil, composing.deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	composer := typing(t, model, "3", "c")

	// Assert
	requireScreen(t, composer.View(), "fix(api):")
}

func TestAKeptDraftScopeWinsWhenTheComposerReopens(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit with a scope of its own that the hook refuses keeps its draft for
	// the next time the composer opens.
	failing := newWorld()
	failing.commitErr = errHookFailed
	scoped := typing(t, failing.live(t, 120, 40), "3", "c", keyShiftTab, "d", "b", keyTab)
	failed := typing(t, scoped, append(letters("redact tokens"), keyEnter)...)

	// Act
	reopened := typing(t, failed, keyEsc, "c")

	// Assert
	requireScreen(t, reopened.View(), "fix(db):")
}
