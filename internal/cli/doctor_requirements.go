// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// configReview is everything doctor finds wrong with a configuration that
// loaded: the fields still empty, the values filled in wrong, and whether other
// users can reach the file. The prose and JSON reports each render one review,
// so they reach one verdict.
type configReview struct {
	missing  []string
	problems []string
	mode     os.FileMode
	shared   bool
	// sharedPath is the file others can reach, when one is.
	sharedPath string
}

// reviewConfiguration gathers everything doctor checks in a loaded
// configuration, once for both reports.
func reviewConfiguration(cfg config.Config) configReview {
	problems := cfg.Problems()

	keyErr := tui.CheckKeys(cfg.UI.Keys)
	if keyErr != nil {
		problems = append(problems, keyErr.Error())
	}

	review := configReview{missing: cfg.Missing(), problems: problems}

	for _, path := range cfg.Layers().Each() {
		if mode, shared := config.SharedMode(path); shared {
			review.mode, review.shared, review.sharedPath = mode, true, path

			break
		}
	}

	return review
}

// err is the review's verdict: nil for a configuration with nothing wrong,
// otherwise every kind of fault it found, each counted.
func (review configReview) err() error {
	var sharedErr error
	if review.shared {
		sharedErr = fmt.Errorf("%w: mode %#o", errShared, review.mode)
	}

	return errors.Join(
		sharedErr,
		countedError(errIncomplete, len(review.missing), "field(s) missing"),
		countedError(errInvalid, len(review.problems), "invalid value(s)"),
	)
}

// reportRequirements names the fields still to fill in and the ones filled in
// wrong, or says everything is in order. A set-but-invalid value is a problem
// doctor exists to catch, not an all-clear.
func reportRequirements(out io.Writer, path string, review configReview) {
	if len(review.missing) == 0 && len(review.problems) == 0 {
		fmt.Fprintf(out, "\nEverything required is set.\n")

		return
	}

	reportList(out, "Missing", review.missing)
	reportList(out, "Problems", review.problems)

	fmt.Fprintf(out, "\nEdit %s, then run `workflow doctor` again.\n", path)
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")
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
	// probe says whether the program is there to use, and a detail when that
	// takes more than finding its name on PATH; nil looks the name up.
	probe func() (bool, string)
}

// externalTools names the programs doctor looks for, gh and glab each whatever
// the repository's forge, since a developer may work with both. Built by a
// function rather than held in a package-level variable, which
// gochecknoglobals forbids.
func externalTools(ctx context.Context, run doctorRun, kind forge.Kind) []tool {
	cfg := run.cfg

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
			effect: forgeCLIEffect(cfg.Forge, kind, forge.KindGitHub,
				"supplies a GitHub token, and carries GitHub calls when forge.cli is on"),
		},
		{
			name:     "glab",
			required: false,
			effect:   forgeCLIEffect(cfg.Forge, kind, forge.KindGitLab, "carries GitLab calls when forge.cli is on"),
		},
		{
			name:     "taskwarrior",
			required: false,
			effect:   "the Tasks pane and section are not offered without it",
			probe:    func() (bool, string) { return taskwarriorProbe(ctx, run.env.Process, cfg.Taskwarrior) },
		},
	}
}

// forgeCLIEffect is what a missing gh or glab costs: what it is for, or — when
// forge.cli is on and the repository is on the CLI's own forge — that its calls
// go over HTTP with a token instead of through the CLI forge.cli asked for.
func forgeCLIEffect(settings config.Forge, repoKind, cliKind forge.Kind, usual string) string {
	if settings.CLI && repoKind == cliKind {
		return "forge.cli is on, so " + cliKind.String() + " calls go over HTTP with a token instead"
	}

	return usual
}

// lookFor says whether program is there to use, found on the PATH available
// searches, and any detail its probe gives.
func (program tool) lookFor(available func(name string) bool) (bool, string) {
	if program.probe == nil {
		return available(program.name), ""
	}

	return program.probe()
}

