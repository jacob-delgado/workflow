// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/report"
)

// issueWrite is a change to an issue a command makes: the preview it prints,
// what it says around the confirmation, and the write itself, which answers
// what to say once it is made.
type issueWrite struct {
	preview string
	say     cmdkit.WritePrompt
	write   func(ctx context.Context) (string, error)
}

// run previews the write, and makes it once confirmed, saying so.
func (w issueWrite) run(
	ctx context.Context, out cmdkit.Output, opts cmdkit.WriteOptions, confirm func(string) (bool, error),
) error {
	fmt.Fprintln(out.Artifact, w.preview)

	proceed, err := opts.Proceed(out.Notes, confirm, w.say)
	if err != nil || !proceed {
		return err
	}

	done, err := w.write(ctx)
	if err != nil {
		return err
	}

	out.Notes.Note(done)

	return nil
}

// errStatusBlank refuses a move that names no status to move to.
var errStatusBlank = errors.New("a transition needs a status to move to")

// transitionSeams are what `workflow transition` reads and does, so a test can
// answer without a tracker.
type transitionSeams struct {
	Transitions func(ctx context.Context, issueKey jira.Key) ([]jira.Transition, error)
	Transition  func(ctx context.Context, issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	Confirm     func(question string) (bool, error)
}

// newTransitionCmd builds `workflow transition <issue> <status>`.
func newTransitionCmd(prompt cmdkit.Prompt) *cobra.Command {
	var opts cmdkit.WriteOptions

	cmd := &cobra.Command{
		Use:   "transition <issue> <status>",
		Short: "Move an issue to a status, in Jira or on the forge",
		Long: "Move an issue to the status named, matched whatever its case, by the move the\n" +
			"tracker offers from where the issue stands. The move is printed and confirmed\n" +
			"before it is made. A status the tracker does not offer is refused, naming those\n" +
			"it does, and so is a move that asks for fields, naming them: change its status\n" +
			"with t in the interface or Change status on the web, whose forms ask for them.\n" +
			"Either refusal exits 4. A blank status is refused before anything is asked.",
		Example: cmdkit.Examples(
			`workflow transition PROJ-7 "In Review" --yes   # unattended`,
		),
		Args:              cobra.ExactArgs(2), //nolint:mnd // the issue and the status
		ValidArgsFunction: cmdkit.CompleteAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			status := strings.TrimSpace(args[1])
			if status == "" {
				return errStatusBlank
			}

			conn, err := cmdkit.Connect(cmd)
			if err != nil {
				return err
			}
			defer conn.CloseLog()

			seams := transitionSeams{
				Transitions: conn.Deps.Jira.Transitions,
				Transition:  conn.Deps.Jira.Transition,
				Confirm:     func(question string) (bool, error) { return cmdkit.Confirm(prompt, question) },
			}

			return runTransition(cmd.Context(), cmdkit.OutputOf(cmd), seams, jira.Key(args[0]), status, opts)
		},
	}

	opts.AddFlags(cmd, cmdkit.ConfirmationHelp)

	return cmd
}

// runTransition finds the move to status, previews it, and makes it once
// confirmed. A move that asks for fields says where a form asks for them.
func runTransition(
	ctx context.Context, out cmdkit.Output, seams transitionSeams, issueKey jira.Key, status string,
	opts cmdkit.WriteOptions,
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
		say: cmdkit.WritePrompt{
			Question: "Move " + key + " to " + target + "?",
			DryRun:   "dry run: would move " + key + " to " + target,
			Declined: "Left " + key + " as it is.",
		},
		write: func(ctx context.Context) (string, error) {
			err := seams.Transition(ctx, issueKey, move, nil)
			if err != nil {
				return "", fmt.Errorf("moving %s to %s: %w", key, target, err)
			}

			return "Moved " + key + " to " + target + ".", nil
		},
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
func newAssignCmd(prompt cmdkit.Prompt) *cobra.Command {
	var opts cmdkit.WriteOptions

	cmd := &cobra.Command{
		Use:   "assign <issue> <username>",
		Short: "Assign an issue to someone, in Jira or on the forge",
		Long: "Assign an issue to the person a username names: their Jira username for a\n" +
			"Jira issue, their forge login for a forge issue. The assignment is printed and\n" +
			"confirmed before it is made. A blank username is refused before anything is\n" +
			"asked.",
		Example: cmdkit.Examples(
			`workflow assign PROJ-7 bo --yes   # unattended`,
		),
		Args:              cobra.ExactArgs(2), //nolint:mnd // the issue and the username
		ValidArgsFunction: cmdkit.CompleteAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			assignee := strings.TrimSpace(args[1])
			if assignee == "" {
				return errAssigneeBlank
			}

			conn, err := cmdkit.Connect(cmd)
			if err != nil {
				return err
			}
			defer conn.CloseLog()

			seams := assignSeams{
				Assign:  conn.Deps.Jira.Assign,
				Confirm: func(question string) (bool, error) { return cmdkit.Confirm(prompt, question) },
			}

			return runAssign(cmd.Context(), cmdkit.OutputOf(cmd), seams, jira.Key(args[0]), assignee, opts)
		},
	}

	opts.AddFlags(cmd, cmdkit.ConfirmationHelp)

	return cmd
}

