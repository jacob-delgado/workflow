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
	Branch       func() (gitrepo.Branch, error)
	Naming       convention.BranchNaming
	Confirm      func(question string) (bool, error)
}

// newBranchCmd builds `workflow branch <issue>`.
func newBranchCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "branch <issue>",
		Short: "Branch for an issue, named by the convention, and switch to it",
		Long: "Name a branch for an issue the way the interface does — from the issue's\n" +
			"type and summary — then create it off the current branch's base and switch to\n" +
			"it, which moves the working tree onto the new branch. A preview is printed and\n" +
			"confirmed before anything is created.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBranchCommand(cmd, prompt, args[0], opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runBranchCommand wires the real repository and tracker to the branch flow.
func runBranchCommand(cmd *cobra.Command, prompt Prompt, issueKey string, opts writeOptions) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	seams := branchSeams{
		Issue:        conn.deps.Jira.Issue,
		Branches:     conn.deps.Git.Branches,
		CreateBranch: conn.deps.Git.CreateBranch,
		Branch:       conn.deps.Git.Branch,
		Naming:       conn.cfg.Branch.Naming(),
		Confirm:      func(question string) (bool, error) { return confirm(prompt, question) },
	}

	return runBranch(outputOf(cmd), seams, issueKey, opts)
}

// runBranch names a branch for the issue, previews it, and creates it off the
// current branch's base once confirmed. A branch that already exists is
// refused rather than recreated.
func runBranch(out output, seams branchSeams, issueKey string, opts writeOptions) error {
	detail, err := seams.Issue(jira.Key(issueKey))
	if err != nil {
		return fmt.Errorf("reading issue %s: %w", issueKey, err)
	}

	name := seams.Naming.Name(detail.Issue.Type, string(detail.Issue.Key), detail.Issue.Summary)

	exists, err := branchNamed(seams, name)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("%w: %s (switch to it with git switch %s)", errBranchExists, name, name)
	}

	// Creating the branch also switches to it, carrying the working tree across,
	// so every line before the write says so.
	base := currentBase(seams.Branch)
	fmt.Fprintln(out.artifact, "Branch "+name+" from "+baseLabel(base)+" and switch to it")

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: "Create " + name + " and switch to it?",
		dryRun:   "dry run: would create " + name + " from " + baseLabel(base) + " and switch to it",
		declined: "Not created.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.CreateBranch(name, base)
	if err != nil {
		return fmt.Errorf("creating %s: %w", name, err)
	}

	fmt.Fprintln(out.artifact, "Created "+name)

	return nil
}

// branchNamed reports whether a local branch already goes by name.
func branchNamed(seams branchSeams, name string) (bool, error) {
	names, err := seams.Branches()
	if err != nil {
		return false, fmt.Errorf("listing branches: %w", err)
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
