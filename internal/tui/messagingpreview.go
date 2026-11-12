// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// messagingHelp is what the editor shows below a message being composed. The
// markup depends on the service, so the note stays general rather than naming
// one.
const messagingHelp = "Edit the message above this line."

// Why a preview opened over another post does not post: that one, waiting for
// CI, is being posted, or went while the preview was open.
var (
	errAnnouncementOnItsWay = errors.New("an announcement is on its way; wait for it before posting another")
	errAlreadyAnnounced     = errors.New("this was announced while the preview was open; esc closes it")
)

// errMergeCheckFailed is the gate's refusal of a merge a check failed on,
// which has no CI to wait for: its failed checks are re-run instead.
var errMergeCheckFailed = errors.New("the merge was not announced")

// messagingPreview is an announcement about to be sent.
type messagingPreview struct {
	text string
	// fallback is where a post goes when no channel is chosen — a webhook's own
	// channel, or the note that none is set.
	fallback string
	// channel is the chosen bot channel, and channels the ones it can be cycled
	// through. Both are empty for a webhook, which carries its own channel.
	channel  string
	channels []string
	// moment is what this post marks, so the pane records the right one as posted
	// and offers "announce when CI passes" only where waiting for CI makes sense.
	moment messaging.Moment
	// service names the messaging service, for the preview title.
	service string
	send    sendState
	// tagging is whom a ready-for-review announcement tags; the zero value
	// tags no one.
	tagging tagSection
	// opened is the count of overlays opened when this one opened, so a read
	// started for it lands in it alone.
	opened int
}

// destination is where this post will go, as it is shown: the configuration
// names it, so it is drawn as text alone.
func (p messagingPreview) destination() string {
	if p.channel != "" {
		return sanitize.Line(p.channel)
	}

	return sanitize.Line(p.fallback)
}

// outgoingAnnouncement is an announcement as it leaves: the channel it is sent
// to, empty for a webhook's own, and where that is as the notice names it; what
// it says and the moment it marks; whom it tags; and the count of overlays
// opened when its preview opened, so its answer reaches that preview alone.
type outgoingAnnouncement struct {
	channel, to, text string
	moment            messaging.Moment
	tags              postTags
	opened            int
}

// outgoing is the announcement this preview sends, as it stands.
func (p messagingPreview) outgoing() outgoingAnnouncement {
	return outgoingAnnouncement{
		channel: p.channel, to: p.destination(), text: p.text, moment: p.moment, tags: p.tagging.postTags(),
		opened: p.opened,
	}
}

// refusal is why the preview cannot post now: the announcement waiting for
// CI is being posted, or the moment this one marks was announced while it
// was open.
func (p messagingPreview) refusal(m Model) error {
	if m.messaging.hold.Stage() == armed.StageDue {
		return errAnnouncementOnItsWay
	}

	return p.announcedRefusal(m)
}

// announcedRefusal is errAlreadyAnnounced once the moment this preview marks
// was announced while it was open, as the store last said; nil otherwise.
func (p messagingPreview) announcedRefusal(m Model) error {
	if m.messaging.claimed(loop.Announced{Pull: m.review.pull.Number, Moment: p.moment}) {
		return errAlreadyAnnounced
	}

	return nil
}

var (
	_ editable                   = messagingPreview{}
	_ failable[messagingPreview] = messagingPreview{}
)

// view shows the message as it will be posted, and where, its outcome pinned
// under the title so a long refusal is seen, not clipped.
func (p messagingPreview) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedOutcome(p.send, "announcing", width)
	lines = append(lines, wrap(p.text, width), "", "to  "+p.destination())
	lines = append(lines, p.tagging.lines(kit, width)...)

	return "Announce to " + p.service, strings.Join(lines, "\n")
}

// footer offers announcing now — whatever CI was last seen to do, since the
// gate reads it again as it posts — or when CI passes, changing the channel
// where there is a choice, another edit, or leaving.
func (p messagingPreview) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	buttons := []key.Binding{relabel(keys.confirm, "announce now")}

	if p.waitsForCI() {
		buttons = append(buttons, relabel(keys.postWhenGreen, "when CI passes"))
	}

	if len(p.channels) > 1 {
		buttons = append(buttons, relabel(keys.cycleLeft, "change channel"))
	}

	buttons = append(buttons, keys.edit, relabel(keys.closeOverlay, escDiscard))

	return append(buttons, p.tagging.keys(keys)...)
}

// handleKey answers a key while the message is previewed.
func (p messagingPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.cycleRight):
		return p.cycleChannel(m, 1)
	case key.Matches(msg, m.keys.cycleLeft):
		return p.cycleChannel(m, -1)
	case key.Matches(msg, m.keys.edit) && m.deps.Editor.Edit != nil:
		return m, m.deps.Editor.Edit(p.text, messagingHelp, func(text string, err error) tea.Msg {
			return textEdited{text: text, err: err}
		})
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	case key.Matches(msg, m.keys.postWhenGreen):
		return p.postWhenGreen(m)
	default:
		return p.handleTagKey(m, msg)
	}
}

