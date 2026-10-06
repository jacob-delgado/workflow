// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errNoPullRequest refuses announcing a branch that has no pull request.
var errNoPullRequest = errors.New("there is no pull request on this branch to announce")

// errMessagingNotConfigured refuses announcing when no messaging transport is
// set up.
var errMessagingNotConfigured = errors.New("no messaging transport is configured")

// announceSeams are what `workflow announce` reads and does, so a test can
// answer without a repository, a forge or a messaging service.
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
	// Tagging is whom a ready-for-review announcement tags on Slack.
	Tagging announceTagging
}

// announceTagging is what an announcement's tags are read through: the code
// owners of the branch's changes, whom each is on Slack and the repository's
// user groups, kept between sessions in the Slack workspace Workspace names,
// and the Slack directory, read only to learn whether the token has the
// scopes tagging needs. A nil ChannelMembers — no Slack user token — or
// OwnerLinks — no store — tags no one.
type announceTagging struct {
	Owners       loop.OwnerSeams
	OwnerLinks   func(workspace string) ([]loop.OwnerLink, error)
	RepoGroups   func(workspace string) ([]loop.SlackTarget, error)
	LastGroups   func(workspace string) ([]string, bool)
	RecordGroups func(workspace string, ids []string) error
	Workspace    func() (string, error)
	Grant        func() (messaging.Grant, error)
}

// newAnnounceCmd builds `workflow announce`.
func newAnnounceCmd(prompt Prompt) *cobra.Command {
	var opts writeOptions

	cmd := &cobra.Command{
		Use:   "announce",
		Short: "Announce the branch's pull or merge request to your team's chat",
		Long: "Announce what the messaging pane would — the branch's pull request, its\n" +
			"issue, and where it stands (ready for review, merged, or CI red) — to the\n" +
			"configured Slack, Teams, Discord or webhook. A preview is printed and\n" +
			"confirmed before anything is announced.\n\n" +
			"With a Slack user token, a ready-for-review announcement tags the code owners\n" +
			"of the branch's changes already linked to Slack, and the user groups chosen for\n" +
			"the repository, on a line after the text. It never asks: an owner not linked yet\n" +
			"is named in the preview, to link in the interface's People and groups. A token\n" +
			"without the scopes tagging needs posts untagged, naming the scope to add.\n\n" +
			"What it announces is remembered, with what the interface announces: a pull\n" +
			"request already announced at the moment it is at is said to be, and asked about\n" +
			"again rather than repeated — with --yes, it is left as it is.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnnounceCommand(cmd, prompt, opts)
		},
	}

	opts.addFlags(cmd, confirmationHelp)

	return cmd
}

// runAnnounceCommand wires the real repository, forge, Jira and messaging
// service to the announce flow.
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
		Memory:    loop.AnnounceMemory{Recorded: deps.Store.Announced, Record: deps.Store.RecordAnnounce},
		Confirm:   func(question string) (bool, error) { return confirm(prompt, question) },
		Tagging: announceTagging{
			Owners: loop.OwnerSeams{
				ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
				IsGroup: deps.Forge.IsGroup,
			},
			OwnerLinks:   deps.Store.OwnerLinks,
			RepoGroups:   deps.Store.RepoGroups,
			LastGroups:   deps.Store.LastGroups,
			RecordGroups: deps.Store.RecordGroups,
		},
	}

	// The directory seams are bound whatever the settings; only a Slack user
	// token can read the directory, so only it tags anyone.
	if cfg.Messaging.Mode() == config.MessagingUser {
		seams.Tagging.Workspace = deps.Messaging.Workspace
		seams.Tagging.Grant = deps.Messaging.Grant
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Post = func(channel, text string) error { return deps.Messaging.Post(channel, text) }
	}

	return runAnnounce(outputOf(cmd), seams, opts)
}

