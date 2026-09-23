// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// errNoCommitsToOpen refuses opening a pull request with nothing to propose.
var errNoCommitsToOpen = errors.New("no branch with commits to open a pull request for")

// errPullAlreadyOpen refuses opening a second pull request for a branch that
// already has an open one.
var errPullAlreadyOpen = errors.New("an open pull request already exists for this branch")

// errPushFailed reports a push that could not publish the branch.
var errPushFailed = errors.New("the branch could not be pushed")

// prSeams are what `workflow pr` reads and does, so a test can answer without a
// repository or a forge.
type prSeams struct {
	// Compose and Options are what the pull request is composed from, and under.
	Compose     loop.PullSeams
	Options     loop.PullOptions
	Push        func(branch string) (proc.Output, error)
	CreatePull  func(forge.NewPullRequest) (forge.PullRequest, error)
	Transitions func(jira.Key) ([]jira.Transition, error)
	Transition  func(jira.Key, jira.Transition, []jira.FieldValue) error
	// ReviewStatus is the status an issue moves to once its pull request is open,
	// offered after opening. Empty makes no offer.
	ReviewStatus string
	Confirm      func(question string) (bool, error)
}

// newPRCmd builds `workflow pr`.
func newPRCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Open a pull request for the current branch",
		Long: "Compose a pull request for the checked-out branch from its commits, the\n" +
			"issue and the repository's template — the same as the interface — pushing the\n" +
			"branch first when it is not yet on its remote. A preview is confirmed first.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPRCommand(cmd, prompt, opts)
		},
	}

	opts.addFlags(cmd, "opening")

	return cmd
}

// runPRCommand wires the real repository and forge to the pull-request flow.
func runPRCommand(cmd *cobra.Command, prompt Prompt, opts writeOptions) error {
	ctx := cmd.Context()

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining the working directory: %w", err)
	}

	home, _ := os.UserHomeDir()
	cfg, _ := config.Load(dir, home)
	deps := wiring.Deps(ctx, cfg, wiring.Locate(ctx, dir), nil)

	seams := prSeams{
		Compose: loop.PullSeams{
			Branch:    deps.Git.Branch,
			FindPull:  deps.Forge.FindPullRequest,
			Templates: deps.Forge.Templates,
			Issue:     deps.Jira.Issue,
			BrowseURL: deps.Jira.BrowseURL,
		},
		Options: loop.PullOptions{
			Project:     cfg.Jira.Project,
			TitleSource: convention.TitleSource(cfg.PullRequest.TitleSource),
		},
		Push:         deps.Git.Push,
		CreatePull:   deps.Forge.CreatePullRequest,
		Transitions:  deps.Jira.Transitions,
		Transition:   deps.Jira.Transition,
		ReviewStatus: cfg.Jira.ReviewStatus,
		Confirm:      func(question string) (bool, error) { return confirm(prompt, question) },
	}

	return runPR(cmd.OutOrStdout(), seams, opts)
}

// runPR composes the pull request, previews it, pushes the branch when needed,
// and opens it once confirmed.
func runPR(out io.Writer, seams prSeams, opts writeOptions) error {
	request, branch, err := loop.ComposePull(seams.Compose, seams.Options)
	if err != nil {
		return composeRefusal(err)
	}

	fmt.Fprintln(out, "Open "+request.Title)
	fmt.Fprintln(out, "  "+branch.Name+" → "+request.Base)

	proceed, err := opts.proceed(out, seams.Confirm, writePrompt{
		question: "Open the pull request?",
		dryRun:   "dry run: would " + pushClause(branch) + "open " + request.Title,
		declined: "Not opened.",
	})
	if err != nil || !proceed {
		return err
	}

	err = loop.EnsurePushed(seams.Push, branch)
	if err != nil {
		return pushFailure(branch.Name, err)
	}

	pull, err := seams.CreatePull(request)
	if err != nil {
		return fmt.Errorf("opening the pull request: %w", err)
	}

	fmt.Fprintln(out, "Opened #"+strconv.Itoa(pull.Number)+" "+pull.URL)

	key, _ := convention.IssueKey(branch.Name, seams.Options.Project)

	return offerReviewStatus(out, seams, jira.Key(key), opts)
}

// composeRefusal words a refusal to compose in the command line's own terms.
func composeRefusal(err error) error {
	switch {
	case errors.Is(err, loop.ErrNothingToOpen):
		return errNoCommitsToOpen
	case errors.Is(err, loop.ErrPullAlreadyOpen):
		return errPullAlreadyOpen
	default:
		return err
	}
}

// pushFailure words a push that did not publish the branch: with the push's own
// output when it ran and failed, and with why it could not start otherwise.
func pushFailure(name string, err error) error {
	if failed, ok := errors.AsType[loop.PushFailedError](err); ok {
		return fmt.Errorf("%w:\n%s", errPushFailed, strings.Join(failed.Output, "\n"))
	}

	return fmt.Errorf("pushing %s: %w", name, err)
}

// offerReviewStatus offers to move the branch's issue to the configured review
// status once the pull request is open, chosen by name because it shares a
// category with "in progress". A read that fails, a status Jira does not offer,
// or one whose transition needs fields this command cannot fill, is passed over
// quietly — the pull request is already open.
func offerReviewStatus(out io.Writer, seams prSeams, issueKey jira.Key, opts writeOptions) error {
	target, ok := reviewTarget(seams, issueKey)
	if !ok {
		return nil
	}

	proceed, err := opts.proceed(out, seams.Confirm, writePrompt{
		question: "Move " + string(issueKey) + " to " + target.ToStatus + "?",
		dryRun:   "dry run: would move " + string(issueKey) + " to " + target.ToStatus,
		declined: "Left " + string(issueKey) + " as it is.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.Transition(issueKey, target, nil)
	if err != nil {
		return fmt.Errorf("moving %s to %s: %w", issueKey, target.ToStatus, err)
	}

	fmt.Fprintln(out, "Moved "+string(issueKey)+" to "+target.ToStatus)

	return nil
}

// reviewTarget is the transition to the configured review status and whether the
// command should offer it: the status must be configured, the seams present, the
// transition offered by Jira, and fillable without a form this command cannot
// show. A tracker read that fails is passed over — the pull request is open.
func reviewTarget(seams prSeams, issueKey jira.Key) (jira.Transition, bool) {
	if seams.ReviewStatus == "" || issueKey == "" || seams.Transitions == nil || seams.Transition == nil {
		return jira.Transition{}, false
	}

	moves, err := seams.Transitions(issueKey)
	if err != nil {
		return jira.Transition{}, false
	}

	target, found := transitionTo(moves, seams.ReviewStatus)
	if !found || len(target.Fields) > 0 {
		return jira.Transition{}, false
	}

	return target, true
}

// transitionTo is the transition leading to a status of the given name, matched
// case-insensitively.
func transitionTo(moves []jira.Transition, status string) (jira.Transition, bool) {
	for _, move := range moves {
		if strings.EqualFold(move.ToStatus, status) {
			return move, true
		}
	}

	return jira.Transition{}, false
}

// pushClause names the push a not-yet-pushed branch needs first, for the dry-run
// line, or nothing when the branch is already up.
func pushClause(branch gitrepo.Branch) string {
	if branch.Pushed() {
		return ""
	}

	return "push " + branch.Name + " and "
}
