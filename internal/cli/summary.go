// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errFlagsApart refuses flags summary does not take together: --json is the
// summary for a script, which posts nothing, and --yes and --channel are only
// for a post: --yes answers its question, and --channel says where it goes.
var errFlagsApart = errors.New("--json prints the summary, and --yes and --channel are --post's: " +
	"use --json without --post, and --yes or --channel only with it")

// summarySeams are what `workflow summary` reads and posts through, so a test
// can answer without a repository, a tracker or a messaging service.
type summarySeams struct {
	Activity loop.ActivitySeams
	Now      func() time.Time
	// Post is nil when no messaging transport is set up.
	Post func(ctx context.Context, channel, text string) error
	// Messaging is where a post goes and how it is rendered.
	Messaging config.Messaging
	Confirm   func(question string) (bool, error)
	// Watch runs the reads, keeping a live line of the source being read
	// while it answers.
	Watch func(reads func(console.Wait) error) error
}

// summaryOptions are summary's flags: the period, the JSON, the post and the
// channel it goes to, and the dry run and --yes every scriptable write shares.
type summaryOptions struct {
	from    string
	to      string
	asJSON  bool
	post    bool
	channel string
	write   writeOptions
}

// apart reports flags given together that summary does not take together:
// --json with --post, or --yes or --channel without it.
func (o summaryOptions) apart() bool {
	return o.asJSON && o.post || !o.post && (o.write.yes || strings.TrimSpace(o.channel) != "")
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
			"once you confirm, its headings written as the service shows them; with a Slack\n" +
			"user token, --channel posts it in another channel than messaging.channel.\n" +
			"Nothing is kept: each run reads the sources again. A source that cannot be read\n" +
			"is named in the summary, and the command exits non-zero once it has printed the\n" +
			"rest. A source that is not set up — no token, no forge the base remote names,\n" +
			"no Taskwarrior — is left out, with a note on standard error saying how to set it\n" +
			"up.",
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
	addJSONFlag(cmd, &opts.asJSON, "print the summary as JSON")
	cmd.Flags().BoolVar(&opts.post, "post", false, "post the summary to your team's chat, after a preview")
	cmd.Flags().StringVar(&opts.channel, "channel", "",
		"with --post, post in `CHANNEL` instead of messaging.channel; a Slack user token only")
	opts.write.addFlags(cmd, "post without the confirmation")

	return cmd
}

// runSummaryCommand wires the real sources and messaging service to summary.
func runSummaryCommand(cmd *cobra.Command, prompt Prompt, opts summaryOptions) error {
	if opts.apart() {
		return fmt.Errorf("%w: %w", errUsage, errFlagsApart)
	}

	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	settings, err := postingIn(conn.cfg.Messaging, opts.channel)
	if err != nil {
		return err
	}

	cfg, deps := conn.cfg, conn.deps
	seams := summarySeams{
		Activity: loop.ActivitySeams{
			Commits: deps.Git.CommitsBetween, Touched: deps.Tasks.Touched,
			Jira: deps.Jira.Activity, BrowseURL: deps.Jira.BrowseURL,
			Forge: deps.Forge.Activity, ForgeKind: deps.Forge.Kind,
		},
		Now:       time.Now,
		Messaging: settings,
		Confirm:   func(question string) (bool, error) { return confirm(prompt, question) },
		Watch: func(reads func(console.Wait) error) error {
			return waiting(cmd, prompt, func(_ output, live console.Wait) error { return reads(live) })
		},
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = deps.Messaging.Post
	}

	return runSummary(cmd.Context(), outputOf(cmd), seams, opts)
}

// runSummary reads every source for the period asked under ctx and prints
// what they answered — as Markdown, or as the web's JSON — then posts it when
// asked. A source that could not be read fails the command once the rest is
// out.
func runSummary(ctx context.Context, out output, seams summarySeams, opts summaryOptions) error {
	now := seams.Now()
	today := activity.DateOf(now)

	period, err := activity.PeriodAsked(opts.from, opts.to, today)
	if err != nil {
		return fmt.Errorf("%w: the period could not be read: %w", errUsage, err)
	}

	start, end := period.Bounds(now.Location())

	// A live line that could not be kept fails the command as a source that
	// could not be read does, once what was read is out.
	reads, watchErr := readNoted(ctx, seams, start, end)
	summary := activity.Summary{Period: period, Reads: reads}
	unread := errors.Join(unreadSources(summary.Reads), watchErr)

	if opts.asJSON {
		answer := report.Activity(summary, today, now.Location(), report.FaultDetail)
		answer.PostLength = report.PostLength(seams.Messaging, answer.Text)
		err = encodeJSON(out.artifact, answer)
		noteLeftOut(out.notes, summary.Reads)

		return errors.Join(err, unread)
	}

	// Every value in it is a source's to write, so none may drive the terminal.
	text := sanitize.Text(summary.Text(now.Location()))
	fmt.Fprint(out.artifact, text)
	noteLeftOut(out.notes, summary.Reads)

	if opts.post {
		err = postSummary(ctx, out, seams, text, opts.write)
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
func noteLeftOut(notes console.Notes, reads []activity.Read) {
	for _, read := range reads {
		if read.NotSetUp != nil {
			notes.Note(read.Source.Title() + " is not set up, so it was left out: " +
				report.FaultDetail(read.NotSetUp))
		}
	}
}

// postSummary says where the summary goes and posts it there once the write
// options allow: a dry run says what it would post, --yes posts without
// asking, and otherwise nothing is sent before the confirmation. A webhook
// posts to the channel it is bound to, which is said as such. How long the
// summary is against what the service takes is said beside where it goes, and
// a summary longer than that is refused before anything is asked or sent.
func postSummary(ctx context.Context, out output, seams summarySeams, text string, opts writeOptions) error {
	if seams.Post == nil {
		return fmt.Errorf("%w: set messaging.kind and messaging.webhook_url in %s — or, for Slack, run "+
			"workflow slack login — then check them with workflow doctor", errMessagingNotConfigured, config.FileName)
	}

	service, target := seams.Messaging.Service(), seams.Messaging.Target()
	length := loop.SummaryLength(seams.Messaging.Kind, text)
	out.notes.Note("to " + target + ", " + length.String())

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

	err = loop.PostSummary(ctx, seams.Post, seams.Messaging.Kind, seams.Messaging.Channel, text)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", service, err)
	}

	out.notes.Note("Posted to " + target + ".")

	return nil
}

// readNoted makes each source's read in turn under ctx, as loop.ReadAll does,
// naming on the live line the source it is reading — the forge by its own
// name — and how many it has read.
func readNoted(ctx context.Context, seams summarySeams, start, end time.Time) ([]activity.Read, error) {
	reads := loop.SummaryReads(seams.Activity, start, end)
	made := make([]activity.Read, 0, len(reads))

	err := seams.Watch(func(live console.Wait) error {
		for index, read := range reads {
			live.Doing("Reading " + sourceNoun(read.Source, seams.Activity.ForgeKind))
			live.Counted(index, len(reads))

			made = append(made, read.Read(ctx))
		}

		return nil
	})

	return made, err
}

// sourceNoun names a source mid-sentence, the forge by its own name when the
// remote says which.
func sourceNoun(source activity.Source, kind forge.Kind) string {
	if source == activity.SourceForge {
		return forgeNoun(kind)
	}

	return source.Name()
}