// which names the announcement preview.
func (messagingPreview) which() overlayKind { return overlayMessagingPreview }

// cycleChannel moves the destination to the next configured channel, wrapping,
// and reads who is in it when the post tags anyone.
func (p messagingPreview) cycleChannel(m Model, step int) (Model, tea.Cmd) {
	if len(p.channels) <= 1 {
		return m, nil
	}

	current := slices.Index(p.channels, p.channel)
	p.channel = p.channels[around(current, len(p.channels)).by(step)]

	var readMembers tea.Cmd
	if p.tagging.offered {
		p.tagging.members, readMembers = readChannelMembers(m.session, m.deps, p.channel, p.opened)
	}

	m.overlay = p

	return m, readMembers
}

// post posts the message now: the gate's own read of CI alone refuses it.
func (p messagingPreview) post(m Model) (Model, tea.Cmd) {
	refusal := p.refusal(m)
	if refusal != nil {
		return m.noticedGuidance(refusal), nil
	}

	if strings.TrimSpace(p.text) == "" {
		return m.closeOverlay().noticedGuidance(loop.ErrEmptyAnnouncement), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination() + p.tagging.dryRunNote()), nil
	}

	p.send = starting()
	m.overlay = p

	return m.sendToMessaging(p.outgoing())
}

// waitsForCI reports whether this post can wait for CI to pass. Only a "ready
// for review" announcement can, a merge having already happened — whatever
// its CI: running, no check reported yet, or a check failed, which the wait
// sees fixed; one passed already goes at once. Nor can it while whom it tags
// is still being read, since it would wait tagging no one.
func (p messagingPreview) waitsForCI() bool {
	return p.moment == messaging.MomentReady && !p.tagging.reading
}

// postWhenGreen arms the post to go once CI passes: at once, if a read finds
// it has. Where the post cannot wait for CI, the preview does not offer the
// key, and it does nothing.
func (p messagingPreview) postWhenGreen(m Model) (Model, tea.Cmd) {
	if !p.waitsForCI() {
		return m, nil
	}

	refusal := p.announcedRefusal(m)
	if refusal != nil {
		return m.noticedGuidance(refusal), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination() + " once CI passes" +
			p.tagging.dryRunNote()), nil
	}

	return m.armAsked(p.outgoing())
}

// sendToMessaging posts an announcement now, the way every surface delivers
// one: the gate first, then the store's claim, the post and the claim's
// confirmation, with the groups it tagged when it offered any. One waiting
// for CI is left waiting until this one has gone, and then is posted in its
// place.
func (m Model) sendToMessaging(out outgoingAnnouncement) (Model, tea.Cmd) {
	ctx, seams, memory, delivery := m.session, m.deliverSeams(), m.announceMemory(out.tags), m.deliveryOf(out)
	m.messaging.send = starting()

	return m, func() tea.Msg {
		err := withFailedChecks(loop.Deliver(ctx, seams, memory, delivery))

		return messagingPosted{made: delivery.Made(), to: out.to, opened: out.opened, err: refusedAt(delivery.Moment, err)}
	}
}

// deliverSeams are what a post goes through: the service, and the read of CI
// its gate makes first, both going on past a quit.
func (m Model) deliverSeams() loop.DeliverSeams {
	return loop.DeliverSeams{Post: m.deps.Messaging.Post, CheckCI: m.deps.announceCI}
}

// announceMemory is the store's claim a post takes before it goes, its
// confirmation and its release, and, for one whose tags offered groups, the
// groups it tagged, remembered as the workspace's last choice.
func (m Model) announceMemory(tags postTags) loop.AnnounceMemory {
	store := m.deps.Store
	memory := loop.AnnounceMemory{
		Recorded: nil, Claim: store.ClaimAnnounce, Confirm: store.ConfirmAnnounce, Release: store.ReleaseAnnounce,
		Reclaim: nil, RecordGroups: nil,
	}

	if record := store.RecordGroups; tags.offersGroups && record != nil {
		memory.RecordGroups = func(ctx context.Context, ids []string) error { return record(ctx, tags.workspace, ids) }
	}

	return memory
}

// withFailedChecks is err, a post's outcome, naming the checks the gate saw
// fail where it refused the post for them.
func withFailedChecks(err error) error {
	refusal, _ := errors.AsType[loop.GateRefusalError](err)

	names := failedCheckNames(refusal.CI)
	if names == "" {
		return err
	}

	return failedChecksError{err: err, names: names}
}

// failedChecksError is a refusal for a failed check that names the checks
// that failed.
type failedChecksError struct {
	err   error
	names string
}

var _ error = failedChecksError{}

