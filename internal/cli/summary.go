// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errFlagsApart refuses flags summary does not take together: --json is the
// summary for a script, which posts nothing, and --yes answers only --post's
// question.
var errFlagsApart = errors.New("--json prints the summary and --yes answers --post's question: " +
	"use --json without --post, and --yes only with --post")

// summarySeams are what `workflow summary` reads and posts through, so a test
// can answer without a repository, a tracker or a messaging service.
type summarySeams struct {
	Activity loop.ActivitySeams
	Now      func() time.Time
	// Post is nil when no messaging transport is set up.
	Post func(channel, text string) error
	// Messaging is where a post goes and how it is rendered.
	Messaging config.Messaging
	Confirm   func(question string) (bool, error)
	// Note names the source being read while it answers.
	Note *progressNote
}

// summaryOptions are summary's flags: the period, the JSON, the post, and the
// dry run and --yes every scriptable write shares.
type summaryOptions struct {
	from   string
	to     string
	asJSON bool
	post   bool
	write  writeOptions
}

// newSummaryCmd builds `workflow summary`.
func newSummaryCmd(prompt Prompt) *cobra.Command {
	var opts summaryOptions

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Say what you did over a period — commits, tasks, issues and reviews — and post it",
		Long: "Read back what you did, as the Summary pane and the web's Summary section read\n" +
			"it: the commits you wrote, the Taskwarrior tasks you touched, what you did to Jira\n" +
			"issues, and the pull or merge requests you opened, had merged and reviewed. It\n" +
			"prints the summary as Markdown, by year, month, day and hour; --json prints the web\n" +
			"API's Activity shape instead. --from and --to name the first and last day, written\n" +
			"YYYY-MM-DD; one alone is that day, and neither is the previous working day.\n\n" +
			"--post previews it and posts it to the configured Slack, Teams, Discord or webhook\n" +
			"once you confirm, its headings written as the service shows them. Nothing is\n" +
			"kept: each run reads the sources again. A source that cannot be read is named in\n" +
			"the summary, and the command exits non-zero once it has printed the rest. A source\n" +
			"that is not set up — no token, no forge the origin names, no Taskwarrior — is left\n" +
			"out, with a note on standard error saying how to set it up.",
		Example: examples(
			`workflow summary --json | jq -r .text   # what you did on the previous working day`,
			`workflow summary --post --yes          # post it to your team, unattended`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSummaryCommand(cmd, prompt, opts)
		},
	}

	cmd.Flags().StringVar(&opts.from, "from", "", "the first day, written YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.to, "to", "", "the last day, written YYYY-MM-DD")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print the summary as JSON")
	cmd.Flags().BoolVar(&opts.post, "post", false, "post the summary to your team's chat, after a preview")
	opts.write.addFlags(cmd, "post without the confirmation")

	return cmd
}

// runSummaryCommand wires the real sources and messaging service to summary.
func runSummaryCommand(cmd *cobra.Command, prompt Prompt, opts summaryOptions) error {
	if opts.asJSON && opts.post || opts.write.yes && !opts.post {
		return fmt.Errorf("%w: %w", errUsage, errFlagsApart)
	}

	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	note := newProgressNote(cmd.ErrOrStderr(), prompt.IsTerminal)
	defer note.clear()

	cfg, deps := conn.cfg, conn.deps
	seams := summarySeams{
		Activity: loop.ActivitySeams{
			Commits: deps.Git.CommitsBetween, Touched: deps.Tasks.Touched,
			Jira: deps.Jira.Activity, BrowseURL: deps.Jira.BrowseURL,
			Forge: deps.Forge.Activity, ForgeKind: deps.Forge.Kind,
		},
		Now:       time.Now,
		Messaging: cfg.Messaging,
		Confirm:   func(question string) (bool, error) { return confirm(prompt, question) },
		Note:      note,
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = deps.Messaging.Post
	}

	return runSummary(note.around(outputOf(cmd)), seams, opts)
}

