// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
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
	Confirm   func(question string) (bool, error)
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
			"confirmed before anything posts.",
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
		Confirm:   func(question string) (bool, error) { return confirm(prompt, question) },
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = func(channel, text string) error { return deps.Messaging.Post(channel, text) }
	}

	return runAnnounce(outputOf(cmd), seams, opts)
}

// runAnnounce composes the announcement for the branch's pull request, previews
// it, and posts it once confirmed.
func runAnnounce(out output, seams announceSeams, opts writeOptions) error {
	if seams.Post == nil {
		return fmt.Errorf("%w: set messaging.kind and messaging.webhook_url — or messaging.token, for a "+
			"Slack bot — in %s, then check them with workflow doctor", errMessagingNotConfigured, config.FileName)
	}

	announcement, _, err := loop.ComposeAnnouncement(seams.Compose, seams.Messaging, seams.Project, seams.Kind)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return fmt.Errorf("%w (open one with workflow pr)", errNoPullRequest)
	}

	if err != nil {
		return err
	}

	service := seams.Messaging.Service()
	target := announceTarget(seams.Messaging.Channel, service)
	text := announcement.Text()
	fmt.Fprintln(out.artifact, text)
	fmt.Fprintln(out.artifact, "to "+target)

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: "Post to " + service + "?",
		dryRun:   "dry run: would post to " + target,
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.Post(seams.Messaging.Channel, text)
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
