// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errRefusedWhereRemoved is why the fake refuses a setting as the file is
// written without a removed credential, for another reason than a save's write
// gave.
var errRefusedWhereRemoved = errors.New("refused where the credential is removed")

func TestARefusedRemovalKeepsEveryMarkBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")
	refusedWrite := typing(t, repo.live(t, 120, 80), editing(projectRow, editedProject)...)
	repo.mu.Lock()
	repo.saveSettingsErr = errSettingsUnreadable
	repo.mu.Unlock()

	toToken := slices.Repeat([]string{"k"}, projectRow-tokenRow)

	// Act
	view := typing(t, refusedWrite, append(toToken, removeKey, keyEnter)...).View().Content

	// Assert
	if under := lineUnder(t, view, projectLabel); !strings.Contains(under, "✗ "+errRefusedWhereWritten.Error()) {
		t.Errorf("under the project's row is %q, want the write's refusal kept:\n%s", under, plain(view))
	}

	requireScreen(t, view, errSettingsUnreadable.Error())
}

func TestARefusedRemovalMarksWhatItNamesBesideTheMarksBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")
	heldBack := typing(t, repo.live(t, 120, 80), templateAndTimeoutRefused()...)
	toToken := slices.Repeat([]string{"k"}, branchTemplateRow-tokenRow)

	// Act
	view := typing(t, heldBack, append(toToken, removeKey, keyEnter)...).View().Content

	// Assert
	if under := lineUnder(t, view, projectLabel); !strings.Contains(under, "✗ "+errRefusedWhereWritten.Error()) {
		t.Errorf("under the project's row is %q, want why the removal was refused:\n%s", under, plain(view))
	}

	if row := lineShowing(t, view, projectLabel); !strings.Contains(row, "▸ "+projectLabel) {
		t.Errorf("the project's row is %q, want the cursor on what the removal was refused for", row)
	}

	if under := lineUnder(t, view, templateLabel); !strings.Contains(under, "✗ "+templateRefused) {
		t.Errorf("under the template's row is %q, want the check's refusal kept:\n%s", under, plain(view))
	}
}

func TestARefusedRemovalsReasonForASettingReplacesTheWritesBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")
	refusedWrite := typing(t, repo.live(t, 120, 80), editing(projectRow, editedProject)...)
	repo.mu.Lock()
	repo.saveSettingsErr = fmt.Errorf("%w: %w", config.ErrInvalid,
		config.FieldError{Key: "jira.project", Err: errRefusedWhereRemoved})
	repo.mu.Unlock()

	toToken := slices.Repeat([]string{"k"}, projectRow-tokenRow)

	// Act
	view := typing(t, refusedWrite, append(toToken, removeKey, keyEnter)...).View().Content

	// Assert
	if under := lineUnder(t, view, projectLabel); !strings.Contains(under, "✗ "+errRefusedWhereRemoved.Error()) {
		t.Errorf("under the project's row is %q, want why the removal was refused:\n%s", under, plain(view))
	}

	refuseScreen(t, view, errRefusedWhereWritten.Error())
}
