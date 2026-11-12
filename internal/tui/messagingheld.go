// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errAnnouncementDropped is an announcement that waited for CI and was given
// up on: a failure, told with the service it was for and why.
var errAnnouncementDropped = errors.New("dropped the announcement")

// armOnOpen arms the announcement of pull, just opened, to post once its CI
// passes, as a preview's w would, tagging whom announce --yes would, and
// answers what the opened notice adds of it and the read arming starts: where
// it goes, why a draft is not announced, or that another pull request's
// announcement still waits, which one armed on its own never replaces.
// Nothing is armed, and nothing said, with messaging.auto_announce off,
// nowhere to post, or for one announced already. A dry run opens nothing, so
// it arms nothing either.
func (m Model) armOnOpen(pull forge.PullRequest) (Model, string, tea.Cmd) {
	armable := m.cfg.Messaging.AutoAnnounce && m.cfg.Messaging.Mode() != config.MessagingNone &&
		m.deps.Messaging.Post != nil && !m.messaging.announced(m.review)
	if !armable {
		return m, "", nil
	}

	if pull.Draft {
		return m, m.marks.separator + "a draft is not announced; announce it once it is ready", nil
	}

	channel := defaultChannel(m.cfg)
	out := outgoingAnnouncement{
		channel: channel, to: sanitize.Line(cmp.Or(channel, m.cfg.Messaging.Target())),
		text: m.announcement(messaging.MomentReady), moment: messaging.MomentReady, tags: postTags{}, opened: 0,
	}

	started, err := m.messaging.hold.Arm(m.deps.now(), armed.Arming{
		Delivery: m.deliveryOf(out), Branch: m.branch.branch.Name, Opened: true, Unasked: true, Automatic: true,
		Seen: 0,
	})
	if err != nil {
		return m, m.marks.separator + m.pullName(m.messaging.hold.Pull()) + " is still waiting, so " +
			m.pullName(pull.Number) + " was not armed", nil
	}

	m.messaging.send.err = nil
	m, read := m.held(started, postTags{})

	return m, m.marks.separator + "announces to " + out.to + " once CI passes", read
}

// armAsked arms out, as its preview asked, to post once CI passes — at once,
// if a read finds it has — and closes the preview. One armed over another
// pull request's waiting announcement replaces it, saying so; one asked for
// while another is being posted is refused, the preview left open.
func (m Model) armAsked(out outgoingAnnouncement) (Model, tea.Cmd) {
	started, err := m.messaging.hold.Arm(m.deps.now(), armed.Arming{
		Delivery: m.deliveryOf(out), Branch: m.branch.branch.Name, Opened: false, Unasked: false, Automatic: false,
		Seen: 0,
	})
	if err != nil {
		return m.noticedGuidance(err), nil
	}

	notice := m.marks.inFlight + " will announce to " + out.to + " once CI passes"
	if replaced := started.Replaced; replaced != nil {
		notice += m.marks.separator + "replaced the announcement waiting for " + m.pullName(replaced.Pull())
	}

	m.messaging.send.err = nil

	return m.closeOverlay().noticed(notice).held(started, out.tags)
}

// deliveryOf is out, the announcement of the pull request on screen, as it is
// delivered: its CI gated at the commit the forge says it is at.
func (m Model) deliveryOf(out outgoingAnnouncement) loop.Delivery {
	return loop.Delivery{
		Channel: out.channel, Text: out.text, Pull: m.review.pull, Head: m.review.pull.HeadCommit, Moment: out.moment,
		Mentions: out.tags.mentions,
	}
}

// held is the model holding the announcement just armed, which tags whom tags
// say, and doing what arming it asks first.
func (m Model) held(started armed.Armed, tags postTags) (Model, tea.Cmd) {
	m.messaging.hold, m.messaging.holdTags = started.Hold, tags

	return m.afterNext(started.Next)
}

// afterNext does what a step of the hold asks: a read now, a read once the CI
// interval has passed, the post, or nothing.
func (m Model) afterNext(next armed.Next) (Model, tea.Cmd) {
	return map[armed.Next]func() (Model, tea.Cmd){
		armed.NextNothing:   func() (Model, tea.Cmd) { return m, nil },
		armed.NextRead:      func() (Model, tea.Cmd) { return m, m.readHold() },
		armed.NextReadLater: m.readHoldLater,
		armed.NextDeliver:   func() (Model, tea.Cmd) { return m, m.deliverHold() },
	}[next]()
}

