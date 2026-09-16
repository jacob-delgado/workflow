// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Errors doctor reports. Callers distinguish them with errors.Is.
var (
	// errIncomplete reports a configuration that loaded but is missing fields.
	errIncomplete = errors.New("configuration is incomplete")
	// errMissingTooling reports a required external program that is absent.
	errMissingTooling = errors.New("required tooling is missing")
)

// labelWidth keeps the report's values in one column so the eye can scan them.
const labelWidth = 14

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
			effect:   "the hook panes stay hidden without it",
		},
		{
			name:     "gh",
			required: false,
			effect:   "supplies a GitHub token when none is configured",
		},
		{
			name:     "glab",
			required: false,
			effect:   "supplies a GitLab token when none is configured",
		},
	}
}

// newDoctorCmd builds `workflow doctor`.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report the repository, tooling, and configuration in effect",
		Long: "Report the git repository this session is in, which external\n" +
			"programs are installed, which " + config.FileName + " is in effect,\n" +
			"and which required fields are still empty. Makes no network calls,\n" +
			"so it never tells you a token is valid — only that one is present.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadFromEnvironment()

			return runDoctor(cmd.Context(), cmd.OutOrStdout(), cfg, err)
		},
	}
}

// runDoctor writes the report. Every section runs even when an earlier one found
// a problem: someone running doctor wants the whole picture, not the first thing
// that went wrong.
func runDoctor(ctx context.Context, out io.Writer, cfg config.Config, loadErr error) error {
	reportRepository(ctx, out)
	fmt.Fprintln(out)

	toolingErr := reportTooling(out)
	fmt.Fprintln(out)

	return errors.Join(toolingErr, reportConfiguration(out, cfg, loadErr))
}

// reportRepository describes the git repository the working directory is in. A
// directory outside any work tree is reported rather than returned as an error:
// `workflow doctor` is exactly what someone runs to find that out.
func reportRepository(ctx context.Context, out io.Writer) {
	dir, err := os.Getwd()
	if err != nil {
		field(out, "Repository", fmt.Sprintf("(cannot read the working directory: %v)", err))

		return
	}

	repo, err := gitrepo.Describe(ctx, proc.Run, dir)
	if err != nil {
		field(out, "Repository", fmt.Sprintf("(none — %s is not in a git work tree)", dir))

		return
	}

	field(out, "Repository", repo.Root)
	field(out, "Branch", branchLabel(repo))
	field(out, "Remote", describe(repo.Remote))
}

// branchLabel names the checked-out branch, or says why there isn't one.
func branchLabel(repo gitrepo.Repo) string {
	if repo.Detached {
		return "(detached HEAD — check out a branch before starting work)"
	}

	return repo.Branch
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

// reportConfiguration writes the configuration section. A load error is part of
// the report rather than a failure to produce one: "there is no configuration
// file" is exactly what someone running doctor is asking about.
func reportConfiguration(out io.Writer, cfg config.Config, loadErr error) error {
	if loadErr != nil {
		return reportLoadError(out, loadErr)
	}

	field(out, "Configuration", cfg.Path)
	field(out, "Jira", fmt.Sprintf("%s (%s)", describe(cfg.Jira.BaseURL), cfg.Jira.AuthMode()))
	// The target, never the credential: a webhook URL is itself the secret, and
	// this output is what the bug report template invites people to paste.
	field(out, "Slack", fmt.Sprintf("%s (%s)", cfg.Slack.Target(), cfg.Slack.Mode()))

	missing := cfg.Missing()
	if len(missing) == 0 {
		fmt.Fprintf(out, "\nEverything required is set.\n")

		return nil
	}

	fmt.Fprintf(out, "\nMissing:\n")

	for _, name := range missing {
		fmt.Fprintf(out, "  - %s\n", name)
	}

	fmt.Fprintf(out, "\nEdit %s, then run `workflow doctor` again.\n", cfg.Path)
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

	return fmt.Errorf("%w: %d field(s) missing", errIncomplete, len(missing))
}

// reportLoadError explains a configuration that could not be read, and says what
// to do about it.
func reportLoadError(out io.Writer, loadErr error) error {
	if errors.Is(loadErr, config.ErrNotFound) {
		fmt.Fprintf(out, "No %s found here or in your home directory.\n", config.FileName)
		fmt.Fprintf(out, "\nCreate one with `workflow config init`.\n")
		fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

		return loadErr
	}

	return loadErr
}

// field writes one aligned "Label: value" line.
func field(out io.Writer, label, value string) {
	fmt.Fprintf(out, "%-*s %s\n", labelWidth, label+":", value)
}

// describe renders an unset value as something a reader can act on.
func describe(value string) string {
	if value == "" {
		return "(not set)"
	}

	return value
}
