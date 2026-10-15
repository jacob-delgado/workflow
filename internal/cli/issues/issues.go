// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package issues is the commands over the issue tracker, Jira or the forge's
// issues: `issues` and `issue`, which read, and `transition`, `assign`,
// `worklog` and `comment`, which write an issue once its preview is
// confirmed.
package issues

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errUnknownView is a --view naming no view the configuration offers.
var errUnknownView = errors.New("name one of the views jira.views configures")

// issuesOptions are `workflow issues`' flags.
type issuesOptions struct {
	view   string
	asJSON bool
}

// NewIssuesCmd builds `workflow issues`.
func NewIssuesCmd() *cobra.Command {
	var opts issuesOptions

	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List a view's issues, as the Issues pane lists them",
		Long: "List the issues of one of jira.views — the first when --view names none, or\n" +
			"the issues assigned to you when none is configured — one a line: the key, the\n" +
			"summary and the status. A view is scoped to you unless its query names the\n" +
			"assignee, as the interfaces scope it. Only the first page is read: the view's\n" +
			"first 50 issues and, on the first view when issues.forge is on, the issues\n" +
			"assigned to you on the repository's forge ahead of them. A longer list is\n" +
			"noted on stderr, and --json carries its total, the forge's issues counted in\n" +
			"it. --json prints the web API's page of issues.",
		Example: cmdkit.Examples(
			`workflow issues --view Sprint`,
			`workflow issues --json | jq -r '.issues[].key'   # the keys alone`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIssuesCommand(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.view, "view", "", "the view to list, by its name in jira.views; the first when not given")
	cmdkit.AddJSONFlag(cmd, &opts.asJSON, "print the page of issues as JSON")

	return cmd
}

// runIssuesCommand wires the real tracker to the listing.
func runIssuesCommand(cmd *cobra.Command, opts issuesOptions) error {
	conn, err := cmdkit.Connect(cmd)
	if err != nil {
		return err
	}
	defer conn.CloseLog()

	view, known := jira.ViewNamed(conn.Config.Jira.Views, opts.view)
	if !known {
		return fmt.Errorf(`%w %q for "--view" flag: %w: %s`,
			cmdkit.ErrUsage, opts.view, errUnknownView, viewNames(conn.Config.Jira.Views))
	}

	return runIssues(cmd.Context(), cmdkit.OutputOf(cmd), conn.Deps.Jira.Search, view, opts.asJSON)
}

// viewNames are the names of the views configured, as a --view may name them.
func viewNames(configured []config.JiraView) string {
	views := jira.Views(configured)

	names := make([]string, 0, len(views))
	for _, view := range views {
		names = append(names, view.Name)
	}

	return strings.Join(names, ", ")
}

// runIssues lists the first page of view's issues, as JSON or a line each.
func runIssues(
	ctx context.Context, out cmdkit.Output,
	search func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error),
	view jira.View, asJSON bool,
) error {
	result, err := search(ctx, view.JQL, 0)
	if err != nil {
		return fmt.Errorf("listing %s: %w", view.Name, err)
	}

	if asJSON {
		return cmdkit.EncodeJSON(out.Artifact, report.IssuesPage(result, 0))
	}

	renderIssueList(out, view, result)

	return nil
}

// renderIssueList writes an issue a line, then what the list leaves out:
// that it is empty, cut short, or missing a tracker that could not be read.
func renderIssueList(out cmdkit.Output, view jira.View, result jira.SearchResult) {
	for _, issue := range result.Issues {
		fmt.Fprintln(out.Artifact, issueRow(issue))
	}

	name := sanitize.Line(view.Name)

	switch {
	case len(result.Issues) == 0:
		out.Notes.Note("No issues in " + name + ".")
	case result.Total > len(result.Issues):
		out.Notes.Note(fmt.Sprintf("Listed the first %d of %d issues in %s.", len(result.Issues), result.Total, name))
	}

	for _, unread := range result.Unavailable {
		out.Notes.Note("Not read: " + unread + ".")
	}
}

// issueRow is an issue's line: its key, its summary and its status. A forge
// issue's key is its bare number, as the tracker keys it, since # begins a
// shell comment.
func issueRow(issue jira.Issue) string {
	return string(issue.Key) + "  " + sanitize.Line(issue.Summary) + "  (" + sanitize.Line(issue.Status) + ")"
}

