// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// errBranchExists refuses branching for an issue that already has a branch —
// that branch is switched to, not recreated.
var errBranchExists = errors.New("a branch for this issue already exists")

// branchSeams are what `workflow branch` reads and does, so a test can answer
// without a repository or a tracker.
type branchSeams struct {
	Issue        func(jira.Key) (jira.IssueDetail, error)
	Branches     func() ([]string, error)
	CreateBranch func(name, start string) error
	// CreateWorktree creates the branch in a new worktree beside the
	// repository instead, and says where; Fetch refreshes origin's refs first.
	CreateWorktree func(name, start string) (string, error)
	Fetch          func() error
	Branch         func() (gitrepo.Branch, error)
	Naming         convention.BranchNaming
	Confirm        func(question string) (bool, error)
}

// startOptions are how `workflow branch` starts the work: after fetching
// origin, so the branch starts from what origin holds now, and in a new
// worktree rather than the checkout it runs in.
type startOptions struct {
	fetch    bool
	worktree bool
}

// startPlan is the branch a start of work makes: its issue, its name, the
// base it starts from, and how.
type startPlan struct {
	key      string
	name     string
	base     string
	fetch    bool
	worktree bool
}

// newBranchCmd builds `workflow branch <issue>`.
func newBranchCmd(prompt Prompt) *cobra.Command {
	var (
		opts  writeOptions
		start startOptions
	)

	cmd := &cobra.Command{
		Use:   "branch <issue>",
		Short: "Start work on an issue: create its branch, named by the convention, and switch to it",
		Long: "Start work on an issue: name its branch the way the interface does — from\n" +
			"the issue's type and summary — then create it off the current branch's base and\n" +
			"switch to it, which moves the working tree onto the new branch. A preview is\n" +
			"printed and confirmed before anything is created.\n\n" +
			"--fetch fetches origin first, so the branch starts from what origin holds now;\n" +
			"a fetch that fails creates nothing. --worktree creates the branch in a new\n" +
			"worktree beside the repository instead, leaving this checkout where it is, and\n" +
			"prints the worktree's directory alone on the last line.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBranchCommand(cmd, prompt, args[0], opts, start)
		},
	}

	opts.addFlags(cmd, confirmationHelp)
	cmd.Flags().BoolVar(&start.fetch, "fetch", false,
		"fetch origin first, so the branch starts from what origin holds now")
	cmd.Flags().BoolVar(&start.worktree, "worktree", false,
		"create the branch in a new worktree beside the repository, and print its directory")

	return cmd
}

// runBranchCommand wires the real repository and tracker to the branch flow.
func runBranchCommand(cmd *cobra.Command, prompt Prompt, issueKey string, opts writeOptions, start startOptions) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	seams := branchSeams{
		Issue:          conn.deps.Jira.Issue,
		Branches:       conn.deps.Git.Branches,
		CreateBranch:   conn.deps.Git.CreateBranch,
		CreateWorktree: conn.deps.Git.CreateWorktree,
		Fetch:          conn.deps.Git.Fetch,
		Branch:         conn.deps.Git.Branch,
		Naming:         conn.cfg.Branch.Naming(),
		Confirm:        func(question string) (bool, error) { return confirm(prompt, question) },
	}

	return runBranch(outputOf(cmd), seams, issueKey, opts, start)
}

// runBranch names a branch for the issue, previews it, and creates it off the
// current branch's base once confirmed — after fetching origin, or in a new
// worktree, as start asks. A branch that already exists is refused rather than
// recreated.
func runBranch(out output, seams branchSeams, issueKey string, opts writeOptions, start startOptions) error {
	plan, err := planStart(seams, issueKey, start)
	if err != nil {
		return err
	}

	fmt.Fprintln(out.artifact, "Start work on "+plan.key+": "+plan.describe())

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: "Start work on " + plan.key + ": " + plan.question(),
		dryRun:   "dry run: would start work on " + plan.key + ": " + plan.describe(),
		declined: "Not created.",
	})
	if err != nil || !proceed {
		return err
	}

	return plan.carryOut(out, seams)
}

// planStart names the issue's branch and the base it would start from,
// refusing a name a branch already goes by. Only a base can be refreshed, so
// there is nothing to fetch without one.
func planStart(seams branchSeams, issueKey string, start startOptions) (startPlan, error) {
	detail, err := seams.Issue(jira.Key(issueKey))
	if err != nil {
		return startPlan{}, fmt.Errorf("reading issue %s: %w", issueKey, err)
	}

	name := seams.Naming.Name(detail.Issue.Type, string(detail.Issue.Key), detail.Issue.Summary)

	exists, err := branchNamed(seams, name)
	if err != nil {
		return startPlan{}, err
	}

	if exists {
		return startPlan{}, fmt.Errorf("%w: %s; switch to it with `git switch %s`", errBranchExists, name, name)
	}

	base := currentBase(seams.Branch)

	return startPlan{
		key: string(detail.Issue.Key), name: name, base: base,
		fetch: start.fetch && base != "", worktree: start.worktree,
	}, nil
}

// describe says what the start does. Creating a branch here also switches to
// it, carrying the working tree across, so every line before the write says
// so; a worktree leaves the checkout as it is.
func (p startPlan) describe() string {
	made := "create " + p.name + " from " + baseLabel(p.base) + " and switch to it"
	if p.worktree {
		made = "create " + p.name + " from " + baseLabel(p.base) + " in a new worktree beside the repository"
	}

	if p.fetch {
		return "fetch origin, then " + made
	}

	return made
}

// question asks for what describe says, in fewer words.
func (p startPlan) question() string {
	made := "create " + p.name + " and switch to it?"
	if p.worktree {
		made = "create " + p.name + " in a new worktree?"
	}

	if p.fetch {
		return "fetch origin, then " + made
	}

	return made
}

// carryOut fetches when the plan asks, then creates the branch here or in a new
// worktree, whose directory it prints alone so a script can go there. A fetch
// that fails creates nothing, and says how to branch from what is here.
func (p startPlan) carryOut(out output, seams branchSeams) error {
	if p.fetch {
		err := seams.Fetch()
		if err != nil {
			return fmt.Errorf("fetching origin: %w; run without --fetch to branch from what you have", err)
		}
	}

	if !p.worktree {
		err := seams.CreateBranch(p.name, p.base)
		if err != nil {
			return fmt.Errorf("creating %s: %w", p.name, err)
		}

		fmt.Fprintln(out.artifact, "Created "+p.name)

		return nil
	}

	dir, err := seams.CreateWorktree(p.name, p.base)
	if err != nil {
		return fmt.Errorf("creating %s: %w", p.name, err)
	}

	fmt.Fprintln(out.notes, "Created "+p.name+" in a new worktree.")
	fmt.Fprintln(out.artifact, dir)

	return nil
}

// branchNamed reports whether a local branch already goes by name.
func branchNamed(seams branchSeams, name string) (bool, error) {
	names, err := seams.Branches()
	if err != nil {
		return false, err
	}

	return slices.Contains(names, name), nil
}

// currentBase is the base a new branch starts from — the checked-out branch's
// base (origin's default), or empty to branch from HEAD when it cannot be read.
func currentBase(branch func() (gitrepo.Branch, error)) string {
	current, err := branch()
	if err != nil {
		return ""
	}

	return current.Base
}

// baseLabel names the base a branch starts from, or "HEAD" when there is none.
func baseLabel(base string) string {
	if base == "" {
		return "HEAD"
	}

	return base
}
