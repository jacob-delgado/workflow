// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// newRepositoriesCmd builds `workflow repositories`.
func newRepositoriesCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "repositories",
		Short: "List where workflow works: this directory, its repository's worktrees and your favorites",
		Long: "List where workflow works, as the Repositories pane and the web's Repositories\n" +
			"section show it: this directory and its repository, the repository's worktrees,\n" +
			"the main one first, and the directories you keep as favorites, each with what is\n" +
			"there now. --json prints the web API's Repositories shape.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRepositoriesCommand(cmd, asJSON)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print where workflow works as JSON")

	return cmd
}

// runRepositoriesCommand reads where the command runs through the same seams
// the interface and the web read, and prints it.
func runRepositoriesCommand(cmd *cobra.Command, asJSON bool) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	view := webserver.RepositoriesView{
		Repositories:  conn.deps.Repositories,
		Favorites:     conn.deps.Store.Favorites,
		FavoritesKept: conn.deps.Store.Favor != nil && !dryRunRequested(cmd),
	}

	report, readErr := view.Report(func(err error) string { return err.Error() })
	if readErr != nil {
		readErr = fmt.Errorf("reading your favorites: %w", readErr)
	}

	out := outputOf(cmd)
	if asJSON {
		return errors.Join(encodeJSON(out.artifact, report), readErr)
	}

	renderRepositories(out, report)

	return readErr
}

// renderRepositories writes one line for where workflow works, then one per
// worktree and one per favorite, each path written from your home.
func renderRepositories(out output, report api.Repositories) {
	writeRow(out.artifact, "here", report.Here.Shown, report.Here.Origin)

	for _, worktree := range report.Worktrees {
		branch := worktree.Branch
		if branch == "" {
			branch = "(detached)"
		}

		writeRow(out.artifact, "worktree", worktree.Shown, branch, worktree.Head, worktreeNote(worktree))
	}

	if report.WorktreesError != "" {
		fmt.Fprintln(out.notes, "The worktrees could not be read: "+sanitize.Line(report.WorktreesError))
	}

	for _, favorite := range report.Favorites {
		writeRow(out.artifact, "favorite", favorite.Shown, string(favorite.State), favorite.Origin)
	}
}

// worktreeNote is what a worktree's line says beyond its branch: that it is
// the one here, gone from disk, or locked.
func worktreeNote(worktree api.Worktree) string {
	notes := []string{}

	if worktree.State != api.WorktreeOther {
		notes = append(notes, string(worktree.State))
	}

	if worktree.Locked {
		notes = append(notes, "locked")
	}

	return strings.Join(notes, ", ")
}

// writeRow writes a kind of place and its fields, two spaces apart, leaving
// out the empty ones. Each field is read from disk or from git, so it is
// neutralized before it reaches the terminal.
func writeRow(out io.Writer, kind string, fields ...string) {
	row := []string{fmt.Sprintf("%-8s", kind)}

	for _, field := range fields {
		if field != "" {
			row = append(row, sanitize.Line(field))
		}
	}

	fmt.Fprintln(out, strings.Join(row, "  "))
}
