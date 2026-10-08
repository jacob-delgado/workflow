// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/store"
)

// keptWarning is what --all says before removing the kept file: what it holds
// cannot be seen again, only asked for again.
const keptWarning = "warning: --all also removes kept.db, whom each code owner is on Slack and " +
	"each repository's groups; people and group associations will be asked again"

// newDBCleanCmd builds `workflow db-clean`.
func newDBCleanCmd(prompt Prompt) *cobra.Command {
	var (
		opts writeOptions
		all  bool
	)

	cmd := &cobra.Command{
		Use:   "db-clean",
		Short: "Remove workflow's local data, the databases it keeps between sessions",
		Long: "List the database files workflow keeps between sessions, with each one's size\n" +
			"and what it holds, then remove the cache (workflow.db) once confirmed: the last\n" +
			"scope, what was announced and the cached issue lists, which a session makes\n" +
			"again. --all also removes kept.db, whom each code owner is on Slack and each\n" +
			"repository's groups, which are then asked again. Nothing is removed under\n" +
			"--dry-run.",
		Example: examples(
			`workflow db-clean --dry-run   # each file and what it holds, nothing removed`,
			`workflow db-clean --yes       # remove the cache, unattended`,
			`workflow db-clean --all       # also forget whom each code owner is on Slack`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := environmentOf(cmd).Process.StateDir()
			if err != nil {
				return fmt.Errorf("finding the local data: %w", err)
			}

			clean := storeClean{dir: dir, scope: map[bool]store.CleanScope{false: store.CleanCache, true: store.CleanAll}[all]}
			ask := func(question string) (bool, error) { return confirm(prompt, question) }

			return runDBClean(cmd.Context(), outputOf(cmd), ask, clean, opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)
	cmd.Flags().BoolVar(&all, "all", false,
		"also remove kept.db, so people and group associations are asked again")

	return cmd
}

// storeClean is what a db-clean removes: the files a clean of scope reaches in
// the store's directory, dir.
type storeClean struct {
	dir   string
	scope store.CleanScope
}

// runDBClean lists the store's files, then removes those clean reaches once
// confirmed.
func runDBClean(
	ctx context.Context, out output, ask func(string) (bool, error), clean storeClean, opts writeOptions,
) error {
	files, err := store.Files(ctx, clean.dir)
	if err != nil {
		return fmt.Errorf("reading the local data: %w", err)
	}

	listDataFiles(out.artifact, clean.dir, files)

	reached := reachedBy(files, clean.scope)
	if len(reached) == 0 {
		fmt.Fprintln(out.notes, "Nothing to remove.")

		return nil
	}

	return removeDataFiles(out, ask, clean, reached, opts)
}

// removeDataFiles removes the files clean reaches once confirmed, warning
// first when the kept file is among them.
func removeDataFiles(
	out output, ask func(string) (bool, error), clean storeClean, reached []store.DataFile, opts writeOptions,
) error {
	dir := clean.dir

	names := make([]string, 0, len(reached))

	for _, file := range reached {
		names = append(names, file.Name)
		if file.Kind == store.DataKept {
			fmt.Fprintln(out.notes, keptWarning)
		}
	}

	listed := strings.Join(names, " and ")

	proceed, err := opts.proceed(out.notes, ask, writePrompt{
		question: "Remove " + listed + " from " + dir + "?",
		dryRun:   "dry run: would remove " + listed + " from " + dir + "; nothing was removed",
		declined: "Nothing removed.",
	})
	if err != nil || !proceed {
		return err
	}

	err = store.Clean(dir, clean.scope)
	if err != nil {
		return fmt.Errorf("removing the local data: %w", err)
	}

	fmt.Fprintln(out.notes, "Removed "+listed+".")

	return nil
}

// reachedBy are the files a clean of scope removes.
func reachedBy(files []store.DataFile, scope store.CleanScope) []store.DataFile {
	var reached []store.DataFile

	for _, file := range files {
		if file.Kind == store.DataCache || scope == store.CleanAll {
			reached = append(reached, file)
		}
	}

	return reached
}

// listDataFiles prints the store directory and each file in it with its kind,
// size and what it holds.
func listDataFiles(out io.Writer, dir string, files []store.DataFile) {
	fmt.Fprintln(out, "Local data in "+dir)

	if len(files) == 0 {
		fmt.Fprintln(out, "  no database files")
	}

	for _, file := range files {
		fmt.Fprintf(out, "  %-12s %-6s %10s  %s\n", file.Name, file.Kind, store.HumanBytes(file.Bytes), holdings(file.Holds))
	}
}

// holdings says what a file holds, or that it could not be read.
func holdings(holds []store.Held) string {
	if len(holds) == 0 {
		return "contents not readable"
	}

	parts := make([]string, 0, len(holds))
	for _, held := range holds {
		parts = append(parts, fmt.Sprintf("%s: %d", held.What, held.Count))
	}

	return strings.Join(parts, ", ")
}
