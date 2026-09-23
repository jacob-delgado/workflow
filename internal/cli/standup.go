// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// standupSeams are what `workflow standup` reads and does, so a test can answer
// without a repository, a network, or an editor.
type standupSeams struct {
	Commits  func(since string) ([]gitrepo.Commit, error)
	Branches func() ([]string, error)
	FindPull func(branch string) (forge.PullRequest, bool, error)
	Search   func(jql string, startAt int) (jira.SearchResult, error)
	Compose  func(draft string) (string, error)
	Post     func(text string) error
	Confirm  func(question string) (bool, error)
	// Service names the messaging service in use, for the confirmation and result.
	Service string
	// Configured is true when a messaging transport is set up, so posting is
	// offered.
	Configured bool
}

// standupBranchLimit bounds how many recent local branches are asked about, so a
// repository with a long history of branches does not fire a forge request for
// each one.
const standupBranchLimit = 15

// standupOptions are standup's flags: how far back to gather, whether to skip
// the editor, and the dry run and --yes every scriptable write shares.
type standupOptions struct {
	days   int
	noEdit bool
	write  writeOptions
}

// newStandupCmd builds `workflow standup`.
func newStandupCmd(prompt Prompt) *cobra.Command {
	var opts standupOptions

	cmd := &cobra.Command{
		Use:   "standup",
		Short: "Draft what you did — commits, issues and pull requests — to share",
		Long: "Gather the commits you made, the issues you touched and the open pull\n" +
			"requests on your branches over the last day, open the draft in your editor,\n" +
			"and offer to post it to your team's chat. Nothing is posted until you confirm;\n" +
			"--yes posts without asking, and --dry-run prints the draft and posts nothing.\n" +
			"--yes does not skip the editor: add --no-edit to run it unattended.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStandupCommand(cmd, prompt, opts)
		},
	}

	cmd.Flags().IntVar(&opts.days, "days", 1, "how many days back to gather, 1 or more")
	cmd.Flags().BoolVar(&opts.noEdit, "no-edit", false, "skip the editor and use the draft as it is")
	opts.write.addFlags(cmd, "posting")

	return cmd
}

// runStandupCommand wires the real repository, forge, Jira and Slack to standup.
func runStandupCommand(cmd *cobra.Command, prompt Prompt, opts standupOptions) error {
	// A window of no days, or fewer, gathers nothing and asks git and Jira for
	// dates they read differently, so it is refused as a mistake in the call.
	if opts.days < 1 {
		return fmt.Errorf(`%w %q for "--days" flag: it must be 1 or more`, errUsage, strconv.Itoa(opts.days))
	}

	ctx := cmd.Context()

	// A missing configuration is not fatal: the local commits still read, and
	// the service sections simply stay empty.
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	cfg, deps := conn.cfg, conn.deps
	// No tui.GitDeps seam reads recent commits, so standup asks the repository.
	repo := gitrepo.At(proc.Run, conn.where.Root)
	seams := standupSeams{
		Commits:    func(since string) ([]gitrepo.Commit, error) { return repo.RecentCommits(ctx, since) },
		Branches:   deps.Git.Branches,
		FindPull:   deps.Forge.FindPullRequest,
		Search:     deps.Jira.Search,
		Compose:    prompt.Compose,
		Post:       func(text string) error { return deps.Messaging.Post("", text) },
		Confirm:    func(question string) (bool, error) { return confirm(prompt, question) },
		Service:    cfg.Messaging.Service(),
		Configured: cfg.Messaging.Mode() != config.MessagingNone,
	}

	return runStandup(outputOf(cmd), seams, opts)
}

// runStandup gathers the work, offers it for editing, previews it, and posts it
// once confirmed.
func runStandup(out output, seams standupSeams, opts standupOptions) error {
	draft, err := gatherStandup(seams, opts.days)
	if err != nil {
		return err
	}

	if !opts.noEdit && seams.Compose != nil {
		draft, err = seams.Compose(draft)
		if err != nil {
			return fmt.Errorf("editing the standup: %w", err)
		}
	}

	draft = strings.TrimSpace(draft)
	if draft == "" {
		fmt.Fprintln(out.notes, "Nothing to share.")

		return nil
	}

	fmt.Fprintln(out.artifact, draft)

	return offerToPost(out.notes, seams, draft, opts.write)
}

