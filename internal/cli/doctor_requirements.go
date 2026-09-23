// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// reportRequirements names the fields still to fill in and the ones filled in
// wrong, or says everything is in order. A set-but-invalid value is a problem
// doctor exists to catch, not an all-clear.
func reportRequirements(out io.Writer, cfg config.Config) error {
	missing := cfg.Missing()
	problems := append(cfg.Problems(), forgeKindProblem(cfg.Forge)...)

	keyErr := tui.CheckKeys(cfg.UI.Keys)
	if keyErr != nil {
		problems = append(problems, keyErr.Error())
	}

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

// tool is an external program workflow uses, and what its absence costs.
type tool struct {
	name     string
	required bool
	effect   string
}

// externalTools names the programs doctor looks for. Built by a function rather
// than held in a package-level variable, which gochecknoglobals forbids.
func externalTools() []tool {
	return []tool{
		{
			name:     "git",
			required: true,
			effect:   "every repository action runs through it",
		},
		{
			name:     "lefthook",
			required: false,
			effect:   "the hook keys are not offered without it",
		},
		{
			name:     "gh",
			required: false,
			effect:   "supplies a GitHub token when none is configured",
		},
	}
}

// reportTooling lists the external programs and returns an error naming any
// required one that is absent.
func reportTooling(out io.Writer) error {
	fmt.Fprintln(out, "Tooling:")

	var missing []string

	for _, program := range externalTools() {
		installed := proc.Available(program.name)
		fmt.Fprintf(out, "  %-10s %s\n", program.name, toolStatus(program, installed))

		if !installed && program.required {
			missing = append(missing, program.name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", errMissingTooling, strings.Join(missing, ", "))
	}

	return nil
}

// toolStatus says whether a program was found, and what its absence costs.
func toolStatus(program tool, installed bool) string {
	if installed {
		return "found"
	}

	if program.required {
		return "MISSING — " + program.effect
	}

	return "not found — " + program.effect
}
