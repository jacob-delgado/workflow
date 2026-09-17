// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
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

func TestExistingHooksAreOfferedALefthookConfiguration(t *testing.T) {
	t.Parallel()

	offering := newWorld()
	offering.gitHooks = legacyHooks()

	view := offering.live(t, 120, 50).View()
	requireScreen(t, view, "┏━ No lefthook configuration", "Found 2 hooks in .git/hooks", "pre-commit · 4 lines",
		"commit-msg · 2 lines", "01-gofmt:", "1 hook kept whole as scripts under .lefthook",
		"enter write lefthook.yml", "v keep scripts whole", "esc skip")

	written := typing(t, offering.live(t, 120, 50), keyEnter)
	requireScreen(t, written.View(), "● wrote lefthook.yml and installed lefthook")

	if calls := offering.asked("write"); len(calls) != 1 || !strings.Contains(calls[0], "01-gofmt") {
		t.Errorf("write calls = %q, want the structured configuration", calls)
	}
}

func TestTheOfferCanKeepEveryHookWhole(t *testing.T) {
	t.Parallel()

	offering := newWorld()
	offering.gitHooks = legacyHooks()

	typing(t, offering.live(t, 120, 50), "v")

	calls := offering.asked("write")
	if len(calls) != 1 || strings.Contains(calls[0], "commands:") || !strings.Contains(calls[0], "scripts:") {
		t.Errorf("write calls = %q, want every hook as a script", calls)
	}
}

func TestTheOfferIsMadeOnlyWhenItHelps(t *testing.T) {
	t.Parallel()

	configured := newWorld()
	configured.gitHooks, configured.configured = legacyHooks(), true
	refuseScreen(t, configured.live(t, 120, 50).View(), "No lefthook configuration")

	nothing := newWorld()
	refuseScreen(t, nothing.live(t, 120, 50).View(), "No lefthook configuration")

	// Skipped once, it is not offered again in the session.
	skipping := newWorld()
	skipping.gitHooks = legacyHooks()

	skipped := typing(t, skipping.live(t, 120, 50), "esc")
	refuseScreen(t, skipped.View(), "No lefthook configuration")

	if calls := skipping.asked("write"); len(calls) != 0 {
		t.Errorf("a skipped offer wrote: %q", calls)
	}
}

func TestAConfigurationThatCannotBeWrittenSaysWhy(t *testing.T) {
	t.Parallel()

	refusing := newWorld()
	refusing.gitHooks = legacyHooks()
	refusing.writeErr = errConfigExists

	refused := typing(t, refusing.live(t, 120, 50), keyEnter)
	requireScreen(t, refused.View(), "┏━ No lefthook configuration", "✗ file already exists")
}

func TestADryRunWritesNoConfiguration(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.gitHooks = legacyHooks()

	model := sized(t, dryInterface(dry), 120, 50)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, keyEnter).View(),
		"dry run: would write lefthook.yml and 1 script, then install lefthook")

	if calls := dry.asked("write"); len(calls) != 0 {
		t.Errorf("a dry run wrote: %q", calls)
	}
}