// readHoldLater schedules the next read of the hold once the CI interval has
// passed, on a beat of its own whatever the Review pane polls at. Only the
// read last scheduled is made, so a read asked for meanwhile, which
// schedules its own, leaves one beat going.
func (m Model) readHoldLater() (Model, tea.Cmd) {
	m.messaging.tick++
	tick := holdTick{tick: m.messaging.tick}

	return m, m.deps.after(m.deps.ciInterval(), func(time.Time) tea.Msg { return tick })
}

// holdTick is time to read the hold again, as the read scheduled tick-th.
type holdTick struct {
	tick int
}

var _ applier = holdTick{}

// apply reads the hold again, unless another read has been scheduled since.
func (msg holdTick) apply(m Model) (Model, tea.Cmd) {
	if msg.tick != m.messaging.tick {
		return m, nil
	}

	return m, m.rereadHold()
}

// rereadHold is the read of the hold now, while it waits; nothing otherwise.
func (m Model) rereadHold() tea.Cmd {
	if !m.messaging.hold.Stage().Waiting() {
		return nil
	}

	return m.readHold()
}

// readHold reads the held pull request, found by the branch it was armed on,
// never the checkout's, and its CI, under the session.
func (m Model) readHold() tea.Cmd {
	ctx, seams, hold, now := m.session, m.holdSeams(), m.messaging.hold, m.deps.now

	return func() tea.Msg {
		reading, failure := seams.Read(ctx, hold, now)

		return holdRead{reading: reading, failure: failure}
	}
}

// holdRead is a read of the hold: what it heard, or how it failed.
type holdRead struct {
	reading armed.Reading
	failure armed.Failure
}

var _ applier = holdRead{}

// apply steps the hold on the read.
func (msg holdRead) apply(m Model) (Model, tea.Cmd) {
	return m.steppedTo(m.messaging.hold.Hear(msg.reading, msg.failure))
}

// deliverHold posts the held announcement, its gate, claim and post going on
// past a quit as every write does, so a claim is never left mid-post.
func (m Model) deliverHold() tea.Cmd {
	ctx, seams, hold, now := m.session, m.holdSeams(), m.messaging.hold, m.deps.now

	return func() tea.Msg {
		return holdDelivered{outcome: seams.DeliverHeld(ctx, hold, now)}
	}
}

// holdDelivered is how the post of the held announcement went.
type holdDelivered struct {
	outcome armed.Outcome
}

var _ applier = holdDelivered{}

// apply steps the hold on the post's outcome, and reads what was announced
// again, which the post's claim changed.
func (msg holdDelivered) apply(m Model) (Model, tea.Cmd) {
	m, next := m.steppedTo(m.messaging.hold.Delivered(msg.outcome))

	return m, tea.Batch(next, loadAnnounces(m.session, m.deps))
}

// holdSeams are what the hold reads and posts through: the forge's reads
// under the session, and the post, its gate and the store's claim, which go
// on past it.
func (m Model) holdSeams() armed.Seams {
	return armed.Seams{
		FindPull: m.deps.Forge.FindPullRequest, CheckCI: m.deps.Forge.CheckStatus, Deliver: m.deliverSeams(),
		Memory: m.announceMemory(m.messaging.holdTags), UnaskedTags: m.unaskedTags(),
	}
}

// steppedTo is the model holding hold, a step on from the one it held, having
// said what the step changed, and doing what it asks next.
func (m Model) steppedTo(hold armed.Hold, next armed.Next) (Model, tea.Cmd) {
	was := m.messaging.hold
	m.messaging.hold = hold

	return m.toldOf(was, hold).afterNext(next)
}

// toldOf says what a step from was to now changed that the rail alone would
// not: how a post went, or that a waiting announcement was given up on. A
// read that leaves it waiting, or one answering for a hold since ended, says
// nothing.
func (m Model) toldOf(was, now armed.Hold) Model {
	switch {
	case was.Stage() == armed.StageDue:
		return m.toldOfPost(now)
	case was.Stage().Waiting() && now.Stage() == armed.StageDropped:
		return m.droppedNotice(now)
	default:
		return m
	}
}

