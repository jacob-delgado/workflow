// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// writeOptions are the flags every scriptable write shares: a dry run that
// changes nothing, and a yes that skips the confirmation for unattended use.
type writeOptions struct {
	dryRun bool
	yes    bool
}

// addFlags declares --dry-run and --yes on a scriptable write command. verb
// names the action in the flag help ("creating", "opening", "posting").
func (o *writeOptions) addFlags(cmd *cobra.Command, verb string) {
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "preview without "+verb+" anything")
	cmd.Flags().BoolVar(&o.yes, "yes", false, "go ahead without the confirmation")
}

// writePrompt is what a scriptable write says around its confirmation: the
// question it asks, what a dry run would do, and what it prints when declined.
type writePrompt struct {
	question string
	dryRun   string
	declined string
}

// proceed reports whether to go ahead with a write, the preview already printed.
// A dry run says what it would do and stops; --yes goes ahead without asking;
// otherwise it asks, and a declined answer says so and stops.
func (o *writeOptions) proceed(out io.Writer, confirm func(string) (bool, error), say writePrompt) (bool, error) {
	if o.dryRun {
		fmt.Fprintln(out, say.dryRun)

		return false, nil
	}

	if o.yes {
		return true, nil
	}

	ok, err := confirm(say.question)
	if err != nil {
		return false, err
	}

	if !ok {
		fmt.Fprintln(out, say.declined)
	}

	return ok, nil
}

// encodeJSON writes value as indented JSON: the one encoder every command's
// data goes out through, so each one's output is shaped and worded the same.
func encodeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("encoding the output: %w", err)
	}

	return nil
}

// completeAssignedIssues completes an issue argument with the keys of the issues
// assigned to you, for `workflow branch <tab>`. A tracker that cannot be reached
// offers nothing rather than an error.
func completeAssignedIssues(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Completion runs on every <tab>, so it records nothing in a request log.
	home, _ := os.UserHomeDir()
	conn := connectAt(cmd.Context(), dir, home, nil)

	result, err := conn.deps.Jira.Search(jira.AssignedToMe, 0)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var keys []string

	for _, issue := range result.Issues {
		if key := string(issue.Key); strings.HasPrefix(key, toComplete) {
			keys = append(keys, key)
		}
	}

	return keys, cobra.ShellCompDirectiveNoFileComp
}
