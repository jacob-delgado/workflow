// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestADryRunStartInAForksCloneFetchesUpstream(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.branch.Base, dry.branch.BaseRemote = "upstream/main", "upstream"
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "b", keyEnter).View().Content

	// Assert
	requireScreen(t, view,
		"dry run: would fetch upstream, then create fix/PROJ-412-fix-token-redaction from upstream/main and switch to it")
}
