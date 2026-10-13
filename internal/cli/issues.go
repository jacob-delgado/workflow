// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

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

// newIssuesCmd builds `workflow issues`.
func newIssuesCmd() *cobra.Command {
	var opts issuesOptions

	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List a view's issues, as the Issues pane lists them",
		Long: "List the issues of one of jira.views — the first when --view names none, or\n" +
			"the issues assigned to you when none is configured — one a line: the key, the\n" +
			"summary and the status. A view is scoped to you unless its query names the\n" +
			"assignee, as the interfaces scope it, and the forge's issues join the first\n" +
			"view when issues.forge is on. Only the first page of 50 is read: a longer\n" +
			"list is noted on stderr, and --json carries its total. --json prints the web\n" +
			"API's page of issues.",
		Example: examples(
			`workflow issues --view Sprint`,
			`workflow issues --json | jq -r '.issues[].key'   # the keys alone`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIssuesCommand(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.view, "view", "", "the view to list, by its name in jira.views; the first when not given")
	addJSONFlag(cmd, &opts.asJSON, "print the page of issues as JSON")

	return cmd
}

// runIssuesCommand wires the real tracker to the listing.
func runIssuesCommand(cmd *cobra.Command, opts issuesOptions) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	view, known := jira.ViewNamed(conn.cfg.Jira.Views, opts.view)
	if !known {
		return fmt.Errorf(`%w %q for "--view" flag: %w: %s`,
			errUsage, opts.view, errUnknownView, viewNames(conn.cfg.Jira.Views))
	}

	return runIssues(cmd.Context(), outputOf(cmd), conn.deps.Jira.Search, view, opts.asJSON)
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
	ctx context.Context, out output,
	search func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error),
	view jira.View, asJSON bool,
) error {
	result, err := search(ctx, view.JQL, 0)
	if err != nil {
		return fmt.Errorf("listing %s: %w", view.Name, err)
	}

	if asJSON {
		return encodeJSON(out.artifact, report.IssuesPage(result, 0))
	}

	renderIssueList(out, view, result)

	return nil
}

// renderIssueList writes an issue a line, then what the list leaves out:
// that it is empty, cut short, or missing a tracker that could not be read.
func renderIssueList(out output, view jira.View, result jira.SearchResult) {
	for _, issue := range result.Issues {
		fmt.Fprintln(out.artifact, issueRow(issue))
	}

	name := sanitize.Line(view.Name)

	switch {
	case len(result.Issues) == 0:
		fmt.Fprintln(out.notes, "No issues in "+name+".")
	case result.Total > len(result.Issues):
		fmt.Fprintf(out.notes, "Listed the first %d of %d issues in %s.\n", len(result.Issues), result.Total, name)
	}

	for _, unread := range result.Unavailable {
		fmt.Fprintln(out.notes, "Not read: "+unread+".")
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

// newIssueCmd builds `workflow issue <issue>`.
func newIssueCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "issue <issue>",
		Short: "Print an issue in full, in Jira or on the forge",
		Long: "Print an issue in full, as the Issues pane's detail shows it: its summary, its\n" +
			"type, status and priority, who reported it and who it is assigned to, its link,\n" +
			"its description and its comments, oldest first. --json prints the web API's\n" +
			"issue.",
		Example: examples(
			`workflow issue PROJ-7`,
			`workflow issue PROJ-7 --json | jq -r .status   # where it stands`,
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := connect(cmd)
			if err != nil {
				return err
			}
			defer conn.closeLog()

			seams := issueSeams{Issue: conn.deps.Jira.Issue, BrowseURL: conn.deps.Jira.BrowseURL}

			return runIssue(cmd.Context(), outputOf(cmd), seams, jira.Key(args[0]), asJSON)
		},
	}

	addJSONFlag(cmd, &asJSON, "print the issue as JSON")

	return cmd
}

// runIssue reads the issue and prints it, as JSON or in prose.
func runIssue(ctx context.Context, out output, seams issueSeams, issueKey jira.Key, asJSON bool) error {
	detail, err := seams.Issue(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("reading issue %s: %w", issueKey, err)
	}

	link := seams.BrowseURL(issueKey)
	if asJSON {
		return encodeJSON(out.artifact, report.IssueDetail(detail, link))
	}

	renderIssueDetail(out.artifact, detail, link)

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

// completeAssignedIssues completes an issue argument with the keys of the issues
// assigned to you, for `workflow branch <tab>`. A tracker that cannot be reached
// offers nothing rather than an error.
func completeAssignedIssues(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dir, err := workingDir(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Completion runs on every <tab>, so it records nothing in a request log.
	conn := connectAt(cmd.Context(), cmd, dir, nil)

	result, err := conn.deps.Jira.Search(cmd.Context(), jira.AssignedToMe, 0)
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
