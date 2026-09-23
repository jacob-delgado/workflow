// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// output is where a command writes: the artifact — the preview, the JSON, what
// it created — on stdout, and what it says about it — a notice, a dry-run line,
// a warning — on stderr, so a script capturing stdout gets data alone.
type output struct {
	artifact io.Writer
	notes    io.Writer
}

// outputOf is the command's stdout and stderr.
func outputOf(cmd *cobra.Command) output {
	return output{artifact: cmd.OutOrStdout(), notes: cmd.ErrOrStderr()}
}

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
// otherwise it asks, and a declined answer says so and stops. What it says is
// commentary, written to notes.
func (o *writeOptions) proceed(notes io.Writer, confirm func(string) (bool, error), say writePrompt) (bool, error) {
	if o.dryRun {
		fmt.Fprintln(notes, say.dryRun)

		return false, nil
	}

	if o.yes {
		return true, nil
	}

	ok, err := confirm(say.question)
	if errors.Is(err, errNoTerminal) {
		return false, fmt.Errorf("%w; pass --yes to go ahead without asking", err)
	}

	if err != nil {
		return false, err
	}

	if !ok {
		fmt.Fprintln(notes, say.declined)
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

// Exit statuses, so a script can tell one kind of failure from another
// without reading the message. docs/content/docs/scripting.md is their
// contract; they mirror the web's problem codes (docs/content/docs/errors.md).
const (
	exitSuccess       = 0
	exitFailure       = 1
	exitUsage         = 2
	exitConfiguration = 3
	exitRefused       = 4
	exitUnreachable   = 5
	// exitInterrupted is 128 plus SIGINT's number, what a shell reports for a
	// program stopped by Ctrl+C.
	exitInterrupted = 130
)

// errUsage marks a mistake in how a command was called — a flag or an argument
// it does not take — as distinct from a command that ran and failed.
var errUsage = errors.New("invalid usage")

// usageError is a mistake in how a command was called, in cobra's own words.
type usageError struct {
	err error
}

var _ error = usageError{}

// Error is cobra's message, unchanged.
func (e usageError) Error() string {
	return e.err.Error()
}

// Unwrap lets errors.Is match both cobra's error and errUsage.
func (e usageError) Unwrap() []error {
	return []error{e.err, errUsage}
}

// asUsage marks err, when there is one, as a mistake in how the command was
// called.
func asUsage(err error) error {
	if err == nil {
		return nil
	}

	return usageError{err: err}
}

// markMisuse makes every command in the tree report its flag and argument
// errors as usage errors: a flag it does not know, a wrong argument count, and
// an unknown command, which is an argument to a command that takes none.
func markMisuse(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return asUsage(err) })
	markArgsMisuse(root)
}

// markArgsMisuse wraps the argument check of cmd and everything under it. A
// command that only groups others, and so has no run of its own, is given one
// that prints its help, as bare `config` does: cobra would otherwise print the
// help for an unknown subcommand and succeed, before its check could refuse it.
func markArgsMisuse(cmd *cobra.Command) {
	if cmd.HasSubCommands() && !cmd.Runnable() {
		cmd.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}

	// A command that declares no check, such as cobra's help, is left to
	// cobra's own.
	if check := cmd.Args; check != nil {
		cmd.Args = func(cmd *cobra.Command, args []string) error { return asUsage(check(cmd, args)) }
	}

	for _, child := range cmd.Commands() {
		markArgsMisuse(child)
	}
}

// ExitStatus is the status the process exits with for err: 0 for none, 130 when
// it was interrupted, and otherwise the first family err belongs to — usage 2,
// configuration 3, a refused precondition 4, unreachable 5 — or 1 for any other
// failure. A joined error takes the first family any of its errors is in, in
// that order.
func ExitStatus(err error) int {
	if err == nil {
		return exitSuccess
	}

	// Checked first: an interrupted request also wraps its service's
	// unreachable error, and whoever pressed Ctrl+C is owed 130, not 5.
	if errors.Is(err, context.Canceled) {
		return exitInterrupted
	}

	for _, family := range exitFamilies() {
		if family.holds(err) {
			return family.status
		}
	}

	return exitFailure
}

// exitFamily is an exit status and the errors that earn it.
type exitFamily struct {
	status  int
	members []error
}

// holds reports whether err is, or wraps, one of the family's errors.
func (f exitFamily) holds(err error) bool {
	for _, member := range f.members {
		if errors.Is(err, member) {
			return true
		}
	}

	return false
}

// exitFamilies are the kinds of failure a script can tell apart, in the order a
// joined error is classified. Built by a function rather than held in a
// package-level variable, which gochecknoglobals forbids.
func exitFamilies() []exitFamily {
	return []exitFamily{
		{status: exitUsage, members: []error{errUsage, errNoTerminal}},
		{status: exitConfiguration, members: configurationErrors()},
		{status: exitRefused, members: refusalErrors()},
		{status: exitUnreachable, members: unreachableErrors()},
	}
}

// configurationErrors are a configuration that is missing, unreadable,
// incomplete — a credential or an address not set, or set unusably — shared,
// or holding a credential a service would not accept: what the user fixes in
// the file. A command meeting a credential that is not there exits as doctor
// does, reading the same file.
func configurationErrors() []error {
	return []error{
		config.ErrNotFound, config.ErrInvalid,
		jira.ErrNoCredential, jira.ErrInvalidBaseURL, jira.ErrCredentialInBaseURL,
		forge.ErrNoToken, forge.ErrKindNeedsHost, messaging.ErrNoCredential, messaging.ErrInsecureWebhook,
		jira.ErrUnauthorized, jira.ErrForbidden, forge.ErrUnauthorized, messaging.ErrRejected,
		errCredentialRejected, errIncomplete, errInvalid, errShared,
		errMessagingNotConfigured,
	}
}

// refusalErrors are a command that would not go ahead because of the state it
// found: an open pull request, nothing to open, a dirty tree, a branch or file
// already there, a directory that is no repository.
func refusalErrors() []error {
	return []error{
		loop.ErrPullAlreadyOpen, loop.ErrNothingToOpen, loop.ErrNoPullRequest,
		loop.ErrDirtyTree, loop.ErrNothingStaged,
		errPullAlreadyOpen, errNoCommitsToOpen, errNoPullRequest,
		errBranchExists, errConfigExists, gitrepo.ErrNotARepository,
	}
}

// unreachableErrors are a service that never answered, or answered only to say
// wait: a script may try again later.
func unreachableErrors() []error {
	return []error{
		jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable,
		errUnreachable, httpx.ErrRateLimited,
	}
}
