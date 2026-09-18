// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

func TestStandupDraftsFromTheRepository(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "config", "user.email", "me@example.com")
	git(t, dir, "config", "user.name", "Me")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "Add the widget")

	// Act
	out, err := run(t, dir, "standup", "--no-edit")
	// Assert
	if err != nil {
		t.Fatalf("standup in a repository failed: %v\n%s", err, out)
	}

	if !strings.Contains(out, "# Standup") || !strings.Contains(out, "Add the widget") {
		t.Errorf("standup did not draft the commit:\n%s", out)
	}
}

func TestStandupOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "standup", "--no-edit")

	// Assert
	if err == nil {
		t.Error("standup outside a repository returned no error")
	}
}