// issueSeams are what `workflow issue` reads, so a test can answer without a
// tracker.
type issueSeams struct {
	Issue     func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error)
	BrowseURL func(issueKey jira.Key) string
}

// NewIssueCmd builds `workflow issue <issue>`.
func NewIssueCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "issue <issue>",
		Short: "Print an issue in full, in Jira or on the forge",
		Long: "Print an issue in full, as the Issues pane's detail shows it: its summary, its\n" +
			"type, status and priority, who reported it and who it is assigned to, its link,\n" +
			"its description and its comments, oldest first. --json prints the web API's\n" +
			"issue.",
		Example: cmdkit.Examples(
			`workflow issue PROJ-7`,
			`workflow issue PROJ-7 --json | jq -r .status   # where it stands`,
		),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cmdkit.CompleteAssignedIssues,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := cmdkit.Connect(cmd)
			if err != nil {
				return err
			}
			defer conn.CloseLog()

			seams := issueSeams{Issue: conn.Deps.Jira.Issue, BrowseURL: conn.Deps.Jira.BrowseURL}

			return runIssue(cmd.Context(), cmdkit.OutputOf(cmd), seams, jira.Key(args[0]), asJSON)
		},
	}

	cmdkit.AddJSONFlag(cmd, &asJSON, "print the issue as JSON")

	return cmd
}

// runIssue reads the issue and prints it, as JSON or in prose, linked under
// the key the tracker answered with, as the web links it: an issue moved to
// another project is still read by its old key.
func runIssue(ctx context.Context, out cmdkit.Output, seams issueSeams, issueKey jira.Key, asJSON bool) error {
	detail, err := seams.Issue(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("reading issue %s: %w", issueKey, err)
	}

	link := seams.BrowseURL(detail.Issue.Key)
	if asJSON {
		return cmdkit.EncodeJSON(out.Artifact, report.IssueDetail(detail, link))
	}

	renderIssueDetail(out.Artifact, detail, link)

	return nil
}

// renderIssueDetail writes the issue in prose: its heading and facts, its
// description, then its comments.
func renderIssueDetail(artifact io.Writer, detail jira.IssueDetail, link string) {
	issue := detail.Issue

	fmt.Fprintln(artifact, string(issue.Key)+"  "+sanitize.Line(issue.Summary))
	fmt.Fprintln(artifact, issueFacts(issue))
	fmt.Fprintln(artifact, "Reported by "+cmp.Or(sanitize.Line(detail.Reporter), "nobody named")+", "+
		assignedTo(detail.Assignee))

	if link != "" {
		fmt.Fprintln(artifact, link)
	}

	if description := strings.TrimSpace(sanitize.Text(detail.Description)); description != "" {
		fmt.Fprintln(artifact)
		fmt.Fprintln(artifact, description)
	}

	renderComments(artifact, detail)
}

// issueFacts is an issue's type, status and priority, those it has.
func issueFacts(issue jira.Issue) string {
	facts := make([]string, 0, 3) //nolint:mnd // the three facts named

	for _, fact := range []string{issue.Type, issue.Status, issue.Priority} {
		if fact != "" {
			facts = append(facts, sanitize.Line(fact))
		}
	}

	return strings.Join(facts, " · ")
}

// assignedTo says who an issue is assigned to, or that it is not.
func assignedTo(assignee string) string {
	if assignee == "" {
		return "unassigned"
	}

	return "assigned to " + sanitize.Line(assignee)
}

// renderComments writes each comment as its author and date, then its body
// indented, and how many there are when not all were read.
func renderComments(artifact io.Writer, detail jira.IssueDetail) {
	for _, comment := range detail.Comments {
		fmt.Fprintln(artifact)
		fmt.Fprintln(artifact, commentHeading(comment))

		for line := range strings.SplitSeq(strings.TrimSpace(sanitize.Text(comment.Body)), "\n") {
			fmt.Fprintln(artifact, "  "+line)
		}
	}

	if detail.CommentTotal > len(detail.Comments) {
		fmt.Fprintln(artifact)
		fmt.Fprintf(artifact, "%d of %d comments shown.\n", len(detail.Comments), detail.CommentTotal)
	}
}

// commentHeading is a comment's author, and its date when the tracker gave
// one, in the zone the tracker wrote it in.
func commentHeading(comment jira.Comment) string {
	author := cmp.Or(sanitize.Line(comment.Author), "someone")
	if comment.Created.IsZero() {
		return author
	}

	return author + " · " + comment.Created.Format(time.DateOnly)
}