// runAnnounce composes the announcement for the branch's pull request, previews
// it, and announces it once confirmed, remembering it so a later run does not
// repeat it unasked.
func runAnnounce(out output, seams announceSeams, opts writeOptions) error {
	if seams.Post == nil {
		return fmt.Errorf("%w: set messaging.kind and messaging.webhook_url in %s — or, for Slack, run "+
			"workflow slack login — then check them with workflow doctor", errMessagingNotConfigured, config.FileName)
	}

	announcement, pull, err := loop.ComposeAnnouncement(seams.Compose, seams.Messaging, seams.Project, seams.Kind)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return fmt.Errorf("%w; open one with `workflow pr`", errNoPullRequest)
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

	mentions, memory := tagAnnouncement(out, seams, announcement.Moment, pull.Base)

	proceed, err := opts.proceed(out.notes, seams.Confirm, announcePrompt(service, target, again, opts))
	if err != nil || !proceed {
		return unattendedAgain(err, again)
	}

	err = loop.Deliver(seams.Post, memory, loop.Delivery{
		Channel: seams.Messaging.Channel, Text: text, Made: made, Mentions: mentions,
	})
	if err != nil {
		return fmt.Errorf("announcing to %s: %w", service, err)
	}

	fmt.Fprintln(out.notes, "Announced to "+target+".")

	return nil
}

// announceTarget names where an announcement goes: the configured channel, or
// the service's own destination when none is set (a webhook carries its own).
func announceTarget(channel, service string) string {
	if channel == "" {
		return "the configured " + service + " channel"
	}

	return channel
}

// offerAgain says that an earlier session already made this announcement, and
// reports whether to offer it again: asked, yes, but never repeated under --yes,
// which answers only the question it can see coming. A dry run goes on all the
// same, to preview the announcement and say what --yes would do with it.
func offerAgain(notes io.Writer, pull string, opts writeOptions) bool {
	fmt.Fprintln(notes, pull+" was already announced at this moment in an earlier session.")

	if opts.yes && !opts.dryRun {
		fmt.Fprintln(notes, "Not announced again; run without --yes to be asked.")

		return false
	}

	return true
}

// announcePrompt is what announce says around its confirmation: it asks
// whether to announce again when an earlier session already did, and its dry
// run under --yes says that such a moment would be left as it is.
func announcePrompt(service, target string, again bool, opts writeOptions) writePrompt {
	prompt := writePrompt{
		question: "Announce to " + service + "?",
		dryRun:   "dry run: would announce to " + target,
		declined: "Not announced.",
	}

	if again {
		prompt.question = "Announce to " + service + " again?"
	}

	if again && opts.yes {
		prompt.dryRun = "dry run: would not announce it again; --yes leaves an announced moment as it is"
	}

	return prompt
}

// unattendedAgain words a repeat that nothing could confirm: --yes leaves a
// moment already announced as it is, so the way on is a terminal, not --yes.
func unattendedAgain(err error, again bool) error {
	if again && errors.Is(err, errNoTerminal) {
		return fmt.Errorf("%w; run it at a terminal to be asked whether to announce it again", errNoTerminal)
	}

	return err
}

// tagAnnouncement previews whom the announcement at moment tags — the
// linked owners and the groups that start checked, never asked — and who it
// leaves untagged, and answers the tags with what the post remembers: the
// groups chosen, when groups were offered. A token that lacks a scope tagging
// needs is told the scope, and the post goes out untagged. base is the branch
// the pull request merges into, as the forge says, or empty where it did not.
func tagAnnouncement(out output, seams announceSeams, moment messaging.Moment, base string) (
	messaging.Mentions, loop.AnnounceMemory,
) {
	memory := seams.Memory

	workspace, tagging := seams.Tagging.workspace(out, moment)
	if !tagging {
		return messaging.Mentions{}, memory
	}

	tags := seams.Tagging.propose(seams.Compose.Branch, base, moment, workspace)
	if len(tags.Owners)+len(tags.Groups) == 0 {
		return messaging.Mentions{}, memory
	}

	if scope, missing := seams.Tagging.missingScope(tags); missing {
		fmt.Fprintf(out.notes, "Not tagging anyone: the Slack token lacks the %s scope; "+
			"add it to the Slack app, then run workflow slack login.\n", scope)

		return messaging.Mentions{}, memory
	}

	mentions, err := tags.Mentions(checkedGroups(tags.Groups))
	if err != nil {
		return messaging.Mentions{}, memory
	}

	fmt.Fprintln(out.artifact, "tags "+taggedNames(tags))

	if untagged := untaggedOwners(tags.Owners); untagged != "" {
		fmt.Fprintln(out.artifact, "not tagged: "+untagged)
	}

	if record := seams.Tagging.RecordGroups; len(tags.Groups) > 0 && record != nil {
		memory.RecordGroups = func(ids []string) error { return record(workspace, ids) }
	}

	return mentions, memory
}

