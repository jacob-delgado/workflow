// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNoPullRequest refuses announcing a branch that has no pull request.
var errNoPullRequest = errors.New("there is no pull request on this branch to announce")

// errMessagingNotConfigured refuses announcing when no messaging transport is
// set up.
var errMessagingNotConfigured = errors.New("no messaging transport is configured")

// announceSeams are what `workflow announce` reads and does, so a test can
// answer without a repository, a forge or Slack.
type announceSeams struct {
	// Compose is what the announcement is composed from.
	Compose loop.AnnounceSeams
	Post    func(channel, text string) error
	Kind    forge.Kind
	Project string
	// Messaging is where the announcement goes and how it is rendered: the
	// channel, the service — whose name the prompt and notices use — and the
	// team's template.
	Messaging config.Messaging
	// Memory is what the store remembers being announced in this repository —
	// the interface's record as well as this command's.
	Memory  loop.AnnounceMemory
	Confirm func(question string) (bool, error)
}

// newAnnounceCmd builds `workflow announce`.
func newAnnounceCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "announce",
		Short: "Announce the branch's pull request to your team's chat",
		Long: "Post the message the messaging pane would — the branch's pull request, its\n" +
			"issue, and where it stands (ready for review, merged, or CI red) — to the\n" +
			"configured Slack, Teams, Discord or webhook. A preview is printed and\n" +
			"confirmed before anything posts.\n\n" +
			"What it posts is remembered, with what the interface posts: a pull request\n" +
			"already announced at the moment it is at is said to be, and asked about again\n" +
			"rather than repeated — with --yes, it is left as it is.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnnounceCommand(cmd, prompt, opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runAnnounceCommand wires the real repository, forge, Jira and Slack to the
// announce flow.
func runAnnounceCommand(cmd *cobra.Command, prompt Prompt, opts writeOptions) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	cfg, deps := conn.cfg, conn.deps
	seams := announceSeams{
		Compose: loop.AnnounceSeams{
			Branch:    deps.Git.Branch,
			FindPull:  deps.Forge.FindPullRequest,
			Author:    deps.Forge.Author,
			Issue:     deps.Jira.Issue,
			BrowseURL: deps.Jira.BrowseURL,
			CheckCI:   deps.Forge.CheckStatus,
		},
		Kind:      deps.Forge.Kind,
		Project:   cfg.Jira.Project,
		Messaging: cfg.Messaging,
		Memory:    announceMemory(deps.Store),
		Confirm:   func(question string) (bool, error) { return confirm(prompt, question) },
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = func(channel, text string) error { return deps.Messaging.Post(channel, text) }
	}

	return runAnnounce(outputOf(cmd), seams, opts)
}

// runAnnounce composes the announcement for the branch's pull request, previews
// it, and posts it once confirmed, remembering it so a later run does not repeat
// it unasked.
func runAnnounce(out output, seams announceSeams, opts writeOptions) error {
	if seams.Post == nil {
		return fmt.Errorf("%w: set messaging.kind and messaging.webhook_url — or messaging.token, for a "+
			"Slack bot — in %s, then check them with workflow doctor", errMessagingNotConfigured, config.FileName)
	}

	announcement, pull, err := loop.ComposeAnnouncement(seams.Compose, seams.Messaging, seams.Project, seams.Kind)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return fmt.Errorf("%w (open one with workflow pr)", errNoPullRequest)
	}

	if err != nil {
		return err
	}

	made := loop.Announced{Pull: pull.Number, Moment: announcement.Moment}

	again := seams.Memory.Holds(made)
	if again && !offerAgain(out.notes, seams.Kind.Sigil()+strconv.Itoa(pull.Number), opts) {
		return nil
	}

	service := seams.Messaging.Service()
	target := announceTarget(seams.Messaging.Channel, service)
	text := announcement.Text()
	fmt.Fprintln(out.artifact, text)
	fmt.Fprintln(out.artifact, "to "+target)

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: postQuestion(service, again),
		dryRun:   "dry run: would post to " + target,
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return unattendedAgain(err, again)
	}

	err = loop.Deliver(seams.Post, seams.Memory, loop.Delivery{Channel: seams.Messaging.Channel, Text: text, Made: made})
	if err != nil {
		return fmt.Errorf("posting to %s: %w", service, err)
	}

	fmt.Fprintln(out.notes, "Posted to "+target)

	return nil
}

// announceTarget names where a post goes: the configured channel, or the
// service's own destination when none is set (a webhook carries its own).
func announceTarget(channel, service string) string {
	if channel == "" {
		return "the configured " + service + " channel"
	}

	return channel
}

// offerAgain says that an earlier session already made this announcement, and
// reports whether to offer it again: asked, yes, but never repeated under --yes,
// which answers only the question it can see coming.
func offerAgain(notes io.Writer, pull string, opts writeOptions) bool {
	fmt.Fprintln(notes, pull+" was already announced at this moment in an earlier session.")

	if opts.yes {
		fmt.Fprintln(notes, "Not posted again; run without --yes to be asked.")

		return false
	}

	return true
}

// postQuestion asks to post to the service, and whether to post again when an
// earlier session already did.
func postQuestion(service string, again bool) string {
	if again {
		return "Post to " + service + " again?"
	}

	return "Post to " + service + "?"
}

// announceMemory is the store's record of the announcements made in this
// repository, which the interface keeps too, in the shared layer's terms.
func announceMemory(kept tui.StoreDeps) loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Recorded: func() []loop.Announced {
			posts := kept.Announced()

			made := make([]loop.Announced, 0, len(posts))
			for _, post := range posts {
				made = append(made, loop.Announced{Pull: post.Pull, Moment: messaging.Moment(post.Moment)})
			}

			return made
		},
		Record: func(made loop.Announced) {
			kept.RecordAnnounce(tui.AnnouncedPost{Pull: made.Pull, Moment: int(made.Moment)})
		},
	}
}

// unattendedAgain words a repeat that nothing could confirm: --yes leaves a
// moment already announced as it is, so the way on is a terminal, not --yes.
func unattendedAgain(err error, again bool) error {
	if again && errors.Is(err, errNoTerminal) {
		return fmt.Errorf("%w; run it at a terminal to be asked whether to announce it again", errNoTerminal)
	}

	return err
}
