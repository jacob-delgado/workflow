// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

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

// forkMerged is a merged branch in a fork's clone, whose base is upstream's.
func forkMerged() *world {
	merged := mergedBranch()
	merged.branch.Base, merged.branch.BaseRemote = "upstream/main", "upstream"

	return merged
}

func TestTheFinishPreviewPullsAForksBaseFromUpstream(t *testing.T) {
	t.Parallel()

	// Act
	preview := typing(t, forkMerged().live(t, 120, 40), "4", "F").View().Content

	// Assert
	requireScreen(t, preview, "git pull --ff-only -- upstream main")
}

func TestAForksFinishIsAskedOfUpstream(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := forkMerged()

	// Act
	typing(t, merged.live(t, 120, 40), "4", "F", keyEnter)

	// Assert
	if calls := merged.asked("finish "); len(calls) != 1 || !strings.HasSuffix(calls[0], "onto main from upstream") {
		t.Errorf("finish calls = %q, want one onto main from upstream", calls)
	}
}
