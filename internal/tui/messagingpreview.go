// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// messagingHelp is what the editor shows below a message being composed. The
// markup depends on the service, so the note stays general rather than naming
// one.
const messagingHelp = "Edit the message above this line."

// Why a preview does not post: another, waiting for CI, is being posted, or
// went while the preview was open; or the forge began merging the pull request
// while it was open.
var (
	errAnnouncementOnItsWay = errors.New("an announcement is on its way; wait for it before posting another")
	errAlreadyAnnounced     = errors.New("this was announced while the preview was open; esc closes it")
	errBeganMerging         = errors.New("this began merging while the preview was open; announce it once it has merged")
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
	// send is the post made now, out or refused; problem is why the editor
	// would not open, found before anything is sent.
	send    send.State
	problem error
	// tagging is whom a ready-for-review announcement tags; the zero value
	// tags no one.
	tagging tagSection
	// opened is the round this one opened in, so a read started for it
	// lands in it alone.
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
// it says and the moment it marks; whom it tags; and the round its preview
// opened in, so its answer reaches that preview alone.
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
// CI is being posted, the forge began merging the pull request, or the moment
// this one marks was announced while it was open.
func (p messagingPreview) refusal(m Model) error {
	if m.messaging.held.hold.Stage() == armed.StageDue {
		return errAnnouncementOnItsWay
	}

	return p.changedRefusal(m)
}

// changedRefusal is why what this preview marks changed while it was open:
// errBeganMerging once the forge began merging the pull request, then
// errAlreadyAnnounced once its moment was announced, as the store last said;
// nil otherwise.
func (p messagingPreview) changedRefusal(m Model) error {
	if m.review.pullState() == progress.PullRequestMerging {
		return errBeganMerging
	}

	if m.messaging.claimed(p.marked(m)) {
		return errAlreadyAnnounced
	}

	return nil
}

// marked is what this preview's post announces: the pull request on screen,
// at the moment it marks.
func (p messagingPreview) marked(m Model) loop.Announced {
	return loop.Announced{Pull: m.review.pull().Number, Moment: p.moment}
}

// withoutRefusedPreviewOf closes the announcement's preview open on made
// whose post made now was refused, now the waiting post has announced made in
// its place, as a post made now that went closes its own, with the owner
// picker floating over it. The refusal gives the footer's row up to how the
// waiting post went, while it still holds it: a failure that took the row
// since stays. A preview that posted nothing stays, refusing a post as
// already announced.
func (m Model) withoutRefusedPreviewOf(made loop.Announced) Model {
	preview, open := m.previewShown()
	if !open || preview.send.Stage() != send.StageRefused || preview.marked(m) != made {
		return m
	}

	if m.notice.Text() == m.failureRow("", preview.send.Reason()) {
		m.notice = m.notice.Cleared()
	}

	return m.closeOverlay()
}

// previewShown is the announcement's preview, on top or under the owner
// picker floating over it, and whether one is open.
func (m Model) previewShown() (messagingPreview, bool) {
	shown := m.overlay
	if picker, picking := shown.(ownerPicker); picking {
		shown = picker.back
	}

	preview, open := shown.(messagingPreview)

	return preview, open
}

var (
	_ editable                 = messagingPreview{}
	_ sender[messagingPreview] = messagingPreview{}
	_ linksOwners              = messagingPreview{}
)

// view shows the message as it will be posted, and where, its outcome pinned
// under the title so a long refusal is seen, not clipped.
func (p messagingPreview) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedProblem(p.problem, width), kit.pinnedSend(p.send, "announcing", width)...)
	lines = append(lines, wrap(p.text, width), "", "to  "+p.destination())
	lines = append(lines, p.tagging.lines(kit, width)...)

	return "Announce to " + p.service, strings.Join(lines, "\n")
}

// footer offers announcing now — whatever CI was last seen to do, since the
// gate reads it again as it posts — or when CI passes, changing the channel
// where there is a choice, another edit, or leaving.
func (p messagingPreview) footer(keys keyMap) []key.Binding {
	if p.send.Stage() == send.StageInFlight {
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
	case p.send.Stage() == send.StageInFlight:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.cycleRight):
		return p.cycleChannel(m, 1)
	case key.Matches(msg, m.keys.cycleLeft):
		return p.cycleChannel(m, -1)
	case key.Matches(msg, m.keys.edit) && m.deps.Editor.Edit != nil:
		return m, m.deps.Editor.Edit(p.text, messagingHelp, editedFor(p.opened))
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
		p.tagging.members, readMembers = p.tagging.members.cleared().readingMembers(m.session, m.deps, p.channel, p.opened)
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

	p.send, _ = p.send.Sent()
	p.problem = nil
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

	refusal := p.changedRefusal(m)
	if refusal != nil {
		return m.noticedGuidance(refusal), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination() + " once CI passes" +
			p.tagging.dryRunNote()), nil
	}

	return m.armAsked(p.outgoing())
}

// applyEdit puts the edited message back in the preview, clearing why an
// earlier edit failed, or keeps why the editor failed as its problem, in
// place of a refusal an earlier post had.
func (p messagingPreview) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.problem, p.send = err, p.send.Cleared()
	} else {
		p.text, p.problem = text, nil
	}

	return m.withAnswered(p), nil
}

// sent is the post made now.
func (p messagingPreview) sent() send.State { return p.send }

// withSend is the preview with the post made now as state.
func (p messagingPreview) withSend(state send.State) messagingPreview {
	p.send = state

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

// openedAs is the round the preview opened in.
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
