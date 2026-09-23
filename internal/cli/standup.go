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
	Confirm  func() (bool, error)
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

// newStandupCmd builds `workflow standup`.
func newStandupCmd(prompt Prompt) *cobra.Command {
	var (
		days   int
		noEdit bool
	)

	cmd := &cobra.Command{
		Use:   "standup",
		Short: "Draft what you did — commits, issues and pull requests — to share",
		Long: "Gather the commits you made, the issues you touched and the open pull\n" +
			"requests on your branches over the last day, open the draft in your editor,\n" +
			"and offer to post it to your team's chat. Nothing is posted until you confirm.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStandupCommand(cmd, prompt, days, noEdit)
		},
	}

	cmd.Flags().IntVar(&days, "days", 1, "how many days back to gather")
	cmd.Flags().BoolVar(&noEdit, "no-edit", false, "skip the editor and use the draft as it is")

	return cmd
}

// runStandupCommand wires the real repository, forge, Jira and Slack to standup.
func runStandupCommand(cmd *cobra.Command, prompt Prompt, days int, noEdit bool) error {
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
		Confirm:    func() (bool, error) { return confirm(prompt, "Post to "+cfg.Messaging.Service()+"?") },
		Service:    cfg.Messaging.Service(),
		Configured: cfg.Messaging.Mode() != config.MessagingNone,
	}

	return runStandup(cmd.OutOrStdout(), seams, days, noEdit)
}

// runStandup gathers the work, offers it for editing, previews it, and posts it
// once confirmed.
func runStandup(out io.Writer, seams standupSeams, days int, noEdit bool) error {
	draft, err := gatherStandup(seams, days)
	if err != nil {
		return err
	}

	if !noEdit && seams.Compose != nil {
		draft, err = seams.Compose(draft)
		if err != nil {
			return fmt.Errorf("editing the standup: %w", err)
		}
	}

	draft = strings.TrimSpace(draft)
	if draft == "" {
		fmt.Fprintln(out, "Nothing to share.")

		return nil
	}

	fmt.Fprintln(out, draft)

	return offerToPost(out, seams, draft)
}

// offerToPost posts the standup to the messaging service after a confirmation,
// when one is configured. Nothing is sent before the confirmation.
func offerToPost(out io.Writer, seams standupSeams, text string) error {
	if !seams.Configured {
		return nil
	}

	post, err := seams.Confirm()
	if err != nil {
		return err
	}

	if !post {
		fmt.Fprintln(out, "Not posted.")

		return nil
	}

	err = seams.Post(text)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", seams.Service, err)
	}

	fmt.Fprintf(out, "Posted to %s.\n", seams.Service)

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
