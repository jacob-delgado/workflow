// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNoPullRequest refuses announcing a branch that has no pull request. It is
// told through forgeWordedError, in the forge's own noun.
var errNoPullRequest = errors.New("there is no pull request on this branch to announce")

// announceSeams are what `workflow announce` reads and does, so a test can
// answer without a repository, a forge or a messaging service.
type announceSeams struct {
	// Compose is what the announcement is composed from.
	Compose loop.AnnounceSeams
	// Deliver is what the announcement goes out through: the post, and the
	// CI read its gate makes first.
	Deliver loop.DeliverSeams
	// Interval is how often a wait for CI to pass reads it.
	Interval time.Duration
	Kind     forge.Kind
	Project  string
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
	// Watch runs a wait for CI to pass, keeping a live line of its progress
	// while a person watches the terminal.
	Watch func(wait func(console.Wait) error) error
}

// announceFlags are announce's flags: --yes and --dry-run, the channel to
// announce in instead of messaging.channel, and whether to wait for the pull
// request's CI to pass before announcing it.
type announceFlags struct {
	write     cmdkit.WriteOptions
	channel   string
	whenGreen bool
}

// newAnnounceCmd builds `workflow announce`.
func newAnnounceCmd(prompt cmdkit.Prompt) *cobra.Command {
	var flags announceFlags

	cmd := &cobra.Command{
		Use:   "announce",
		Short: "Announce the branch's pull or merge request to your team's chat",
		Long: "Announce what the messaging pane would — the branch's pull request, its\n" +
			"issue, and where it stands (ready for review, or merged) — to the\n" +
			"configured Slack, Teams, Discord or webhook. A preview is printed and\n" +
			"confirmed before anything is announced.\n\n" +
			"With a Slack user token, a ready-for-review announcement tags the code owners\n" +
			"of the branch's changes already linked to Slack, and the user groups chosen for\n" +
			"the repository, on a line after the text. It never asks: an owner not linked yet\n" +
			"is named in the preview, to link in the interface's People and groups. A token\n" +
			"without the scopes tagging needs posts untagged, naming the scope to add.\n\n" +
			"What it announces is remembered, with what the interface announces: a pull\n" +
			"request already announced at the moment it is at is said to be, and asked about\n" +
			"again rather than repeated — with --yes, it is left as it is. --channel announces\n" +
			"in another channel than messaging.channel, which only a Slack user token can.\n\n" +
			"Nothing is announced while a check has failed on the pull request. With\n" +
			"--when-green, announce waits in the foreground for its CI to pass, through a\n" +
			"failed run, then announces it; ctrl+c stops waiting without announcing, and a\n" +
			"pull request that reports no check for ten minutes is given up on.",
		Example: cmdkit.Examples(
			`workflow --dry-run announce            # the announcement, and where it would go`,
			`workflow announce --yes                # announce it, unattended`,
			`workflow announce --when-green --yes   # announce it once CI passes, unattended`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnnounceCommand(cmd, prompt, flags)
		},
	}

	flags.write.AddFlags(cmd, cmdkit.ConfirmationHelp)
	cmd.Flags().StringVar(&flags.channel, "channel", "",
		"announce in `CHANNEL` instead of messaging.channel; a Slack user token only")
	cmd.Flags().BoolVar(&flags.whenGreen, "when-green", false,
		"wait in the foreground for the pull request's CI to pass, then announce it")

	return cmd
}

// runAnnounceCommand wires the real repository, forge, Jira and messaging
// service to the announce flow, posting in the channel flags give when one is
// given.
func runAnnounceCommand(cmd *cobra.Command, prompt cmdkit.Prompt, flags announceFlags) error {
	conn, err := cmdkit.Connect(cmd)
	if err != nil {
		return err
	}
	defer conn.CloseLog()

	seams := announceSeamsOf(cmd, conn, prompt)

	seams.Messaging, err = cmdkit.PostingIn(seams.Messaging, flags.channel)
	if err != nil {
		return err
	}

	return runAnnounce(cmd.Context(), cmdkit.OutputOf(cmd), seams, flags)
}

// announceSeamsOf wires conn's repository, forge, Jira, store and messaging
// service to what announcing reads and does, asking prompt to confirm and
// keeping cmd's live line while it waits for CI. It posts to the configured
// channel; a command that posts elsewhere says so itself.
func announceSeamsOf(cmd *cobra.Command, conn cmdkit.Connection, prompt cmdkit.Prompt) announceSeams {
	cfg, deps := conn.Config, conn.Deps
	seams := announceSeams{
		Compose: loop.AnnounceSeams{
			Branch:    deps.Git.Branch,
			FindPull:  deps.Forge.FindPullRequest,
			Author:    deps.Forge.Author,
			Issue:     deps.Jira.Issue,
			BrowseURL: deps.Jira.BrowseURL,
		},
		Deliver:   loop.DeliverSeams{Post: nil, CheckCI: deps.Forge.CheckStatus},
		Interval:  cmp.Or(cfg.CIInterval(), config.DefaultCIInterval),
		Kind:      deps.Forge.Kind,
		Project:   cfg.Jira.Project,
		Messaging: cfg.Messaging,
		Memory:    loop.AnnounceMemory{Recorded: deps.Store.Announced, Record: deps.Store.RecordAnnounce},
		Confirm:   func(question string) (bool, error) { return cmdkit.Confirm(prompt, question) },
		Tagging:   taggingFor(cfg.Messaging, deps),
		Watch: func(wait func(console.Wait) error) error {
			return cmdkit.Waiting(cmd, prompt, cmdkit.LiveLook(cfg.UI), func(_ cmdkit.Output, live console.Wait) error {
				return wait(live)
			})
		},
	}

	if cfg.Messaging.Mode() != config.MessagingNone {
		seams.Deliver.Post = deps.Messaging.Post
	}

	return seams
}

