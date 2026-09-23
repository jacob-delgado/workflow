// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// errNoPullRequest refuses announcing a branch that has no pull request.
var errNoPullRequest = errors.New("there is no pull request on this branch to announce")

// errMessagingNotConfigured refuses announcing when no messaging transport is
// set up.
var errMessagingNotConfigured = errors.New("no messaging transport is configured")

// announceSeams are what `workflow announce` reads and does, so a test can
// answer without a repository, a forge or Slack.
type announceSeams struct {
	Branch    func() (gitrepo.Branch, error)
	FindPull  func(branch string) (forge.PullRequest, bool, error)
	Author    func() (string, error)
	Issue     func(jira.Key) (jira.IssueDetail, error)
	BrowseURL func(jira.Key) string
	CheckCI   func(pull forge.PullRequest, head string) (forge.CI, error)
	Post      func(channel, text string) error
	Kind      forge.Kind
	Project   string
	Channel   string
	Template  string
	// MessagingKind is the service the announcement is rendered for: it decides
	// the link markup Text() emits.
	MessagingKind config.MessagingKind
	// Service names the messaging service for the user — "Slack", "Teams",
	// "Discord" or "webhook" — in the prompt and notices.
	Service string
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
			"confirmed before anything posts.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnnounceCommand(cmd, prompt, opts)
		},
	}

	opts.addFlags(cmd, "posting")

	return cmd
}

// runAnnounceCommand wires the real repository, forge, Jira and Slack to the
// announce flow.
func runAnnounceCommand(cmd *cobra.Command, prompt Prompt, opts writeOptions) error {
	ctx := cmd.Context()

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining the working directory: %w", err)
	}

	home, _ := os.UserHomeDir()
	cfg, _ := config.Load(dir, home)
	deps := wiring.Deps(ctx, cfg, wiring.Locate(ctx, dir), nil)

	seams := announceSeams{
		Branch:        deps.Git.Branch,
		FindPull:      deps.Forge.FindPullRequest,
		Author:        deps.Forge.Author,
		Issue:         deps.Jira.Issue,
		BrowseURL:     deps.Jira.BrowseURL,
		CheckCI:       deps.Forge.CheckStatus,
		Kind:          deps.Forge.Kind,
		Project:       cfg.Jira.Project,
		Channel:       cfg.Messaging.Channel,
		Template:      cfg.Messaging.Announcement,
		MessagingKind: cfg.Messaging.Kind,
		Service:       cfg.Messaging.Service(),
		Confirm:       func(question string) (bool, error) { return confirm(prompt, question) },
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = func(channel, text string) error { return deps.Messaging.Post(channel, text) }
	}

	return runAnnounce(cmd.OutOrStdout(), seams, opts)
}

// runAnnounce composes the announcement for the branch's pull request, previews
// it, and posts it once confirmed.
func runAnnounce(out io.Writer, seams announceSeams, opts writeOptions) error {
	if seams.Post == nil {
		return errMessagingNotConfigured
	}

	branch, err := seams.Branch()
	if err != nil {
		return fmt.Errorf("reading the branch: %w", err)
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil {
		return fmt.Errorf("reading the pull request: %w", err)
	}

	if !found {
		return errNoPullRequest
	}

	text := composeAnnouncement(seams, branch, pull).Text()
	fmt.Fprintln(out, text)
	fmt.Fprintln(out, "to "+announceTarget(seams.Channel, seams.Service))

	proceed, err := opts.proceed(out, seams.Confirm, writePrompt{
		question: "Post to " + seams.Service + "?",
		dryRun:   "dry run: would post to " + announceTarget(seams.Channel, seams.Service),
		declined: "Not posted.",
	})
	if err != nil || !proceed {
		return err
	}

	err = seams.Post(seams.Channel, text)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", seams.Service, err)
	}

	fmt.Fprintln(out, "Posted to "+announceTarget(seams.Channel, seams.Service))

	return nil
}

// composeAnnouncement builds the announcement for the branch's pull request,
// marking the moment it is at.
func composeAnnouncement(seams announceSeams, branch gitrepo.Branch, pull forge.PullRequest) messaging.Announcement {
	key, _ := convention.IssueKey(branch.Name, seams.Project)
	issueKey := jira.Key(key)

	return messaging.Announcement{
		Author:           announceAuthor(seams),
		PullRequestURL:   pull.URL,
		PullRequestTitle: pull.Title,
		IssueKey:         key,
		IssueSummary:     announceIssueSummary(seams, issueKey),
		IssueURL:         announceIssueURL(seams, issueKey),
		Noun:             seams.Kind.Noun(),
		Moment:           announceMoment(seams, pull, branch.Head),
		Kind:             seams.MessagingKind,
		Template:         seams.Template,
	}
}

// announceMoment is the moment the pull request is at: merged, its CI red, or —
// the common case — ready for review. A CI read that fails leaves the moment at
// ready rather than failing the announcement.
func announceMoment(seams announceSeams, pull forge.PullRequest, head string) messaging.Moment {
	if pull.State == forge.StateMerged {
		return messaging.MomentMerged
	}

	ci, err := seams.CheckCI(pull, head)
	if err == nil && ci.State == forge.CIFailed {
		return messaging.MomentCIRed
	}

	return messaging.MomentReady
}

// announceTarget names where a post goes: the configured channel, or the
// service's own destination when none is set (a webhook carries its own).
func announceTarget(channel, service string) string {
	if channel == "" {
		return "the configured " + service + " channel"
	}

	return channel
}

// announceAuthor is who the forge credential belongs to, or empty when it will
// not say — the announcement reads without it.
func announceAuthor(seams announceSeams) string {
	author, err := seams.Author()
	if err != nil {
		return ""
	}

	return author
}

// announceIssueSummary is the branch issue's summary, or empty when the branch
// names no issue or the tracker cannot say.
func announceIssueSummary(seams announceSeams, key jira.Key) string {
	if key == "" {
		return ""
	}

	detail, err := seams.Issue(key)
	if err != nil {
		return ""
	}

	return detail.Issue.Summary
}

// announceIssueURL links the issue, or empty when the branch names no issue or
// the tracker has no web address for one — issues read from the forge have none.
func announceIssueURL(seams announceSeams, key jira.Key) string {
	if key == "" || seams.BrowseURL == nil {
		return ""
	}

	return seams.BrowseURL(key)
}