// workspace is the Slack workspace an announcement at moment tags in, and
// whether it tags anyone at all: only one ready for review does, with a Slack
// user token and the store, and only in a workspace Slack names. When Slack
// cannot say which, the post goes out untagged, and the notes say why.
func (t announceTagging) workspace(out output, moment messaging.Moment) (string, bool) {
	if moment != messaging.MomentReady || t.Workspace == nil || t.OwnerLinks == nil {
		return "", false
	}

	workspace, err := loop.TagWorkspace(t.Workspace)
	if errors.Is(err, loop.ErrUnknownWorkspace) {
		fmt.Fprintf(out.notes, "Not tagging anyone: %v; posting untagged.\n", err)
	}

	return workspace, err == nil
}

// propose is whom an announcement at moment proposes to tag from what is kept
// in workspace. The owners are those of the changes since pullBase, the
// branch the pull request merges into, or since the base the branch is read
// to have left when the forge did not say. The tags are a proposal, so a read
// that fails proposes fewer rather than holding the announcement back.
func (t announceTagging) propose(
	branch func() (gitrepo.Branch, error), pullBase string, moment messaging.Moment, workspace string,
) loop.Tags {
	owners, _ := loop.OwnersOf(t.Owners, cmp.Or(pullBase, localBase(branch)))
	links, _ := t.OwnerLinks(workspace)

	var (
		repoGroups []loop.SlackTarget
		last       []string
		chosen     bool
	)

	if t.RepoGroups != nil {
		repoGroups, _ = t.RepoGroups(workspace)
	}

	if t.LastGroups != nil {
		last, chosen = t.LastGroups(workspace)
	}

	return loop.ProposeTags(owners, links, repoGroups, last, chosen, moment)
}

// localBase is the base the branch is read to have left, or empty when the
// branch cannot be read.
func localBase(branch func() (gitrepo.Branch, error)) string {
	current, err := branch()
	if err != nil {
		return ""
	}

	return current.Base
}

// missingScope is a scope the Slack token lacks to tag, as auth.test lists
// what it was granted: the channel's members need users:read and the channel
// scopes, and groups usergroups:read. It reads no directory, which in a large
// workspace is many pages, only to learn a scope; a token whose scopes Slack
// did not list is taken to lack none.
func (t announceTagging) missingScope(tags loop.Tags) (string, bool) {
	if t.Grant == nil {
		return "", false
	}

	grant, err := t.Grant()
	if err != nil {
		return "", false
	}

	needed := []string{"users:read", "channels:read", "groups:read"}
	if len(tags.Groups) > 0 {
		needed = append(needed, "usergroups:read")
	}

	for _, scope := range needed {
		if grant.Lacks(scope) {
			return scope, true
		}
	}

	return "", false
}

// checkedGroups is the ID of every group that starts checked.
func checkedGroups(groups []loop.GroupTag) []string {
	var ids []string

	for _, group := range groups {
		if group.Checked {
			ids = append(ids, group.Slack.ID)
		}
	}

	return ids
}

// taggedNames names everyone the post tags, as Slack shows them: the linked
// people, then the checked groups; or no one.
func taggedNames(tags loop.Tags) string {
	var names []string

	for _, owner := range tags.Owners {
		if !owner.Team && owner.State == loop.OwnerLinked {
			names = append(names, "@"+sanitize.Line(owner.Slack.Label))
		}
	}

	for _, group := range tags.Groups {
		if group.Checked {
			names = append(names, "@"+sanitize.Line(group.Slack.Label))
		}
	}

	if len(names) == 0 {
		return "no one"
	}

	return strings.Join(names, " ")
}

// untaggedOwners names each owner the post leaves untagged, and why.
func untaggedOwners(owners []loop.OwnerTag) string {
	reasons := map[loop.OwnerState]string{
		loop.OwnerUnlinked:   " (not linked — associate in People and groups)",
		loop.OwnerNotOnSlack: " (not on Slack)",
		loop.OwnerLinked:     "",
	}

	var untagged []string

	for _, owner := range owners {
		if owner.State != loop.OwnerLinked {
			untagged = append(untagged, owner.Owner+reasons[owner.State])
		}
	}

	return strings.Join(untagged, ", ")
}