// taggingFor is what an announcement posted under settings reads its tags
// through, from the repository, forge, store and Slack deps wires.
func taggingFor(settings config.Messaging, deps tui.Deps) announceTagging {
	tagging := announceTagging{
		Owners: loop.OwnerSeams{
			ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
			IsGroup: deps.Forge.IsGroup,
		},
		OwnerLinks:   deps.Store.OwnerLinks,
		RepoGroups:   deps.Store.RepoGroups,
		LastGroups:   deps.Store.LastGroups,
		RecordGroups: deps.Store.RecordGroups,
	}

	// The directory seams are bound whatever the settings; only a Slack user
	// token can read the directory, so only it tags anyone.
	if settings.Mode() == config.MessagingUser {
		tagging.Workspace = deps.Messaging.Workspace
		tagging.Grant = deps.Messaging.Grant
	}

	return tagging
}

// runAnnounce composes the announcement for the branch's pull request, previews
// it, and announces it once confirmed — at once, or, with --when-green, once
// its CI passes — remembering it so a later run does not repeat it unasked.
// Slack is asked under ctx.
func runAnnounce(ctx context.Context, out cmdkit.Output, seams announceSeams, flags announceFlags) error {
	opts := flags.write

	composed, err := composeFor(ctx, seams)
	if err != nil {
		return err
	}

	pull := seams.Kind.Sigil() + strconv.Itoa(composed.Pull.Number)

	again := seams.Memory.Holds(ctx, composed.Made())
	if again && !offerAgain(out.Notes, pull, opts) {
		return nil
	}

	err = readyToAnnounce(ctx, seams.Deliver.CheckCI, composed, pull, flags.whenGreen)
	if err != nil {
		return err
	}

	pending := previewAnnouncement(ctx, out, seams, composed)

	say := announcePrompt(seams.Messaging.Service(), seams.Messaging.Target(), again, opts)
	if flags.whenGreen {
		say = onceGreen(say, pull)
	}

	proceed, err := opts.Proceed(out.Notes, seams.Confirm, say)
	if err != nil || !proceed {
		return unattendedAgain(err, again)
	}

	if flags.whenGreen {
		return deliverWhenGreen(ctx, seams, pending, pull, pullFoundBefore)
	}

	return deliverAnnouncement(ctx, out.Notes, seams, pending)
}

// readyToAnnounce refuses composed's pull request, named pull, that cannot be
// announced as asked: now, while a check has failed on it or its CI cannot be
// read; or once CI passes, when it is not ready for review, having merged.
func readyToAnnounce(
	ctx context.Context, check func(context.Context, forge.PullRequest, string) (forge.CI, error),
	composed loop.Composed, pull string, whenGreen bool,
) error {
	if !whenGreen {
		return clearToAnnounce(ctx, check, composed, pull)
	}

	if composed.Announcement.Moment != messaging.MomentReady {
		return forgeWordedError{errOnlyReadyWaits, pull + " has merged, so there is no CI to wait for; " +
			"announce it with workflow announce"}
	}

	return nil
}

// composeFor composes the announcement seams can post, refusing where nothing
// can post it or the branch has no pull request to announce.
func composeFor(ctx context.Context, seams announceSeams) (loop.Composed, error) {
	if seams.Deliver.Post == nil {
		return loop.Composed{}, cmdkit.MessagingNotConfigured()
	}

	composed, err := loop.ComposeAnnouncement(ctx, seams.Compose, seams.Messaging, seams.Project, seams.Kind)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return loop.Composed{}, forgeWordedError{errNoPullRequest, "there is no " + seams.Kind.Noun() +
			" on this branch to announce; open one with `workflow pr`"}
	}

	return composed, err
}

// ownersBase is the branch the owners of composed's changes are read against:
// the one the pull request merges into, as the forge says, or the base the
// branch was read to have left where the forge did not say.
func ownersBase(composed loop.Composed) string {
	return cmp.Or(composed.Pull.Base, composed.Branch.Base)
}

