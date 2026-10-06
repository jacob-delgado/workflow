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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope := map[bool]store.CleanScope{false: store.CleanCache, true: store.CleanAll}[all]
			ask := func(question string) (bool, error) { return confirm(prompt, question) }

			return runDBClean(cmd.Context(), outputOf(cmd), ask, scope, opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)
	cmd.Flags().BoolVar(&all, "all", false,
		"also remove kept.db, so people and group associations are asked again")

	return cmd
}

// runDBClean lists the store's files, then removes those scope reaches once
// confirmed.
func runDBClean(
	ctx context.Context, out output, ask func(string) (bool, error), scope store.CleanScope, opts writeOptions,
) error {
	dir, files, err := localData(ctx)
	if err != nil {
		return err
	}

	listDataFiles(out.artifact, dir, files)

	reached := reachedBy(files, scope)
	if len(reached) == 0 {
		fmt.Fprintln(out.notes, "Nothing to remove.")

		return nil
	}

	return removeDataFiles(out, ask, dir, scope, reached, opts)
}

// removeDataFiles removes the files a clean of scope reaches once confirmed,
// warning first when the kept file is among them.
func removeDataFiles(
	out output, ask func(string) (bool, error), dir string, scope store.CleanScope, reached []store.DataFile,
	opts writeOptions,
) error {
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

	err = cleanLocalData(scope)
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
		fmt.Fprintf(out, "  %-12s %-6s %10s  %s\n", file.Name, file.Kind, humanBytes(file.Bytes), holdings(file.Holds))
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

// humanBytes is a size in bytes, KiB or MiB, with one decimal past bytes.
func humanBytes(size int64) string {
	const unit = 1024

	switch {
	case size < unit:
		return fmt.Sprintf("%d B", size)
	case size < unit*unit:
		return fmt.Sprintf("%.1f KiB", float64(size)/unit)
	default:
		return fmt.Sprintf("%.1f MiB", float64(size)/(unit*unit))
	}
}

// localData is the store's directory and the database files in it, as
// db-clean and the web's Local data area list them.
func localData(ctx context.Context) (string, []store.DataFile, error) {
	dir, err := store.DefaultDir()
	if err != nil {
		return "", nil, fmt.Errorf("finding the local data: %w", err)
	}

	files, err := store.Files(ctx, dir)
	if err != nil {
		return "", nil, fmt.Errorf("reading the local data: %w", err)
	}

	return dir, files, nil
}

// cleanLocalData removes the store's files a clean of scope reaches.
func cleanLocalData(scope store.CleanScope) error {
	dir, err := store.DefaultDir()
	if err != nil {
		return fmt.Errorf("finding the local data: %w", err)
	}

	return store.Clean(dir, scope)
}
