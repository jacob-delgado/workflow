// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// reportRequirements names the fields still to fill in and the ones filled in
// wrong, or says everything is in order. A set-but-invalid value is a problem
// doctor exists to catch, not an all-clear.
func reportRequirements(out io.Writer, cfg config.Config) error {
	missing := cfg.Missing()
	problems := append(cfg.Problems(), forgeKindProblem(cfg.Forge)...)

	if len(missing) == 0 && len(problems) == 0 {
		fmt.Fprintf(out, "\nEverything required is set.\n")

		return nil
	}

	reportList(out, "Missing", missing)
	reportList(out, "Problems", problems)

	fmt.Fprintf(out, "\nEdit %s, then run `workflow doctor` again.\n", cfg.Path)
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

	return errors.Join(
		countedError(errIncomplete, len(missing), "field(s) missing"),
		countedError(errInvalid, len(problems), "invalid value(s)"),
	)
}

// forgeKindProblem reports a forge.kind that names no forge this build knows.
func forgeKindProblem(settings config.Forge) []string {
	if settings.Kind == "" {
		return nil
	}

	_, err := forge.ParseKind(settings.Kind)
	if err != nil {
		return []string{fmt.Sprintf("forge.kind %q is not github or gitlab", settings.Kind)}
	}

	return nil
}

// reportList prints a headed list of items, or nothing when there are none.
func reportList(out io.Writer, heading string, items []string) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(out, "\n%s:\n", heading)

	for _, item := range items {
		fmt.Fprintf(out, "  - %s\n", item)
	}
}

// countedError wraps sentinel with a count, or is nil when the count is zero.
func countedError(sentinel error, count int, noun string) error {
	if count == 0 {
		return nil
	}

	return fmt.Errorf("%w: %d %s", sentinel, count, noun)
}