// clearToAnnounce refuses, before anything is previewed or asked, to announce
// composed's pull request, named pull, while a check has failed on it, naming
// the checks that failed and the way on, or when its CI cannot be read to
// confirm that none has. The forge is asked under ctx.
func clearToAnnounce(
	ctx context.Context, check func(context.Context, forge.PullRequest, string) (forge.CI, error),
	composed loop.Composed, pull string,
) error {
	ci, err := loop.ClearToAnnounce(ctx, check, composed.Pull, composed.Branch.Head)

	switch {
	case errors.Is(err, loop.ErrCIFailed):
		return forgeWordedError{loop.ErrCIFailed, "a check failed on " + pull + failedChecks(ci) +
			checkFailedWayOn(composed.Announcement.Moment)}
	case err != nil:
		return fmt.Errorf("%s was not announced: %w", pull, err)
	default:
		return nil
	}
}

// checkFailedWayOn is the rest of the refusal of an announcement at moment a
// check failed on, saying the way on: a pull request ready for review can
// wait for a run that passes; a merge, the other moment, has no CI to wait
// for, so its failed checks are re-run.
func checkFailedWayOn(moment messaging.Moment) string {
	if moment == messaging.MomentReady {
		return ", so it was not announced; workflow announce --when-green announces it once a run passes"
	}

	return ", so its merge was not announced; re-run the failed checks on the forge, then announce it once they pass"
}

// failedChecks names the checks that failed in ci, in brackets after a space,
// or nothing where the forge named none, as GitLab's pipeline does not.
func failedChecks(ci forge.CI) string {
	var names []string

	for _, check := range ci.Checks {
		if check.State == forge.CIFailed {
			names = append(names, sanitize.Line(check.Name))
		}
	}

	if len(names) == 0 {
		return ""
	}

	return " (" + strings.Join(names, ", ") + ")"
}

// pendingAnnouncement is an announcement previewed and not yet sent: what
// delivering it posts, and the memory that records it once it has gone out.
type pendingAnnouncement struct {
	delivery loop.Delivery
	memory   loop.AnnounceMemory
}

// previewAnnouncement prints the announcement and where it would go, then
// whom it tags, read under ctx, and holds it ready to deliver.
func previewAnnouncement(
	ctx context.Context, out cmdkit.Output, seams announceSeams, composed loop.Composed,
) pendingAnnouncement {
	text := composed.Announcement.Text()
	fmt.Fprintln(out.Artifact, text)
	fmt.Fprintln(out.Artifact, "to "+seams.Messaging.Target())

	mentions, memory := tagAnnouncement(ctx, out, seams, composed.Announcement.Moment, ownersBase(composed))

	return pendingAnnouncement{
		delivery: composed.Delivery(seams.Messaging.Channel, text, mentions),
		memory:   memory,
	}
}

// deliverAnnouncement posts pending under ctx and says where it went, and, for
// one posted that the store could not remember, the warning every surface
// says, then why.
func deliverAnnouncement(
	ctx context.Context, notes console.Notes, seams announceSeams, pending pendingAnnouncement,
) error {
	err := loop.Deliver(ctx, seams.Deliver, pending.memory, pending.delivery)

	why, notKept := loop.NotRememberedReason(err)
	if err != nil && !notKept {
		return fmt.Errorf("announcing to %s: %w", seams.Messaging.Service(), err)
	}

	notes.Note("Announced to " + seams.Messaging.Target() + ".")

	if notKept {
		notes.Warn(loop.NotRememberedWarning + " " + sanitize.Line(why))
	}

	return nil
}

// offerAgain says that an earlier session already made this announcement, and
// reports whether to offer it again: asked, yes, but never repeated under --yes,
// which answers only the question it can see coming. A dry run goes on all the
// same, to preview the announcement and say what --yes would do with it.
func offerAgain(notes console.Notes, pull string, opts cmdkit.WriteOptions) bool {
	notes.Note(pull + " was already announced at this moment in an earlier session.")

	if opts.Yes && !opts.DryRun {
		notes.Note("Not announced again; run without --yes to be asked.")

		return false
	}

	return true
}

// announcePrompt is what announce says around its confirmation: it asks
// whether to announce again when an earlier session already did, and its dry
// run under --yes says that such a moment would be left as it is.
func announcePrompt(service, target string, again bool, opts cmdkit.WriteOptions) cmdkit.WritePrompt {
	prompt := cmdkit.WritePrompt{
		Question: "Announce to " + service + "?",
		DryRun:   "dry run: would announce to " + target,
		Declined: "Not announced.",
	}

	if again {
		prompt.Question = "Announce to " + service + " again?"
	}

	if again && opts.Yes {
		prompt.DryRun = "dry run: would not announce it again; --yes leaves an announced moment as it is"
	}

	return prompt
}

// unattendedAgain words a repeat that nothing could confirm: --yes leaves a
// moment already announced as it is, so the way on is a terminal, not --yes.
func unattendedAgain(err error, again bool) error {
	if again && errors.Is(err, cmdkit.ErrNoTerminal) {
		return fmt.Errorf("%w; run it at a terminal to be asked whether to announce it again", cmdkit.ErrNoTerminal)
	}

	return err
}
