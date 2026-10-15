// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// finishSeams are what `workflow finish` reads and does, so a test can answer
// without a repository or a forge.
type finishSeams struct {
	Branch   func(ctx context.Context) (gitrepo.Branch, error)
	FindPull func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	Finish   func(ctx context.Context, finishing gitrepo.Finishing) error
	// Kind is the forge, whose own words — a merge request, marked "!" on
	// GitLab — the preview and a failed read use.
	Kind    forge.Kind
	Confirm func(question string) (bool, error)
}

// finishPlan is a finish that can go: the merged branch it deletes, the base
// it returns to and the remote it pulls that base from, and the pull request
// that merged it.
type finishPlan struct {
	finishing gitrepo.Finishing
	pull      forge.PullRequest
}

// newFinishCmd builds `workflow finish`.
func newFinishCmd(prompt cmdkit.Prompt) *cobra.Command {
	var opts cmdkit.WriteOptions

	cmd := &cobra.Command{
		Use:   "finish",
		Short: "Finish a merged branch: switch to its base, catch the base up, and delete the branch",
		Long: "Finish the checked-out branch once its pull or merge request has merged: switch\n" +
			"to its base, fast-forward the base from the base remote — upstream in a fork's\n" +
			"clone, else origin — with git pull --ff-only, and delete the branch: the three\n" +
			"git commands the interface's F runs. They are printed and confirmed before any\n" +
			"runs, and a step git refuses stops the rest, keeping the branch.\n\n" +
			"A branch that cannot be finished is refused with the reason, and exits 4: its\n" +
			"pull or merge request has not merged, it is the base itself or a detached\n" +
			"HEAD, there is no base to return to, or it holds commits that neither merged\n" +
			"nor are on its upstream, which deleting it would lose. The delete is a force\n" +
			"delete, so it goes ahead only for a branch at the very commit that merged, or\n" +
			"one whose upstream holds every commit on it.",
		Example: cmdkit.Examples(
			"workflow finish --dry-run   # the three git commands, none run",
			"workflow finish --yes       # finish a merged branch, unattended",
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFinishCommand(cmd, prompt, opts)
		},
	}

	opts.AddFlags(cmd, cmdkit.ConfirmationHelp)

	return cmd
}

// runFinishCommand wires the real repository and forge to the finish.
func runFinishCommand(cmd *cobra.Command, prompt cmdkit.Prompt, opts cmdkit.WriteOptions) error {
	conn, err := cmdkit.Connect(cmd)
	if err != nil {
		return err
	}
	defer conn.CloseLog()

	seams := finishSeams{
		Branch:   conn.Deps.Git.Branch,
		FindPull: conn.Deps.Forge.FindPullRequest,
		Finish:   conn.Deps.Git.Finish,
		Kind:     conn.Deps.Forge.Kind,
		Confirm:  func(question string) (bool, error) { return cmdkit.Confirm(prompt, question) },
	}

	return runFinish(cmd.Context(), cmdkit.OutputOf(cmd), seams, opts)
}

// runFinish previews the three git commands that finish the merged branch and
// runs them once confirmed. A branch that cannot be finished is refused with
// the reason before anything is asked.
func runFinish(ctx context.Context, out cmdkit.Output, seams finishSeams, opts cmdkit.WriteOptions) error {
	plan, err := planFinish(ctx, seams)
	if err != nil {
		return err
	}

	plan.preview(out.Artifact, seams.Kind)

	finishing := plan.finishing
	steps := "switch to " + finishing.Base + ", pull it from " + finishing.Remote

	proceed, err := opts.Proceed(out.Notes, seams.Confirm, cmdkit.WritePrompt{
		Question: "Finish " + finishing.Branch + ": " + steps + ", and delete it?",
		DryRun:   "dry run: would finish " + finishing.Branch + " (" + steps + ", delete it)",
		Declined: "Not finished.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.Finish(ctx, finishing)
	if err != nil {
		return fmt.Errorf("finishing %s: %w", finishing.Branch, err)
	}

	out.Notes.Note("Finished " + finishing.Branch + ": switched to " + finishing.Base + ", caught it up and deleted " +
		finishing.Branch + ".")

	return nil
}

// planFinish reads the branch and, for a branch of its own, its pull request,
// and refuses a finish loop.RefuseFinish finds something in the way of. The
// base or a detached HEAD is refused before the forge is asked anything.
func planFinish(ctx context.Context, seams finishSeams) (finishPlan, error) {
	branch, err := seams.Branch(ctx)
	if err != nil {
		return finishPlan{}, fmt.Errorf("reading the branch: %w", err)
	}

	var (
		pull  forge.PullRequest
		found bool
	)

	if loop.OnFeatureBranch(branch) {
		pull, found, err = seams.FindPull(ctx, branch.Name)
		if err != nil {
			return finishPlan{}, fmt.Errorf("reading the %s of %s: %w", seams.Kind.Noun(), branch.Name, err)
		}
	}

	refused := loop.RefuseFinish(pull, found, branch)
	if refused != nil {
		return finishPlan{}, fmt.Errorf("cannot finish %s: %w", cmp.Or(branch.Name, "a detached HEAD"), refused)
	}

	return finishPlan{finishing: branch.Finishing(), pull: pull}, nil
}

// preview prints the merged pull request, in the forge's own mark, and the
// three git commands that finish its branch, as gitrepo runs them.
func (p finishPlan) preview(artifact io.Writer, kind forge.Kind) {
	fmt.Fprintln(artifact, "Finish "+p.finishing.Branch+": "+kind.Sigil()+strconv.Itoa(p.pull.Number)+" merged")

	for _, command := range p.finishing.Commands() {
		fmt.Fprintln(artifact, "  "+command)
	}
}