// runAssign previews the assignment, and makes it once confirmed.
func runAssign(
	ctx context.Context, out cmdkit.Output, seams assignSeams, issueKey jira.Key, assignee string,
	opts cmdkit.WriteOptions,
) error {
	key, who := string(issueKey), assignee

	return issueWrite{
		preview: "Assign " + key + " to " + who,
		say: cmdkit.WritePrompt{
			Question: "Assign " + key + " to " + who + "?",
			DryRun:   "dry run: would assign " + key + " to " + who,
			Declined: "Not assigned.",
		},
		write: func(ctx context.Context) (string, error) {
			err := seams.Assign(ctx, issueKey, assignee)
			if err != nil {
				return "", fmt.Errorf("assigning %s to %s: %w", key, who, err)
			}

			return "Assigned " + key + " to " + who + ".", nil
		},
	}.run(ctx, out, opts, seams.Confirm)
}

// Why work cannot be logged.
var (
	// errDurationBlank refuses a worklog that says nothing of how long.
	errDurationBlank = errors.New("logging work needs a duration, such as 2h or 30m")
	// errWorklogOnForge refuses a worklog on a forge issue, which keeps none.
	errWorklogOnForge = errors.New("a forge issue keeps no worklog; log work on a Jira issue")
	// errWorklogNeedsJira refuses a worklog where the forge's issues are the
	// whole tracker.
	errWorklogNeedsJira = errors.New("logging work needs Jira; set jira.base_url")
)

// worklogSeams are what `workflow worklog` does, so a test can answer without
// a tracker.
type worklogSeams struct {
	AddWorklog func(ctx context.Context, issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error)
	Confirm    func(question string) (bool, error)
}

// worklogEntry is the work a worklog records: on which issue, how long, and
// the note beside it.
type worklogEntry struct {
	issueKey jira.Key
	spent    string
	note     string
}

// newWorklogCmd builds `workflow worklog <issue> <duration>`.
func newWorklogCmd(prompt cmdkit.Prompt) *cobra.Command {
	var (
		opts cmdkit.WriteOptions
		note string
	)

	cmd := &cobra.Command{
		Use:   "worklog <issue> <duration>",
		Short: "Log work on a Jira issue",
		Long: "Log work on a Jira issue: how long, in Jira's own form (2h, 30m, 1d 4h), with\n" +
			"--comment's note beside it. The worklog is printed and confirmed before it is\n" +
			"logged. A forge issue keeps no worklog, so its number is refused, as is a blank\n" +
			"duration, before anything is asked.",
		Example: cmdkit.Examples(
			`workflow worklog PROJ-7 2h --comment "pairing on the fix" --yes`,
		),
		Args:              cobra.ExactArgs(2), //nolint:mnd // the issue and the duration
		ValidArgsFunction: cmdkit.CompleteAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			entry := worklogEntry{issueKey: jira.Key(args[0]), spent: strings.TrimSpace(args[1]), note: note}

			err := entry.check()
			if err != nil {
				return err
			}

			conn, err := cmdkit.Connect(cmd)
			if err != nil {
				return err
			}
			defer conn.CloseLog()

			seams := worklogSeams{
				AddWorklog: conn.Deps.Jira.AddWorklog,
				Confirm:    func(question string) (bool, error) { return cmdkit.Confirm(prompt, question) },
			}

			return runWorklog(cmd.Context(), cmdkit.OutputOf(cmd), seams, entry, opts)
		},
	}

	opts.AddFlags(cmd, cmdkit.ConfirmationHelp)
	cmd.Flags().StringVar(&note, "comment", "", "a note to log beside the time")

	return cmd
}

// check refuses, from the arguments alone, a worklog no tracker could take: a
// forge issue's, or one with no duration.
func (e worklogEntry) check() error {
	if report.TrackerOf(e.issueKey) == api.IssueTrackerForge {
		return errWorklogOnForge
	}

	if e.spent == "" {
		return errDurationBlank
	}

	return nil
}

// runWorklog previews the worklog, and logs it once confirmed.
func runWorklog(
	ctx context.Context, out cmdkit.Output, seams worklogSeams, entry worklogEntry, opts cmdkit.WriteOptions,
) error {
	if seams.AddWorklog == nil {
		return errWorklogNeedsJira
	}

	key := string(entry.issueKey)

	preview := "Log " + entry.spent + " on " + key
	if entry.note != "" {
		preview += "\n  " + entry.note
	}

	return issueWrite{
		preview: preview,
		say: cmdkit.WritePrompt{
			Question: "Log " + entry.spent + " on " + key + "?",
			DryRun:   "dry run: would log " + entry.spent + " on " + key,
			Declined: "Not logged.",
		},
		write: func(ctx context.Context) (string, error) {
			logged, err := seams.AddWorklog(ctx, entry.issueKey, entry.spent, entry.note)
			if err != nil {
				return "", fmt.Errorf("logging %s on %s: %w", entry.spent, key, err)
			}

			return "Logged " + cmp.Or(logged.TimeSpent, entry.spent) + " on " + key + ".", nil
		},
	}.run(ctx, out, opts, seams.Confirm)
}
