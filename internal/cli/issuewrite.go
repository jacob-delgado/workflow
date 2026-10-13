// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// issueWrite is a change to an issue a command makes: the preview it prints,
// what it says around the confirmation, the write itself, and what it says
// once the write is made.
type issueWrite struct {
	preview string
	say     writePrompt
	write   func(ctx context.Context) error
	done    string
}

// run previews the write, and makes it once confirmed, saying so.
func (w issueWrite) run(ctx context.Context, out output, opts writeOptions, confirm func(string) (bool, error)) error {
	fmt.Fprintln(out.artifact, w.preview)

	proceed, err := opts.proceed(out.notes, confirm, w.say)
	if err != nil || !proceed {
		return err
	}

	err = w.write(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(out.notes, w.done)

	return nil
}

// transitionSeams are what `workflow transition` reads and does, so a test can
// answer without a tracker.
type transitionSeams struct {
	Transitions func(ctx context.Context, issueKey jira.Key) ([]jira.Transition, error)
	Transition  func(ctx context.Context, issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	Confirm     func(question string) (bool, error)
}

// newTransitionCmd builds `workflow transition <issue> <status>`.
func newTransitionCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "transition <issue> <status>",
		Short: "Move an issue to a status, in Jira or on the forge",
		Long: "Move an issue to the status named, matched whatever its case, by the move the\n" +
			"tracker offers from where the issue stands. The move is printed and confirmed\n" +
			"before it is made. A status the tracker does not offer is refused, naming those\n" +
			"it does, and so is a move that asks for fields, naming them: change its status\n" +
			"with t in the interface or Change status on the web, whose forms ask for them.\n" +
			"Either refusal exits 4.",
		Example: examples(
			`workflow transition PROJ-7 "In Review" --yes   # unattended`,
		),
		Args: cobra.ExactArgs(2), //nolint:mnd // the issue and the status
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connect(cmd)
			if err != nil {
				return err
			}
			defer conn.closeLog()

			seams := transitionSeams{
				Transitions: conn.deps.Jira.Transitions,
				Transition:  conn.deps.Jira.Transition,
				Confirm:     func(question string) (bool, error) { return confirm(prompt, question) },
			}

			return runTransition(cmd.Context(), outputOf(cmd), seams, jira.Key(args[0]), args[1], opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runTransition finds the move to status, previews it, and makes it once
// confirmed. A move that asks for fields says where a form asks for them.
func runTransition(
	ctx context.Context, out output, seams transitionSeams, issueKey jira.Key, status string, opts writeOptions,
) error {
	move, err := loop.TransitionTo(ctx, seams.Transitions, issueKey, status)
	if errors.Is(err, loop.ErrNeedsFields) {
		return fmt.Errorf("%w; change its status with t in the interface or Change status on the web, "+
			"whose forms ask for them", err)
	}

	if err != nil {
		return err
	}

	key, target := string(issueKey), move.ToStatus

	return issueWrite{
		preview: "Move " + key + " to " + target + " (" + move.Name + ")",
		say: writePrompt{
			question: "Move " + key + " to " + target + "?",
			dryRun:   "dry run: would move " + key + " to " + target,
			declined: "Left " + key + " as it is.",
		},
		write: func(ctx context.Context) error {
			err := seams.Transition(ctx, issueKey, move, nil)
			if err != nil {
				return fmt.Errorf("moving %s to %s: %w", key, target, err)
			}

			return nil
		},
		done: "Moved " + key + " to " + target + ".",
	}.run(ctx, out, opts, seams.Confirm)
}

// errAssigneeBlank refuses an assignment that names no one.
var errAssigneeBlank = errors.New("an assignee needs a username")

// assignSeams are what `workflow assign` does, so a test can answer without a
// tracker.
type assignSeams struct {
	Assign  func(ctx context.Context, issueKey jira.Key, assignee string) error
	Confirm func(question string) (bool, error)
}

// newAssignCmd builds `workflow assign <issue> <username>`.
func newAssignCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "assign <issue> <username>",
		Short: "Assign an issue to someone, in Jira or on the forge",
		Long: "Assign an issue to the person a username names: their Jira username for a\n" +
			"Jira issue, their forge login for a forge issue. The assignment is printed and\n" +
			"confirmed before it is made. A blank username is refused before anything is\n" +
			"asked.",
		Example: examples(
			`workflow assign PROJ-7 bo --yes   # unattended`,
		),
		Args: cobra.ExactArgs(2), //nolint:mnd // the issue and the username
		RunE: func(cmd *cobra.Command, args []string) error {
			assignee := strings.TrimSpace(args[1])
			if assignee == "" {
				return errAssigneeBlank
			}

			conn, err := connect(cmd)
			if err != nil {
				return err
			}
			defer conn.closeLog()

			seams := assignSeams{
				Assign:  conn.deps.Jira.Assign,
				Confirm: func(question string) (bool, error) { return confirm(prompt, question) },
			}

			return runAssign(cmd.Context(), outputOf(cmd), seams, jira.Key(args[0]), assignee, opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runAssign previews the assignment, and makes it once confirmed.
func runAssign(
	ctx context.Context, out output, seams assignSeams, issueKey jira.Key, assignee string, opts writeOptions,
) error {
	key, who := string(issueKey), assignee

	return issueWrite{
		preview: "Assign " + key + " to " + who,
		say: writePrompt{
			question: "Assign " + key + " to " + who + "?",
			dryRun:   "dry run: would assign " + key + " to " + who,
			declined: "Not assigned.",
		},
		write: func(ctx context.Context) error {
			err := seams.Assign(ctx, issueKey, assignee)
			if err != nil {
				return fmt.Errorf("assigning %s to %s: %w", key, who, err)
			}

			return nil
		},
		done: "Assigned " + key + " to " + who + ".",
	}.run(ctx, out, opts, seams.Confirm)
}