// toldOfPost says how the post of the held announcement went, now the hold
// after it: posted; claimed by another session first; a check the gate saw
// fail, which it waits on a fix past; a failure it tries again past; or given
// up on.
func (m Model) toldOfPost(now armed.Hold) Model {
	stay := func() Model { return m }
	name := m.pullName(now.Pull())
	// A map, not a switch, so there is no arm for the stages a post never
	// leaves a hold at.
	told := map[armed.Stage]func() Model{
		armed.StagePosted: func() Model {
			m.messaging.send.err = nil

			return m.noticed(m.marks.done + " announced to " + m.destinationOf(now.Delivery()) +
				unconfirmedNote(m.marks, now.Reason()))
		},
		armed.StageElsewhere: func() Model {
			return m.noticed(m.marks.done + " already announced from another session; not posted again")
		},
		armed.StageDropped: func() Model { return m.droppedNotice(now) },
		armed.StageAwaitingFix: func() Model {
			return m.noticed(m.marks.inFlight + " not announced yet: a check failed on " + name +
				failedCheckNames(now.CI()) + " as it was to be posted; it waits for a run that passes")
		},
		armed.StageUnheard: func() Model {
			return m.noticed(m.marks.inFlight + " not announced yet: " + briefly(now.Reason()) + "; trying again")
		},
		armed.StageDue: stay, armed.StageIdle: stay, armed.StageListing: stay, armed.StageAwaitingChecks: stay,
		armed.StageRunning: stay, armed.StageStopped: stay,
	}

	return told[now.Stage()]()
}

// destinationOf is where delivery goes, as a notice names it: its channel, or
// the webhook's own.
func (m Model) destinationOf(delivery loop.Delivery) string {
	return sanitize.Line(cmp.Or(delivery.Channel, m.cfg.Messaging.Target()))
}

// droppedNotice says, as a failure, that the hold was given up on: the
// service it was for, the pull request it waited for, and why — a reason of
// its own in its words, and a post's failure in the words every failure of
// its kind is told in.
func (m Model) droppedNotice(hold armed.Hold) Model {
	if words, waited := m.droppedCause(hold); waited {
		return m.noticedFailure(fmt.Errorf("%w to %s waiting for %s: %s", errAnnouncementDropped,
			m.cfg.Messaging.Service(), m.pullName(hold.Pull()), words))
	}

	return m.noticedFailureLedBy(errAnnouncementDropped.Error()+" to "+m.cfg.Messaging.Service()+" waiting for "+
		m.pullName(hold.Pull())+": ", hold.Reason())
}

// droppedWords is why the hold was given up on, in brief.
func (m Model) droppedWords(hold armed.Hold) string {
	words, waited := m.droppedCause(hold)
	if !waited {
		return briefly(hold.Reason())
	}

	return words
}

// droppedCause is why the hold was given up on while it waited, and whether
// it was: the forge never listed its pull request, nothing was heard of it
// for too long, no check was reported, CI ran unchanged for too long, or it
// merged first or is no longer its branch's. Any other reason is a post's failure.
func (m Model) droppedCause(hold armed.Hold) (string, bool) {
	name, reason := m.pullName(hold.Pull()), hold.Reason()
	patience := strconv.Itoa(int(armed.UnheardPatience.Minutes())) + "m"
	quiet := strconv.Itoa(int(armed.NoCIPatience.Minutes())) + "m"
	running := strconv.Itoa(int(armed.RunningPatience.Hours())) + "h"

	// In order: an UnheardError matches ErrNotListedYet when the forge never
	// listed it, which says more.
	causes := []struct {
		cause error
		words func() string
	}{
		{armed.ErrNotListedYet, func() string { return "the forge did not list " + name + " within " + patience }},
		{armed.ErrUnheard, func() string {
			unheard, _ := errors.AsType[armed.UnheardError](reason)

			return "tried for " + patience + " and failed: " + briefly(unheard.Last)
		}},
		{armed.ErrNoCI, func() string {
			return "no checks reported on " + name + " within " + quiet + "; announce it by hand"
		}},
		{armed.ErrRunningTooLong, func() string { return "CI on " + name + " has been running for " + running }},
		{armed.ErrMergedFirst, func() string { return name + " merged before its CI passed" }},
		{armed.ErrPullChanged, func() string { return name + " is no longer its branch's " + m.vocab.noun }},
	}

	for _, known := range causes {
		if errors.Is(reason, known.cause) {
			return known.words(), true
		}
	}

	return "", false
}