// offerToPost posts the standup to the messaging service, when one is
// configured, once the write options allow it: a dry run says what it would post,
// --yes posts without asking, and otherwise nothing is sent before the
// confirmation. What it says about the post is commentary, written to notes.
func offerToPost(notes io.Writer, seams standupSeams, text string, opts writeOptions) error {
	if !seams.Configured {
		return nil
	}

	proceed, err := opts.proceed(notes, seams.Confirm, writePrompt{
		question: "Post to " + seams.Service + "?",
		dryRun:   "dry run: would post to " + seams.Service,
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.Post(text)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", seams.Service, err)
	}

	fmt.Fprintf(notes, "Posted to %s.\n", seams.Service)

	return nil
}

// gatherStandup collects the commits, issues and pull requests and drafts them.
// A service that will not answer leaves its section empty rather than failing
// the whole draft; only the local commits are required.
func gatherStandup(seams standupSeams, days int) (string, error) {
	commits, err := seams.Commits(gitSince(days))
	if err != nil {
		return "", fmt.Errorf("reading recent commits: %w", err)
	}

	issues, _ := seams.Search(standupJQL(days), 0)
	pulls := gatherPulls(seams)

	return draftStandup(commits, issues.Issues, pulls), nil
}

// gatherPulls collects the open pull requests on your recent local branches.
func gatherPulls(seams standupSeams) []forge.PullRequest {
	branches, err := seams.Branches()
	if err != nil {
		return nil
	}

	var pulls []forge.PullRequest

	for index, branch := range branches {
		if index >= standupBranchLimit {
			break
		}

		pull, found, err := seams.FindPull(branch)
		if err == nil && found && pull.IsOpen() {
			pulls = append(pulls, pull)
		}
	}

	return pulls
}

// gitSince is the git date for the window.
func gitSince(days int) string {
	return strconv.Itoa(days) + " days ago"
}

// standupJQL asks Jira for the issues you touched in the window — a proxy for
// the status changes you made, since a Data Center search reports each issue's
// current status, not its history.
func standupJQL(days int) string {
	return "assignee = currentUser() AND updated >= -" + strconv.Itoa(days) + "d ORDER BY updated DESC"
}

// draftStandup writes the gathered work as a Markdown draft. Every value from a
// service is neutralized, so a hostile subject or title cannot drive the
// terminal or the editor; commit subjects arrive already sanitized.
func draftStandup(commits []gitrepo.Commit, issues []jira.Issue, pulls []forge.PullRequest) string {
	var draft strings.Builder

	draft.WriteString("# Standup\n\n## Commits\n")
	writeCommits(&draft, commits)

	draft.WriteString("\n## Issues\n")
	writeIssues(&draft, issues)

	draft.WriteString("\n## Pull requests\n")
	writePulls(&draft, pulls)

	return draft.String()
}

// writeCommits lists the commits, or says there were none.
func writeCommits(draft *strings.Builder, commits []gitrepo.Commit) {
	if len(commits) == 0 {
		draft.WriteString("- none\n")
	}

	for _, commit := range commits {
		draft.WriteString("- " + commit.Subject + " (" + commit.Hash + ")\n")
	}
}

// writeIssues lists the issues, or says there were none.
func writeIssues(draft *strings.Builder, issues []jira.Issue) {
	if len(issues) == 0 {
		draft.WriteString("- none\n")
	}

	for _, issue := range issues {
		draft.WriteString("- " + string(issue.Key) + " " + sanitize.Text(issue.Summary) +
			" (" + sanitize.Text(issue.Status) + ")\n")
	}
}

// writePulls lists the pull requests, or says there were none.
func writePulls(draft *strings.Builder, pulls []forge.PullRequest) {
	if len(pulls) == 0 {
		draft.WriteString("- none\n")
	}

	for _, pull := range pulls {
		draft.WriteString("- #" + strconv.Itoa(pull.Number) + " " + sanitize.Text(pull.Title) + " " + pull.URL + "\n")
	}
}