// Error is the refusal's own words, then the checks that failed.
func (e failedChecksError) Error() string {
	return e.err.Error() + e.names
}

// Unwrap lets errors.Is match what the refusal does.
func (e failedChecksError) Unwrap() error {
	return e.err
}

// failedCheckNames names the checks that failed in ci, in brackets after a
// space, or nothing where the forge named none as failed.
func failedCheckNames(ci forge.CI) string {
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

// refusedAt is err, how an announcement at moment went, with a merge's
// failed check told apart from a ready one's, since only a ready one can
// wait for a run that passes.
func refusedAt(moment messaging.Moment, err error) error {
	if moment == messaging.MomentMerged && errors.Is(err, loop.ErrCIFailed) {
		return fmt.Errorf("%w: %w", errMergeCheckFailed, err)
	}

	return err
}

// applyEdit puts the edited message back in the preview, or records why the
// editor failed.
func (p messagingPreview) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.send = p.send.failed(err)
	} else {
		p.text = text
	}

	m.overlay = p

	return m, nil
}

// messagingPosted reports how posting went, the announcement it made — a pull
// request and the moment it marked — where it went, and the count of overlays
// opened when the preview it was written in opened.
type messagingPosted struct {
	made   loop.Announced
	to     string
	opened int
	err    error
}

var _ applier = messagingPosted{}

// apply records the post, or why it failed, in the pane, and in the preview it
// was written in while that is still open, and reads what was announced
// again, which its claim changed. One that went out is posted in place of the
// announcement waiting for CI of the same moment, which then goes no more;
// one that failed leaves that one waiting.
func (msg messagingPosted) apply(m Model) (Model, tea.Cmd) {
	preview, open := beneath[messagingPreview](m, msg.opened)
	m.messaging.send = sendState{}
	reread := loadAnnounces(m.session, m.deps)

	if errors.Is(msg.err, loop.ErrAnnouncedAlready) {
		return m.noticed(m.marks.done + " already announced from another session; not posted again"), reread
	}

	if _, unconfirmed := loop.UnconfirmedReason(msg.err); msg.err != nil && !unconfirmed {
		m.messaging.send = m.messaging.send.failed(msg.err)
		if open {
			m = m.withBeneath(preview.failed(msg.err))
		}

		return m.noticedFailure(msg.err), reread
	}

	m.messaging.hold = m.messaging.hold.Superseded(msg.made)

	if open {
		m = m.closeOverlay()
	}

	return m.noticed(m.marks.done + " announced to " + msg.to + unconfirmedNote(m.marks, msg.err)), reread
}

// unconfirmedNote is what the notice adds of an announcement the store could
// not confirm, on its one row: the warning every surface says, then why. It
// adds nothing when the store confirmed it.
func unconfirmedNote(marks glyphs, err error) string {
	why, unconfirmed := loop.UnconfirmedReason(err)
	if !unconfirmed {
		return ""
	}

	return marks.separator + loop.UnconfirmedWarning + " " + why
}

// failed is the preview kept open with the reason the post failed.
func (p messagingPreview) failed(err error) messagingPreview {
	p.send = p.send.failed(err)

	return p
}

// handleTagKey answers the tag section's keys in the preview.
func (p messagingPreview) handleTagKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !p.tagging.interactive() {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.cursorKeys()...):
		p.tagging = p.tagging.moved(m.keys.stepOf(msg))
	case key.Matches(msg, m.keys.toggleOption):
		p.tagging = p.tagging.toggled()
	case key.Matches(msg, m.keys.linkToSlack):
		return p.pickLink(m)
	case key.Matches(msg, m.keys.notOnSlack):
		return p.markNotOnSlack(m)
	}

	m.overlay = p

	return m, nil
}

// pickLink opens the picker on the owner under the cursor: the channel's
// members for a person, the user groups for a team.
func (p messagingPreview) pickLink(m Model) (Model, tea.Cmd) {
	owner, onOwner := p.tagging.selectedOwner()
	if !onOwner || !p.tagging.canLink {
		return m, nil
	}

	m.overlay = newOwnerPicker(m, owner.Owner, owner.Team, p)

	return m, nil
}

// openedAs is the count of overlays opened when the preview opened.
func (p messagingPreview) openedAs() int {
	return p.opened
}

// directoryFor is the directory the owner picker chooses from: the channel's
// members for a person, the user groups for a team.
func (p messagingPreview) directoryFor(team bool) directory {
	if team {
		return p.tagging.groups
	}

	return p.tagging.members
}

// markNotOnSlack saves that the owner under the cursor is not on Slack.
func (p messagingPreview) markNotOnSlack(m Model) (Model, tea.Cmd) {
	owner, onOwner := p.tagging.selectedOwner()
	if !onOwner || !p.tagging.canLink {
		return m, nil
	}

	return m.savingLink(decided(owner.Owner, owner.Team, nil), p.opened)
}
