// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

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
// CI, went while the preview was open.
var (
	errAnnouncementOnItsWay = errors.New("an announcement is on its way; wait for it before posting another")
	errAlreadyAnnounced     = errors.New("this was announced while the preview was open; esc closes it")
)

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
	noCI    bool
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

// refusal is why the preview cannot post now: a post waiting for CI went while
// it was open and has not answered, or has, announcing the moment this one
// marks.
func (p messagingPreview) refusal(m Model) error {
	switch {
	case m.messaging.send.sending:
		return errAnnouncementOnItsWay
	case slices.Contains(m.messaging.posted, loop.Announced{Pull: m.review.pull.Number, Moment: p.moment}):
		return errAlreadyAnnounced
	default:
		return nil
	}
}

var (
	_ editable                   = messagingPreview{}
	_ failable[messagingPreview] = messagingPreview{}
)

// view shows the message as it will be posted, where, and how CI stands, its
// outcome pinned under the title so a long refusal is seen, not clipped.
func (p messagingPreview) view(kit renderKit, width, _ int) (string, string) {
	lines := pinnedOutcome(kit.styles, kit.marks, p.send, "announcing", width)
	lines = append(lines, wrap(p.text, width), "", "to  "+p.destination())
	lines = append(lines, p.tagging.lines(kit.marks, kit.styles, width)...)

	return "Announce to " + p.service, strings.Join(lines, "\n")
}

// footer offers announcing now or when CI passes, changing the channel where
// there is a choice, another edit, or leaving.
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

// cycleChannel moves the destination to the next configured channel, wrapping,
// and reads who is in it when the post tags anyone.
func (p messagingPreview) cycleChannel(m Model, step int) (Model, tea.Cmd) {
	if len(p.channels) <= 1 {
		return m, nil
	}

	current := slices.Index(p.channels, p.channel)
	p.channel = p.channels[(current+step+len(p.channels))%len(p.channels)]

	var readMembers tea.Cmd
	if p.tagging.offered {
		p.tagging.members, readMembers = m.readMembers(p.channel, p.opened)
	}

	m.overlay = p

	return m, readMembers
}

// post posts the message now.
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
// for review" announcement can: a merge or a red CI has already happened, and a
// pull request that reports no CI has none to wait for. Nor can it while whom
// it tags is still being read, since it would wait tagging no one.
func (p messagingPreview) waitsForCI() bool {
	return p.moment == messaging.MomentReady && !p.noCI && !p.tagging.reading
}

// postWhenGreen posts once CI passes: now, if it already has. Where the post
// cannot wait for CI, the preview does not offer the key, and it does nothing.
func (p messagingPreview) postWhenGreen(m Model) (Model, tea.Cmd) {
	if !p.waitsForCI() {
		return m, nil
	}

	refusal := p.refusal(m)
	if refusal != nil {
		return m.noticedGuidance(refusal), nil
	}

	if m.review.ci.State == forge.CIPassed {
		return p.post(m)
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination() + " once CI passes" +
			p.tagging.dryRunNote()), nil
	}

	m = m.closeOverlay().noticed(m.marks.inFlight + " will announce to " + p.destination() + " once CI passes")
	m.messaging.pending, m.messaging.send.err, m.messaging.dropped = queuedPost{
		pull: m.review.pull.Number, post: p.outgoing(),
	}, nil, ""

	return m.keepPolling(m.checkCI())
}

// sendToMessaging posts an announcement, and once it has gone out records it
// in the store, with the groups it tagged when it offered any, the way every
// surface delivers an announcement. It replaces any post waiting for CI: that
// one would otherwise follow it once CI passed, and the channel would read it
// twice.
func (m Model) sendToMessaging(out outgoingAnnouncement) (Model, tea.Cmd) {
	post, memory := m.deps.Messaging.Post, loop.AnnounceMemory{Record: m.deps.Store.RecordAnnounce}
	if record := m.deps.Store.RecordGroups; out.tags.offersGroups && record != nil {
		memory.RecordGroups = func(ids []string) error { return record(out.tags.workspace, ids) }
	}

	made := loop.Announced{Pull: m.review.pull.Number, Moment: out.moment}
	delivery := loop.Delivery{Channel: out.channel, Text: out.text, Made: made, Mentions: out.tags.mentions}
	m.messaging.send, m.messaging.pending, m.messaging.dropped = starting(), queuedPost{}, ""

	return m, func() tea.Msg {
		return messagingPosted{made: made, to: out.to, opened: out.opened, err: loop.Deliver(post, memory, delivery)}
	}
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
// was written in while that is still open. A post that waited for CI was
// written in a preview long closed, so it leaves one opened since as it is.
func (msg messagingPosted) apply(m Model) (Model, tea.Cmd) {
	preview, open := beneath[messagingPreview](m, msg.opened)

	if _, notKept := loop.NotRememberedReason(msg.err); msg.err != nil && !notKept {
		m.messaging.send = m.messaging.send.failed(msg.err)
		if open {
			m = m.withBeneath(preview.failed(msg.err))
		}

		return m.noticedFailure(msg.err), nil
	}

	m.messaging.posted, m.messaging.send = append(slices.Clone(m.messaging.posted), msg.made), sendState{}

	if open {
		m = m.closeOverlay()
	}

	return m.noticed(m.marks.done + " announced to " + msg.to + m.notKept(msg.err)), nil
}

// notKept is what the notice adds of an announcement the store could not
// remember, on its one row: the warning every surface says, then why. It adds
// nothing when the store remembered it.
func (m Model) notKept(err error) string {
	why, notKept := loop.NotRememberedReason(err)
	if !notKept {
		return ""
	}

	return m.marks.separator + loop.NotRememberedWarning + " " + why
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

	return m, m.saveLink(decided(owner.Owner, owner.Team, nil), p.opened)
}
