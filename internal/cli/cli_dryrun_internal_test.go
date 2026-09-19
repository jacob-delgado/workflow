// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// The bare `workflow` command builds the model and hands it to tui.Run, which
// needs a real terminal. These inject a fake runner in its place so the command's
// own wiring — in particular whether --dry-run reaches the model — is tested
// without one. Without this, deleting `if dryRun { model.WithDryRun() }` still
// passed the suite.

import (
	"context"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// runWithArgs runs the root command with args and a fake runner, returning the
// model the command built and handed off.
func runWithArgs(t *testing.T, args ...string) tui.Model {
	t.Helper()

	var captured tui.Model

	run := func(_ context.Context, model tui.Model, _ io.Writer) error {
		captured = model

		return nil
	}

	serve := func(context.Context, config.Config, webserver.Deps, webserver.Info, io.Writer) error {
		t.Error("the web server ran; bare workflow should open the TUI")

		return nil
	}

	root := newRootCmd(Prompt{}, run, serve)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err := root.Execute()
	if err != nil {
		t.Fatalf("the root command returned %v, want it to build the model and hand it off", err)
	}

	return captured
}

// shownAtSize renders a model once it has been told the terminal's size, since
// the spine that names dry run is only drawn when there is room for it.
func shownAtSize(t *testing.T, model tui.Model) string {
	t.Helper()

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	sized, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned a %T, not a tui.Model", updated)
	}

	return sized.View()
}

func TestTheDryRunFlagBuildsTheModelInDryRun(t *testing.T) {
	t.Parallel()

	// Act
	view := shownAtSize(t, runWithArgs(t, "--dry-run"))

	// Assert
	if !strings.Contains(view, "DRY RUN") {
		t.Errorf("the --dry-run flag did not put the model in dry run:\n%s", view)
	}
}

func TestWithoutTheFlagTheModelIsNotInDryRun(t *testing.T) {
	t.Parallel()

	// Act
	view := shownAtSize(t, runWithArgs(t))

	// Assert
	if strings.Contains(view, "DRY RUN") {
		t.Errorf("the model was in dry run with no flag asking for it:\n%s", view)
	}
}