// runSummary reads every source for the period asked and prints what they
// answered — as Markdown, or as the web's JSON — then posts it when asked.
// A source that could not be read fails the command once the rest is out.
func runSummary(out output, seams summarySeams, opts summaryOptions) error {
	now := seams.Now()
	today := activity.DateOf(now)

	period, err := activity.PeriodAsked(opts.from, opts.to, today)
	if err != nil {
		return fmt.Errorf("%w: the period could not be read: %w", errUsage, err)
	}

	start, end := period.Bounds(now.Location())
	summary := activity.Summary{Period: period, Reads: readNoted(seams, start, end)}
	unread := unreadSources(summary.Reads)

	if opts.asJSON {
		report := webserver.ActivityReport(summary, today, now.Location(), webserver.FaultDetail)
		report.PostLength = webserver.PostLength(seams.Messaging, report.Text)
		err = encodeJSON(out.artifact, report)
		noteLeftOut(out.notes, summary.Reads)

		return errors.Join(err, unread)
	}

	// Every value in it is a source's to write, so none may drive the terminal.
	text := sanitize.Text(summary.Text(now.Location()))
	fmt.Fprint(out.artifact, text)
	noteLeftOut(out.notes, summary.Reads)

	if opts.post {
		err = postSummary(out, seams, text, opts.write)
	}

	return errors.Join(err, unread)
}

// unreadSources is why each source that could not be read was not, one error
// apiece, or nil when every source was.
func unreadSources(reads []activity.Read) error {
	var unread []error

	for _, read := range reads {
		if read.Failed != nil {
			unread = append(unread, fmt.Errorf("%s could not be read: %w", read.Source.Title(), read.Failed))
		}
	}

	return errors.Join(unread...)
}

// noteLeftOut says, of each source that is not set up, that it was left out
// and how to set it up, in the words the web gives it.
func noteLeftOut(notes io.Writer, reads []activity.Read) {
	for _, read := range reads {
		if read.NotSetUp != nil {
			fmt.Fprintln(notes, read.Source.Title()+" is not set up, so it was left out: "+
				webserver.FaultDetail(read.NotSetUp))
		}
	}
}

// postSummary says where the summary goes and posts it there once the write
// options allow: a dry run says what it would post, --yes posts without
// asking, and otherwise nothing is sent before the confirmation. A webhook
// posts to the channel it is bound to, which is said as such. How long the
// summary is against what the service takes is said beside where it goes, and
// a summary longer than that is refused before anything is asked or sent.
func postSummary(out output, seams summarySeams, text string, opts writeOptions) error {
	if seams.Post == nil {
		return fmt.Errorf("%w: set messaging.kind and messaging.webhook_url in %s — or, for Slack, run "+
			"workflow slack login — then check them with workflow doctor", errMessagingNotConfigured, config.FileName)
	}

	service, target := seams.Messaging.Service(), seams.Messaging.Target()
	length := loop.SummaryLength(seams.Messaging.Kind, text)
	fmt.Fprintln(out.notes, "to "+target+", "+length.String())

	err := length.Check()
	if err != nil {
		return err
	}

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: "Post to " + service + "?",
		dryRun:   "dry run: would post to " + target,
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return err
	}

	err = loop.PostSummary(seams.Post, seams.Messaging.Kind, "", text)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", service, err)
	}

	fmt.Fprintln(out.notes, "Posted to "+target+".")

	return nil
}

// readNoted makes each source's read in turn, as loop.ReadAll does, naming on
// the note the source it is reading — the forge by its own name.
func readNoted(seams summarySeams, start, end time.Time) []activity.Read {
	reads := loop.SummaryReads(seams.Activity, start, end)
	made := make([]activity.Read, 0, len(reads))

	for _, read := range reads {
		seams.Note.show("Reading", sourceNoun(read.Source, seams.Activity.ForgeKind))
		made = append(made, read.Read())
	}

	return made
}

// sourceNoun names a source mid-sentence, the forge by its own name when the
// remote says which.
func sourceNoun(source activity.Source, kind forge.Kind) string {
	if source == activity.SourceForge {
		return forgeNoun(kind)
	}

	return source.Name()
}
