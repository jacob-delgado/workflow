// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errConfigExists stands in for a configuration appearing before it was written.
var errConfigExists = errors.New("file already exists: this repository already has a lefthook configuration")

// legacyHooks is a repository's hooks from before lefthook: one plain, one not.
func legacyHooks() []hooks.GitHook {
	return []hooks.GitHook{
		{Name: "pre-commit", Script: "#!/bin/sh\nset -e\ngofmt -l .\ngo vet ./...\n"},
		{Name: "commit-msg", Script: "#!/usr/bin/env bash\ngrep -q '^feat' \"$1\"\n"},
	}
}

// lefthookHint is the Commits detail's line offering to set up lefthook, and
// lefthookKey the footer's key for it.
const (
	lefthookHint = "A hook is not managed by lefthook"
	lefthookKey  = "g set up lefthook"
)

// requireLefthookOffered fails unless the Commits pane offers to set up
// lefthook: the hint in its detail and g in the footer.
func requireLefthookOffered(t *testing.T, model tui.Model) {
	t.Helper()

	view := model.View().Content
	requireScreen(t, view, lefthookHint)

	if footer := footerLine(view); !strings.Contains(footer, lefthookKey) {
		t.Errorf("footer = %q, want it to offer %q", footer, lefthookKey)
	}
}

// refuseLefthookOffered fails if the Commits pane still offers to set up
// lefthook, in its detail or in the footer.
func refuseLefthookOffered(t *testing.T, model tui.Model) {
	t.Helper()

	view := model.View().Content
	refuseScreen(t, view, lefthookHint)

	if footer := footerLine(view); strings.Contains(footer, lefthookKey) {
		t.Errorf("footer = %q, still offers %q", footer, lefthookKey)
	}
}

func TestTheLefthookOfferOpensFromTheCommitsPaneNotAtStart(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act: open the interface
	model := offering.live(t, 120, 50)

	// Assert: the first screen is the panes, not the offer
	refuseScreen(t, model.View().Content, "No lefthook configuration")
	requireScreen(t, model.View().Content, "1 Issues")

	// Act: open the offer from the Commits pane
	opened := typing(t, model, "3", "g")

	// Assert: the offer is open
	requireScreen(t, opened.View().Content, "┏━ No lefthook configuration", "Found 2 hooks in .git/hooks")

	// Act: skip it, then open it again
	reopened := typing(t, opened, keyEsc, "g")

	// Assert: it reopens
	requireScreen(t, reopened.View().Content, "┏━ No lefthook configuration")
}

func TestExistingHooksAreOfferedALefthookConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act: open the interface, then the offer from the Commits pane
	model := typing(t, offering.live(t, 120, 50), "3", "g")

	// Assert: the hooks are offered as lefthook jobs and scripts
	requireScreen(t, model.View().Content, "┏━ No lefthook configuration", "Found 2 hooks in .git/hooks",
		"pre-commit · 4 lines", "commit-msg · 2 lines", "01-gofmt:", "1 hook kept whole as scripts under .lefthook",
		"enter write lefthook.yml", "v keep scripts whole", "esc skip")

	// Act: accept the offer
	written := typing(t, model, keyEnter)

	// Assert: the structured configuration is written
	requireScreen(t, written.View().Content, "● wrote lefthook.yml and installed lefthook")

	if calls := offering.asked("write"); len(calls) != 1 || !strings.Contains(calls[0], "01-gofmt") {
		t.Errorf("write calls = %q, want the structured configuration", calls)
	}
}

func TestWritingTheConfigurationEndsTheOffer(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act: open the interface on the Commits pane
	commits := typing(t, offering.live(t, 200, 50), "3")

	// Assert: it offers to set up lefthook
	requireLefthookOffered(t, commits)

	// Act: write the configuration
	written := typing(t, commits, "g", keyEnter)

	// Assert: it offers it no more
	refuseLefthookOffered(t, written)
}

func TestARefreshFindsALefthookConfigurationWrittenElsewhere(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act: open the interface on the Commits pane
	commits := typing(t, offering.live(t, 200, 50), "3")

	// Assert: it offers to set up lefthook
	requireLefthookOffered(t, commits)

	// Act: configure lefthook outside the interface, then refresh the pane
	offering.configured = true
	refreshed := typing(t, commits, "r")

	// Assert: it offers it no more
	refuseLefthookOffered(t, refreshed)
}

func TestARefreshFindsTheHooksRemovedElsewhere(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act: open the interface on the Commits pane
	commits := typing(t, offering.live(t, 200, 50), "3")

	// Assert: it offers to set up lefthook
	requireLefthookOffered(t, commits)

	// Act: remove the hooks outside the interface, then refresh the pane
	offering.gitHooks = nil
	refreshed := typing(t, commits, "r")

	// Assert: it offers it no more
	refuseLefthookOffered(t, refreshed)
}

func TestTheOfferCanKeepEveryHookWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	offering := newWorld()
	offering.gitHooks = legacyHooks()

	// Act
	typing(t, offering.live(t, 120, 50), "3", "g", "v")

	// Assert
	calls := offering.asked("write")
	if len(calls) != 1 || strings.Contains(calls[0], "commands:") || !strings.Contains(calls[0], "scripts:") {
		t.Errorf("write calls = %q, want every hook as a script", calls)
	}
}

func TestTheOfferIsMadeOnlyWhenItHelps(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		hooks      []hooks.GitHook
		configured bool
	}{
		"a repository lefthook already configures": {hooks: legacyHooks(), configured: true},
		"a repository with no hooks":               {hooks: nil, configured: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			quiet := newWorld()
			quiet.gitHooks, quiet.configured = tt.hooks, tt.configured

			// Act
			view := quiet.live(t, 120, 50).View().Content

			// Assert
			refuseScreen(t, view, "No lefthook configuration")
		})
	}
}

func TestASkippedOfferWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	skipping := newWorld()
	skipping.gitHooks = legacyHooks()

	// Act
	skipped := typing(t, skipping.live(t, 120, 50), "3", "g", keyEsc)

	// Assert
	refuseScreen(t, skipped.View().Content, "No lefthook configuration")

	if calls := skipping.asked("write"); len(calls) != 0 {
		t.Errorf("a skipped offer wrote: %q", calls)
	}
}

func TestAConfigurationThatCannotBeWrittenSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := newWorld()
	refusing.gitHooks = legacyHooks()
	refusing.writeErr = errConfigExists

	// Act
	refused := typing(t, refusing.live(t, 120, 50), "3", "g", keyEnter)

	// Assert
	requireScreen(t, refused.View().Content, "┏━ No lefthook configuration", "✗ file already exists")
}

func TestADryRunWritesNoConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.gitHooks = legacyHooks()

	model := sized(t, dryInterface(dry), 120, 50)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "3", "g", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would write lefthook.yml and 1 script, then install lefthook")

	if calls := dry.asked("write"); len(calls) != 0 {
		t.Errorf("a dry run wrote: %q", calls)
	}
}