// pullName is pull request number as the forge writes it: #42, or !42.
func (m Model) pullName(number int) string {
	return m.vocab.sigil + strconv.Itoa(number)
}

// unaskedTags is whom an announcement armed as its pull request opened tags,
// asked of no one, as workflow announce --yes proposes: the linked owners of
// the changes against the base its pull request merges into, where the forge
// says, and the groups that start checked, read under ctx as it goes. A read
// that fails tags fewer, no workspace tags no one, and the token's scopes
// limit it as they limit a preview's. It is nil without a Slack user token
// and its directory, which alone tag anyone.
func (m Model) unaskedTags() func(ctx context.Context, delivery loop.Delivery) messaging.Mentions {
	if m.cfg.Messaging.Mode() != config.MessagingUser || m.deps.Messaging.ChannelMembers == nil {
		return nil
	}

	deps, branchBase := m.deps, m.branch.branch.BaseName()

	return func(ctx context.Context, delivery loop.Delivery) messaging.Mentions {
		read := readTagProposal(ctx, deps, cmp.Or(delivery.Pull.Base, branchBase))
		if read.workspaceErr != nil {
			return messaging.Mentions{}
		}

		tags, tagging := scopedTags(ctx, deps, delivery.Channel, read.proposal.tags())
		if !tagging {
			return messaging.Mentions{}
		}

		_, mentions := tags.Proposed()

		return mentions
	}
}

// scopedTags is tags as the Slack token's scopes let them go, learned from
// the directory, read under ctx, as a preview learns them: no one without the
// scope reading channel's members needs, and no group without the one the
// user groups need. It reports false when it tags no one.
func scopedTags(ctx context.Context, deps Deps, channel string, tags loop.Tags) (loop.Tags, bool) {
	_, err := deps.Messaging.ChannelMembers(ctx, channel)
	if (directory{err: err}).missingScope() != "" {
		return loop.Tags{}, false
	}

	var groupsErr error
	if read := deps.Messaging.UserGroups; read != nil {
		_, groupsErr = read(ctx)
	}

	if (directory{err: groupsErr}).missingScope() != "" {
		tags.Groups = nil
	}

	return tags, true
}

// stopWaiting stops the announcement waiting for CI, unposted, at the user's
// word; with none waiting, it says why it did nothing.
func (m Model) stopWaiting() (Model, tea.Cmd) {
	stopped, err := m.messaging.hold.Stop()
	if err != nil {
		return m.noticedGuidance(err), nil
	}

	m.messaging.hold = stopped

	return m.noticed("stopped waiting; nothing was announced"), nil
}

// quitOrGuard quits, unless a post is waiting for CI, in which case it asks
// first: quitting would lose the post without a word.
func (m Model) quitOrGuard() (Model, tea.Cmd) {
	if m.messaging.hold.Stage().Waiting() {
		m.overlay = quitGuard{}

		return m, nil
	}

	return m.quit()
}

// quit ends the interface, stopping the announcement waiting for CI, if one
// is, which the Next it ends with names so it is said once the terminal is
// back.
func (m Model) quit() (Model, tea.Cmd) {
	if m.messaging.hold.Stage().Waiting() {
		m.next.stoppedWaiting = m.pullName(m.messaging.hold.Pull())
	}

	return m, tea.Quit
}

// quitGuard asks before quitting while a post is waiting for CI, which quitting
// would silently lose.
type quitGuard struct{}

var _ overlay = quitGuard{}

// view says what quitting would cost.
func (quitGuard) view(_ renderKit, width, _ int) (string, string) {
	return "Quit", wrap("An announcement is waiting for CI and will be lost.", width)
}

// footer offers quitting anyway or staying.
func (quitGuard) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "quit"), relabel(keys.closeOverlay, escStay)}
}

// handleKey answers a key while the guard is shown.
func (quitGuard) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return m.quit()
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	default:
		return m, nil
	}
}

// which names the quit guard.
func (quitGuard) which() overlayKind { return overlayQuitGuard }