// toolingFacts looks for each external program, and names any required one that
// is absent. It is the one place the programs are looked for, so the prose and
// JSON reports cannot disagree about what is installed.
func toolingFacts(ctx context.Context, run doctorRun, remote string) ([]toolFacts, error) {
	programs := externalTools(ctx, run, wiring.ForgeKind(run.cfg.Forge, remote))
	facts := make([]toolFacts, 0, len(programs))

	var missing []string

	for _, program := range programs {
		installed, detail := program.lookFor(run.env.Process.Available)
		facts = append(facts, toolFacts{
			Name: program.name, Found: installed, Required: program.required, Effect: program.effect, Detail: detail,
		})

		if !installed && program.required {
			missing = append(missing, program.name)
		}
	}

	if len(missing) > 0 {
		return facts, fmt.Errorf("%w: %s", errMissingTooling, strings.Join(missing, ", "))
	}

	return facts, nil
}

// reportTooling lists the external programs and returns an error naming any
// required one that is absent. remote says which forge the repository is on.
func reportTooling(ctx context.Context, out io.Writer, run doctorRun, remote string) error {
	fmt.Fprintln(out, "Tooling:")

	facts, err := toolingFacts(ctx, run, remote)
	for _, program := range facts {
		fmt.Fprintf(out, "  %-10s %s\n", program.Name, toolStatus(program))
	}

	return err
}

// toolStatus says whether a program was found, and what its absence costs — or,
// where its probe said more, what it found or why it cannot be used.
func toolStatus(program toolFacts) string {
	switch {
	case program.Found && program.Detail != "":
		return "found — " + program.Detail
	case program.Found:
		return "found"
	case program.Detail != "":
		return "not usable — " + program.Detail
	case program.Required:
		return "MISSING — " + program.Effect
	default:
		return "not found — " + program.Effect
	}
}

// taskwarriorProbe finds Taskwarrior as the interface would, and says which it
// found or why none is usable. Taskwarrior is found by asking each task on PATH,
// not by its name alone: go-task, the Taskfile runner, is also called task.
func taskwarriorProbe(ctx context.Context, process wiring.Environment, settings config.Taskwarrior) (bool, string) {
	if settings.Disabled {
		return false, "disabled by taskwarrior.disabled"
	}

	candidates := taskwarrior.Candidates(process.Getenv("PATH"), runtime.GOOS)

	install, err := taskwarrior.Detect(ctx, settings.Program, candidates, process.CaptureWithin)
	if err != nil {
		return false, taskwarriorTrouble(err)
	}

	found := install.Version + " at " + install.Program
	if !install.LinkUDADefined {
		found += "; add uda.jiraid.type=string, uda.jiraid.label=Jira, uda.jiraurl.type=string and " +
			"uda.jiraurl.label=Jira URL to your taskrc so task jiraid:KEY works in your shell"
	}

	return true, found
}

// taskwarriorTrouble words why no Taskwarrior is usable, or says nothing when
// none is installed, which the row's effect already covers.
func taskwarriorTrouble(err error) string {
	switch {
	case errors.Is(err, taskwarrior.ErrNotInstalled):
		return ""
	case errors.Is(err, taskwarrior.ErrNotTaskwarrior):
		return "task on PATH is not Taskwarrior (go-task?); set taskwarrior.program, e.g. /opt/homebrew/bin/task"
	case errors.Is(err, taskwarrior.ErrNotConfigured):
		return "installed but never run; run " + neverRunProgram(err) + " once"
	case errors.Is(err, taskwarrior.ErrRefused):
		return sanitize.Line(strings.TrimPrefix(err.Error(), taskwarrior.ErrRefused.Error()+": "))
	default:
		// ErrTooOld names the version found, where, and the one needed.
		return sanitize.Line(err.Error())
	}
}

// neverRunProgram is the Taskwarrior found never run, by the path it was found
// at, since go-task can come first on PATH and answer to task; task when the
// error names none.
func neverRunProgram(err error) string {
	var neverRun taskwarrior.NeverRunError
	if errors.As(err, &neverRun) && neverRun.Program != "" {
		return sanitize.Line(neverRun.Program)
	}

	return "task"
}
